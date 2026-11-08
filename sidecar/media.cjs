// 汽水/酷狗的音源容器转换。
//
// 汽水音乐（抖音「车机」接口）只提供 MP4 容器里的 AAC（mime_type=audio_mp4），
// 而播放器只解 MP3 与 FLAC。这里把拿到的音频嗅探一下：本来就是 MP3/FLAC 的
// 直接给 CDN 地址；是 MP4/AAC 的就在本机转成 FLAC（无损承载，播放器直接能解），
// 由桥接自己的 HTTP 端口提供给播放器。
//
// 转码用系统自带的工具：macOS 的 afconvert（始终存在），其他平台退回 ffmpeg。
"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const crypto = require("node:crypto");
const { execFile } = require("node:child_process");

// 转好的文件放临时目录，重启后仍在，重播同一首立刻能用。
const dir = path.join(os.tmpdir(), "echo-sidecar-media");
try {
	fs.mkdirSync(dir, { recursive: true });
} catch {}

// 最多留这么多转好的文件，超了删最旧的（一首 FLAC 约几 MB）。
const maxFiles = 24;

// base 是本机桥接的地址前缀，serve 时拼成播放器可用的 URL。
let base = "";

function setBase(b) {
	base = String(b || "").replace(/\/$/, "");
}

function keyOf(...parts) {
	return crypto.createHash("sha1").update(parts.join("|")).digest("hex").slice(0, 24);
}

// UA 用浏览器的一条：抖音与酷狗的 CDN 都拒绝对陌生 UA 发来的请求。
const UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15";

// classify 看头几个字节，判断容器：mp4（AAC）、mp3、flac，或 unknown。
function classify(head) {
	if (!head || head.length < 8) return "unknown";
	if (head.toString("latin1", 4, 8) === "ftyp") return "mp4";
	if (head.toString("latin1", 0, 4) === "fLaC") return "flac";
	if (head.toString("latin1", 0, 3) === "ID3") return "mp3";
	if (head[0] === 0xff && (head[1] & 0xe0) === 0xe0) return "mp3";
	// 没有 ftyp 的 MP4（有些 CDN 直接从 moov/mdat 起），按片段名再判一次。
	if (head.toString("latin1", 0, 8).includes("moov") || head.toString("latin1", 0, 8).includes("mdat")) return "mp4";
	return "unknown";
}

// sniff 看头几个字节，判断远端文件的容器。
async function sniff(url) {
	return classify(await readHead(url, 16));
}

// sniffFile 判断已经下载到本地的文件是什么容器。
function sniffFile(file) {
	let fd;
	try {
		fd = fs.openSync(file, "r");
		const buf = Buffer.alloc(16);
		const n = fs.readSync(fd, buf, 0, 16, 0);
		return classify(buf.subarray(0, n));
	} catch {
		return "unknown";
	} finally {
		if (fd !== undefined) {
			try {
				fs.closeSync(fd);
			} catch {}
		}
	}
}

// originOf 是 url 的来路，用来拼 Referer。
function originOf(url) {
	try {
		const p = new URL(url);
		return p.protocol + "//" + p.host;
	} catch {
		return "";
	}
}

// fetchAudio 拉一个音频地址。4xx 时带上来路（Referer）再试一次：这两家
// 的 CDN 对不带来路的请求常回 403，而播放器原来拿着直链自己去拉，就是这么
// 失败的——报"加载失败，再点一次试试"。
async function fetchAudio(url, signal, extra) {
	const headers = { "User-Agent": UA, ...(extra || {}) };
	const res = await fetch(url, { signal, headers });
	if (res.ok || res.status < 400 || res.status >= 500) return res;
	const origin = originOf(url);
	if (!origin) return res;
	if (res.body && typeof res.body.cancel === "function") res.body.cancel().catch(() => {});
	return fetch(url, { signal, headers: { ...headers, Referer: origin + "/" } });
}

// readHead 取 url 的前 n 个字节，尽量用 Range，不回就整段读一点点。
async function readHead(url, n) {
	const ctl = new AbortController();
	const timer = setTimeout(() => ctl.abort(), 15000);
	try {
		const res = await fetchAudio(url, ctl.signal, { Range: `bytes=0-${n - 1}` });
		if (!res.ok) return null;
		const buf = Buffer.from(await res.arrayBuffer());
		if (res.body && typeof res.body.cancel === "function") res.body.cancel().catch(() => {});
		return buf.subarray(0, n);
	} catch {
		return null;
	} finally {
		clearTimeout(timer);
	}
}

// save 把响应的音频写进 file。
async function save(res, file) {
	await new Promise((resolve, reject) => {
		const out = fs.createWriteStream(file);
		const reader = res.body.getReader();
		const pump = () => reader.read().then(({ done, value }) => {
			if (done) {
				out.end(resolve);
				return;
			}
			if (out.write(Buffer.from(value))) pump();
			else out.once("drain", pump);
		}).catch(reject);
		out.on("error", reject);
		pump();
	});
}

// downloadTo 把 url 完整抓到 file。
async function downloadTo(url, file) {
	const ctl = new AbortController();
	const timer = setTimeout(() => ctl.abort(), 180000);
	try {
		const res = await fetchAudio(url, ctl.signal);
		if (!res.ok || !res.body) throw new Error(`下载失败（${res.status}）`);
		await save(res, file);
		const size = fs.statSync(file).size;
		if (size < 1024) throw new Error("下载到的音频是空的");
	} finally {
		clearTimeout(timer);
	}
}

