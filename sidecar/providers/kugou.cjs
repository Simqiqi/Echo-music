// 酷狗音乐 (KuGou) 登录桥接。
//
// 直接使用 kugoumusicapi@1.6.0（MakcRe 分支，与 folia-major 同款）完成扫码登录，
// 仅维护一份 cookie（设备标识 + 登录后的 token/userid），登录成功后交回给 Go 侧持久化。
// 概念版(lite)的 token 与完整版互不通用，故进程固定 process.env.platform='lite'。
"use strict";

const crypto = require("node:crypto");
const api = require("kugoumusicapi");
const media = require("../media.cjs");

function genDevice() {
	const guid = crypto.randomUUID().replace(/-/g, "").toUpperCase();
	const mid = BigInt("0x" + crypto.createHash("md5").update(guid).digest("hex")).toString();
	const dev = crypto.randomBytes(5).toString("hex").toUpperCase();
	const mac = crypto
		.randomBytes(6)
		.toString("hex")
		.toUpperCase()
		.match(/.{1,2}/g)
		.join(":");
	const webgl = BigInt("0x" + crypto.randomBytes(8).toString("hex")).toString();
	return {
		KUGOU_API_PLATFORM: "lite",
		KUGOU_API_GUID: guid,
		KUGOU_API_MID: mid,
		KUGOU_API_DEV: dev,
		KUGOU_API_MAC: mac,
		KUGOU_API_WEBGL: webgl,
	};
}

// 把包返回的 cookie 字符串数组合入本进程维护的 cookie 对象。
function mergeCookie(cookie, arr) {
	if (!Array.isArray(arr)) return;
	for (const entry of arr) {
		const i = entry.indexOf("=");
		if (i <= 0) continue;
		cookie[entry.slice(0, i).trim()] = entry.slice(i + 1).trim();
	}
}

// 从包返回的 body 里挑出第一个像样的字段值。
function pick(body, keys) {
	const d = body && (body.data || body);
	for (const k of keys) {
		const v = d && (d[k] ?? (d.data && d.data[k]));
		if (v != null && v !== "") return v;
	}
	return undefined;
}

// 在任意嵌套结构里找第一个数组（酷狗各端点的列表字段不一：info/items/lists）。
function findList(value, depth = 0) {
	if (!value || typeof value !== "object" || depth > 4) return null;
	if (Array.isArray(value)) return value;
	for (const k of ["info", "items", "lists", "list", "songs", "data", "messages"]) {
		const hit = findList(value[k], depth + 1);
		if (hit) return hit;
	}
	for (const v of Object.values(value)) {
		if (Array.isArray(v) && v.length && typeof v[0] === "object") return v;
	}
	return null;
}

// playUrl 从 song_url 的返回体里取播放地址：只认 url / backup_url，
// 不能深扫——返回体里还挂着封面等别的 http 地址。
function playUrl(body) {
	const d = (body && (body.data || body)) || {};
	for (const k of ["url", "backup_url", "urls", "backup_urls"]) {
		const v = d[k];
		if (typeof v === "string" && v.startsWith("http")) return v;
		if (Array.isArray(v)) {
			for (const s of v) if (typeof s === "string" && s.startsWith("http")) return s;
		}
	}
	return "";
}

// kgImg 把封面地址里的 {size} 换成实际尺寸：酷狗的图片 CDN 用它做占位。
function kgImg(value) {
	const s = String(value ?? "");
	return s.replace("{size}", "240");
}

// 安全数值：转不动（NaN/undefined）就给 0，避免 NaN 序列化成 null。
function num(value) {
	const n = Number(value);
	return Number.isFinite(n) ? n : 0;
}

// playableOf 保证地址是播放器认的格式，并且是本机地址。
async function playableOf(url, key) {
	// 酷狗的直链有时效，也只认自家来路：手里的地址交给播放器去拉，常被
	// CDN 回 403（"加载失败，再点一次试试"）。一律由桥接把整首取回本地，
	// 播放器只跟本机地址打交道。
	const made = await media.prepare(url, key);
	return { url: media.url(key), type: made.type };
}

// errText 把上游包的错误说成一句人话（包常把响应体挂在 err.body 上）。
function errText(err) {
	if (!err) return "未知错误";
	if (typeof err === "string") return err;
	if (err.message) return String(err.message);
	const b = err.body;
	if (b && typeof b === "object") {
		const code = b.error_code ?? b.errcode ?? b.code ?? b.status;
		const msg = b.error_msg || b.msg || b.error;
		if (msg || code) return [msg, code ? `(${code})` : ""].filter(Boolean).join(" ");
	}
	return "上游返回错误";
}

// 把酷狗歌单行规整成桥接的统一形状。
function rowPlaylist(row) {
	const id = String(pick(row, ["id", "playlist_id", "global_collection_id", "special_id"]) ?? "");
	if (!id) return null;
	return {
		id,
		name: String(pick(row, ["name", "title"]) ?? "未命名歌单"),
		cover: kgImg(pick(row, ["cover", "imgurl", "img", "pic", "scover"])),
		count: num(pick(row, ["count", "track_count", "song_count"])),
		isOwned: true,
	};
}

