// Echo 云音乐 — 第三方登录桥接进程。
//
// 汽水音乐与酷狗音乐的登录签名逻辑都封装在 Node 包里（qishui-music-api、
// kugoumusicapi），Go 无法直接复现其字节系/酷狗系签名，因此由这个轻量
// Node 进程负责这两家的扫码登录与会话维护，Go 侧只通过本机 HTTP 与它对话。
//
// 启动后在 stdout 第一行打印 {"ready":true,"port":N}，Go 侧据此找到端口。
"use strict";

// 酷狗按"概念版(lite)"运行：token 只在 lite 下生效，且与完整版互不通用。
process.env.platform = "lite";

// 第三方包会往 stdout/stderr 打日志，全部静音，端口改由端口文件传递，
// 避免日志污染 Go 侧对端口的解析。
for (const k of ["log", "info", "warn", "error", "debug", "trace"]) {
	if (typeof console[k] === "function") console[k] = () => {};
}
const fs = require("node:fs");
function reportReady(port) {
	const payload = JSON.stringify({ ready: true, port });
	if (process.env.ECHO_SIDECAR_PORTFILE) {
		try {
			fs.writeFileSync(process.env.ECHO_SIDECAR_PORTFILE, payload);
		} catch {}
	}
	process.stdout.write(payload + "\n");
}

const http = require("node:http");
const media = require("./media.cjs");

const providers = {};
function loadProvider(name) {
	if (providers[name]) return providers[name];
	if (name === "qishui") providers[name] = require("./providers/qishui.cjs").create();
	else if (name === "kugou") providers[name] = require("./providers/kugou.cjs").create();
	return providers[name];
}

function sendJSON(res, code, obj) {
	const body = JSON.stringify(obj);
	res.writeHead(code, { "Content-Type": "application/json; charset=utf-8" });
	res.end(body);
}

// errText 把上游包抛出的错误说成一句人话：包常把响应体挂在 err.body 上，
// 直接 String(err) 只会得到 [object Object]。
function errText(err) {
	if (!err) return "未知错误";
	if (typeof err === "string") return err;
	if (err.message) return String(err.message);
	const body = err.body;
	if (body && typeof body === "object") {
		const code = body.error_code ?? body.errcode ?? body.code ?? body.status;
		const msg = body.error_msg || body.msg || body.error;
		if (msg || code) return [msg, code ? `(${code})` : ""].filter(Boolean).join(" ");
	}
	try {
		return JSON.stringify(err).slice(0, 200);
	} catch {
		return String(err);
	}
}

function readBody(req) {
	return new Promise((resolve) => {
		let data = "";
		req.on("data", (c) => (data += c));
		req.on("end", () => {
			if (!data) return resolve({});
			try {
				resolve(JSON.parse(data));
			} catch {
				resolve({});
			}
		});
	});
}

const ACTIONS = new Set(["start", "check", "status", "session", "restore", "logout",
	"playlists", "playlist", "url", "lyric"]);

const server = http.createServer(async (req, res) => {
	// 仅本机回环访问。
	if (req.socket.remoteAddress && !req.socket.remoteAddress.startsWith("127.")) {
		return sendJSON(res, 403, { error: "forbidden" });
	}
	// 播放器按 url 动作拿到的本机地址，从这里拉转好的音频。
	const url = new URL(req.url, "http://localhost");
	const parts = url.pathname.split("/").filter(Boolean);
	if (parts[0] === "media" && parts.length === 2) {
		media.serve(req, res, parts[1]);
		return;
	}
	if (parts[0] !== "v1" || parts.length !== 3) {
		return sendJSON(res, 404, { error: "not found" });
	}
	const [, provider, action] = parts;
	if (!ACTIONS.has(action)) return sendJSON(res, 404, { error: "unknown action" });
	const inst = loadProvider(provider);
	if (!inst) return sendJSON(res, 404, { error: "unknown provider" });

	const body = req.method === "POST" ? await readBody(req) : {};
	try {
		const result = await inst[action](body || {});
		// A null result (e.g. status with nobody signed in) is meaningful;
		// pass it through rather than coercing to {} so the host can tell
		// "no account" apart from "account with empty fields".
		sendJSON(res, 200, result);
	} catch (err) {
		sendJSON(res, 200, { error: errText(err) });
	}
});

server.listen(0, "127.0.0.1", () => {
	const { port } = server.address();
	media.setBase(`http://127.0.0.1:${port}`);
	reportReady(port);
});

// 父进程退出时随之退出。
function die() {
	server.close();
	process.exit(0);
}
process.on("SIGTERM", die);
process.on("SIGINT", die);