// run 跑一个外部命令，超时就放弃。
function run(cmd, args, timeout = 120000) {
	return new Promise((resolve, reject) => {
		execFile(cmd, args, { timeout, maxBuffer: 4 << 20 }, (err, stdout, stderr) => {
			if (err) {
				const why = String(stderr || err.message || "").trim().slice(0, 200);
				reject(new Error(why || `${cmd} 失败`));
				return;
			}
			resolve(stdout);
		});
	});
}

// have 看某个命令在不在。
async function have(cmd) {
	try {
		await run("/usr/bin/env", ["sh", "-c", `command -v ${cmd}`], 5000);
		return true;
	} catch {
		return false;
	}
}

// toFlac 把任意容器转成 FLAC，返回输出路径。
async function toFlac(src, out) {
	if (process.platform === "darwin" && (await have("afconvert"))) {
		await run("afconvert", ["-f", "flac", "-d", "flac", src, out]);
	} else if (await have("ffmpeg")) {
		await run("ffmpeg", ["-y", "-loglevel", "error", "-i", src, "-vn", "-c:a", "flac", out]);
	} else {
		throw new Error("这台电脑没有可用的音频转码工具（afconvert / ffmpeg），无法播放汽水的 AAC 音源");
	}
	if (!fs.existsSync(out) || fs.statSync(out).size < 1024) throw new Error("转码没有产出音频");
}

// prune 只留最近的 maxFiles 个文件。
function prune() {
	try {
		const files = fs.readdirSync(dir)
			.map((name) => ({ name, at: fs.statSync(path.join(dir, name)).mtimeMs }))
			.sort((a, b) => b.at - a.at);
		for (const f of files.slice(maxFiles)) {
			try {
				fs.unlinkSync(path.join(dir, f.name));
			} catch {}
		}
	} catch {}
}

// prepare 保证 key 对应的音频已经就绪，返回 {file, type}。
// type 是 flac 或 mp3：播放器认的两种格式。
async function prepare(sourceUrl, key) {
	const flacFile = path.join(dir, key + ".flac");
	const mp3File = path.join(dir, key + ".mp3");
	if (fs.existsSync(flacFile)) {
		touch(flacFile);
		return { file: flacFile, type: "flac" };
	}
	if (fs.existsSync(mp3File)) {
		touch(mp3File);
		return { file: mp3File, type: "mp3" };
	}
	const part = path.join(dir, key + ".part");
	await downloadTo(sourceUrl, part);
	try {
		// 下来就是播放器认的容器，改名留着，不必再转码。
		const kind = sniffFile(part);
		if (kind === "flac") {
			fs.renameSync(part, flacFile);
			prune();
			return { file: flacFile, type: "flac" };
		}
		if (kind === "mp3") {
			fs.renameSync(part, mp3File);
			prune();
			return { file: mp3File, type: "mp3" };
		}
		await toFlac(part, flacFile);
	} finally {
		try {
			fs.unlinkSync(part);
		} catch {}
	}
	prune();
	if (fs.existsSync(flacFile)) return { file: flacFile, type: "flac" };
	if (fs.existsSync(mp3File)) return { file: mp3File, type: "mp3" };
	throw new Error("转码没有产出音频");
}

function touch(file) {
	const now = new Date();
	try {
		fs.utimesSync(file, now, now);
	} catch {}
}

// url 返回某份转换结果的播放地址（给播放器拉）。
function url(key) {
	return `${base}/media/${key}`;
}

// serve 把转好的文件写给播放器，支持 Range，便于跳转。
function serve(req, res, key) {
	if (!/^[a-f0-9]{8,40}$/.test(key)) {
		res.writeHead(404);
		res.end();
		return true;
	}
	let file = path.join(dir, key + ".flac");
	if (!fs.existsSync(file)) file = path.join(dir, key + ".mp3");
	if (!fs.existsSync(file)) {
		res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
		res.end("not found");
		return true;
	}
	const size = fs.statSync(file).size;
	const type = file.endsWith(".flac") ? "audio/flac" : "audio/mpeg";
	touch(file);
	const head = { "Content-Type": type, "Accept-Ranges": "bytes" };
	const range = req.headers.range;
	if (range) {
		const m = /^bytes=(\d*)-(\d*)$/.exec(range);
		if (m) {
			let start = m[1] ? Number(m[1]) : 0;
			let end = m[2] ? Number(m[2]) : size - 1;
			if (Number.isNaN(start) || Number.isNaN(end) || start > end || start >= size) {
				res.writeHead(416, { "Content-Range": `bytes */${size}` });
				res.end();
				return true;
			}
			end = Math.min(end, size - 1);
			res.writeHead(206, {
				...head,
				"Content-Range": `bytes ${start}-${end}/${size}`,
				"Content-Length": end - start + 1,
			});
			fs.createReadStream(file, { start, end }).pipe(res);
			return true;
		}
	}
	res.writeHead(200, { ...head, "Content-Length": size });
	fs.createReadStream(file).pipe(res);
	return true;
}

module.exports = { sniff, sniffFile, prepare, url, serve, setBase, keyOf };