// 把酷狗曲目行规整成统一形状；hash 与 album_audio_id 是取播放地址的凭据。
function rowTrack(d) {
	const id = String(pick(d, ["album_audio_id", "audio_id", "id"]) ?? "");
	const hash = String(pick(d, ["hash", "FileHash", "fileHash"]) ?? "");
	if (!id && !hash) return null;
	const name = String(pick(d, ["name", "songname", "filename", "title"]) ?? "");
	// filename 常是「歌手 - 歌名」，没有单独的歌手字段时拆开它。
	let artist = String(pick(d, ["singer_name", "author_name", "singername"]) ?? "");
	let songName = name;
	if (!artist && name.includes(" - ")) {
		const i = name.indexOf(" - ");
		artist = name.slice(0, i);
		songName = name.slice(i + 3);
	}
	// duration 是秒，timelen 是毫秒；统一成毫秒。
	const secs = Number(pick(d, ["duration"]) ?? 0);
	const ms = secs > 0 ? secs * 1000 : Number(pick(d, ["timelen"]) ?? 0);
	return {
		id,
		name: songName,
		artist,
		album: String(pick(d, ["album_name", "remark"]) ?? ""),
		duration: num(ms),
		cover: kgImg(pick(d, ["cover", "imgurl", "img", "scover", "trans_param.cover"])),
		hash,
		albumAudioId: id,
	};
}

// 酷狗的歌词搜索与下载走 lyrics.kugou.com 的明文接口，无需登录态。
// 包里的 search_lyric 指向的 /v1/search 端点已下线，这里直接调在服务的
// 老端点（/search、/download）。
async function kgSearchLyric(hash, durationMs, keyword) {
	const qs = new URLSearchParams({
		ver: "1",
		man: "yes",
		client: "pc",
		keyword: keyword || "",
		duration: String(Math.max(0, Math.round(durationMs || 0))),
		hash: hash || "",
	});
	const r = await fetch(`https://lyrics.kugou.com/search?${qs}`);
	const j = await r.json();
	return Array.isArray(j?.candidates) ? j.candidates : [];
}

async function kgDownloadLyric(id, accesskey) {
	const qs = new URLSearchParams({
		ver: "1", client: "pc", id: String(id), accesskey: String(accesskey),
		fmt: "lrc", charset: "utf8",
	});
	const r = await fetch(`https://lyrics.kugou.com/download?${qs}`);
	const j = await r.json();
	const d = j?.data || j || {};
	if (typeof d.content !== "string" || !d.content) return "";
	try {
		return Buffer.from(d.content, "base64").toString();
	} catch {
		return "";
	}
}

function create() {
	const cookie = genDevice();

	async function start() {
		const keyResp = await api.login_qr_key({ cookie });
		const key = pick(keyResp.body, ["qrcode_key", "key", "qrcode"]);
		if (key == null) {
			return { error: "无法解析扫码密钥", sample: JSON.stringify(keyResp.body).slice(0, 200) };
		}
		const createResp = await api.login_qr_create({ key: String(key), cookie, qrimg: true });
		const data = createResp.body && (createResp.body.data || createResp.body);
		// base64 可能是 data URI，剥掉前缀统一成裸 base64。
		let image = data && (data.base64 || data.qrimg);
		if (typeof image === "string" && image.startsWith("data:")) image = image.slice(image.indexOf(",") + 1);
		if (!image) return { error: "无法生成二维码图片" };
		return { key: String(key), image };
	}

	async function check(body) {
		const r = await api.login_qr_check({ key: body.key, cookie });
		mergeCookie(cookie, r.cookie);
		const data = (r.body && (r.body.data || r.body)) || {};
		const status = Number(data.status);
		let state = "waiting";
		if (status === 0) state = "expired";
		else if (status === 1) state = "waiting";
		else if (status === 2) state = "scanned";
		else if (status === 4) {
			state = "confirmed";
			const tok = pick(r.body, ["token"]) || (r.cookie && undefined);
			if (data.token) cookie.token = String(data.token);
			const uid = data.userid ?? data.user_id;
			if (uid) cookie.userid = String(uid);
		}
		return { state };
	}

	async function status() {
		if (!cookie.token || !cookie.userid) return null;
		let nickname = cookie.nickname || "";
		let avatar = cookie.avatar || "";
		try {
			const r = await api.user_detail({ cookie });
			const d = (r.body && (r.body.data || r.body)) || {};
			const inner = d.data || d;
			const id = String(inner.userid ?? inner.user_id ?? cookie.userid ?? "");
			if (id) {
				nickname = String(inner.nickname ?? inner.nick_name ?? nickname);
				avatar = String(inner.imgurl ?? inner.imgUrl ?? inner.avatar ?? avatar);
				// 记下昵称头像，离线或重启恢复后无需再联网即可显示。
				cookie.nickname = nickname;
				cookie.avatar = avatar;
				cookie.userid = id;
			}
		} catch (e) {
			// 离线：退回上次缓存的昵称与头像。
		}
		if (!nickname && !avatar) return null;
		return {
			id: String(cookie.userid ?? ""),
			nickname,
			avatar,
			isVip: Boolean(cookie.isVip),
		};
	}

	return {
		name: "kugou",
		start,
		check,
		status,

		// 当前账号的歌单。与汽水统一用 offset/limit 分页（酷狗按 page 换算）。
		async playlists(body) {
			const pagesize = 30;
			const page = Math.floor(Math.max(0, Number(body?.offset) || 0) / pagesize) + 1;
			const r = await api.user_playlist({ cookie, page, pagesize });
			const rows = findList(r.body);
			if (!rows) return { error: "无法解析歌单列表" };
			const items = rows.map(rowPlaylist).filter(Boolean);
			return { items, hasMore: items.length >= pagesize, nextOffset: (page - 1) * pagesize + items.length };
		},

		// 一份歌单里的曲目，分页。
		async playlist(body) {
			const pagesize = 100;
			const page = Math.floor(Math.max(0, Number(body?.offset) || 0) / pagesize) + 1;
			const r = await api.playlist_track_all({ id: String(body?.id ?? ""), cookie, page, pagesize });
			const rows = findList(r.body);
			if (!rows) return { error: "无法解析歌单曲目" };
			const items = rows.map(rowTrack).filter(Boolean);
			return { items, hasMore: items.length >= pagesize, nextOffset: (page - 1) * pagesize + items.length };
		},

		// 一首歌的播放地址：凭 hash 与 album_audio_id。
		//
		// 酷狗用 body.status 说明能不能播：1 正常，2/3 是版权或会员限制。
		// 拿到的通常是 mp3/flac，直接给；万一是别的容器就本机转一道。
		async url(body) {
			const hash = String(body?.hash ?? "");
			const albumAudioId = Number(body?.albumAudioId) || 0;
			const quality = ["128", "320", "flac", "high"].includes(body?.quality) ? body.quality : "128";
			const key = media.keyOf("kugou", String(body?.id ?? ""), hash, quality);
			let r;
			try {
				r = await api.song_url({ cookie, hash, album_audio_id: albumAudioId, quality });
			} catch (err) {
				const code = err?.body?.error_code;
				if (code === 35104) throw new Error("酷狗：这首歌需要购买或开通会员");
				throw new Error("酷狗：" + errText(err));
			}
			const resBody = r?.body || {};
			let url = playUrl(resBody);
			if (!url) {
				// 版权/会员受限：试一次试听片段，能拿到就先给试听。
				if (resBody.status !== undefined) {
					let trial = "";
					try {
						const t = await api.song_url({ cookie, hash, album_audio_id: albumAudioId, quality, free_part: true });
						trial = playUrl(t?.body || {});
					} catch {}
					if (trial) {
						const playable = await playableOf(trial, key);
						return { url: playable.url, type: playable.type, quality, preview: true };
					}
					throw new Error("酷狗：这首歌需要会员或没有版权，无法播放");
				}
				throw new Error("这首歌暂时无法播放");
			}
			const playable = await playableOf(url, key);
			return { url: playable.url, type: playable.type, quality, preview: false };
		},

		// 一首歌的歌词：按 hash 与时长搜候选（拿着各自的凭据下载 LRC），
		// 时长对得上的优先。搜不到或下载不动时，如实说没有。
		async lyric(body) {
			const hash = String(body?.hash ?? "");
			const durationMs = Number(body?.duration) || 0;
			const keyword = [body?.artist, body?.name].filter(Boolean).join(" ");
			if (!hash && !keyword) return { error: "缺少歌曲" };
			let cands = [];
			try {
				cands = await kgSearchLyric(hash, durationMs, keyword);
			} catch {}
			const fits = cands.filter((c) => c && c.id && c.accesskey &&
				(!durationMs || Math.abs(Number(c.duration) - durationMs) < 2000));
			const cand = fits[0] || cands.find((c) => c && c.id && c.accesskey);
			if (!cand) return { error: "这首歌没有歌词" };
			try {
				const lrc = await kgDownloadLyric(cand.id, cand.accesskey);
				if (lrc.includes("[")) return { lrc, format: "lrc" };
			} catch {}
			return { error: "这首歌没有歌词" };
		},

		session: () => ({ ...cookie }),
		restore: (s) => {
			if (s && typeof s === "object") {
				Object.assign(cookie, s);
				return true;
			}
			return false;
		},
		// kugoumusicapi 没有 logout 模块；登录态主要由本地 cookie 维护，
		// 远端 token 通常会话失效即作废，这里只清除本地凭据。
		async logout() {
			cookie.token = "";
			cookie.userid = 0;
			return { ok: true };
		},
	};
}

module.exports = { create };
