// QQ 音乐 登录桥接。
//
// QQ 音乐的取播放地址（musicu.fcg 的 vkey）与登录都带签名，Go 侧无法直接
// 复现，故沿用 kugoumusicapi 的老办法：由这个 Node 进程借
// @sansenjian/qq-music-api（MIT，Rain120/qq-music-api 的维护分支）完成
// 扫码登录、歌单读取、取播放地址与歌词，Go 侧只通过本机 HTTP 与它对话。
//
// 登录支持两条通道：QQ 扫码（扫码后换到 y.qq.com 的 cookie）与微信扫码
// （走 open.weixin.qq.com）。两条通道落到同一个会话：uin + cookie，
// 因为进了 QQ 音乐之后要用的是同一份 y.qq.com cookie。
"use strict";

const sdk = require("@sansenjian/qq-music-api/sdk");
const svc = require("@sansenjian/qq-music-api/services");
const media = require("../media.cjs");

// 上游把业务载荷放在 {status, body} 里；错误则可能是 {error} 或抛出来。
function unwrap(r) {
	const body = r && typeof r === "object" && r.body !== undefined ? r.body : r;
	return body || {};
}

// errText 把上游包的错误说成一句人话（包常把响应体挂在 err.body 上）。
function errText(err) {
	if (!err) return "未知错误";
	if (typeof err === "string") return err;
	if (err.message) return String(err.message);
	const b = err.body;
	if (b && typeof b === "object") {
		const code = b.error_code ?? b.errcode ?? b.code ?? b.status;
		const msg = b.error_msg || b.msg || b.error || b.message;
		if (msg || code) return [msg, code ? `(${code})` : ""].filter(Boolean).join(" ");
	}
	return "上游返回错误";
}

// 从包返回的 body 里挑出第一个像样的字段值。
function pick(src, keys) {
	const d = src && (src.data || src);
	for (const k of keys) {
		const v = d && (d[k] ?? (d.data && d.data[k]));
		if (v != null && v !== "") return v;
	}
	return undefined;
}

// 安全数值：转不动（NaN/undefined）就给 0，避免 NaN 序列化成 null。
function num(value) {
	const n = Number(value);
	return Number.isFinite(n) ? n : 0;
}

// 在任意嵌套结构里找第一个数组（QQ 各端点的列表字段不一）。
function findList(value, depth = 0) {
	if (!value || typeof value !== "object" || depth > 4) return null;
	if (Array.isArray(value)) return value;
	for (const k of ["list", "songlist", "songs", "mydiss", "items", "data"]) {
		const hit = findList(value[k], depth + 1);
		if (hit) return hit;
	}
	return null;
}

// qqImg 把专辑 mid 拼成 QQ 的封面地址（300×300 足够列表与播放页用）。
function coverOf(albumMid, fallback) {
	const mid = String(albumMid ?? "").trim();
	if (mid) return `https://y.qq.com/music/photo_new/T002R300x300M000${mid}.jpg`;
	return String(fallback ?? "");
}

// 歌手名从 singer:[{name}] 里拼出来，没有就退回字段本身。
function artistOf(row) {
	const singers = row?.singer || row?.singers;
	if (Array.isArray(singers)) {
		const names = singers.map((s) => (typeof s === "string" ? s : s?.name)).filter(Boolean);
		if (names.length) return names.join("/");
	}
	return String(pick(row, ["singername", "artist", "author"]) ?? "");
}

// 把 QQ 的歌单行规整成桥接的统一形状。
function rowPlaylist(row) {
	const id = String(pick(row, ["dissid", "disstid", "id", "tid"]) ?? "");
	if (!id) return null;
	const cover = pick(row, ["imgurl", "picurl", "cover", "img", "diss_cover"]);
	return {
		id,
		name: String(pick(row, ["dissname", "title", "name"]) ?? "未命名歌单"),
		// 封面地址有时带 {size} 占位（QQ 的图片 CDN 用它做缩放）。
		cover: String(cover ?? "").replace("{size}", "300"),
		count: num(pick(row, ["song_cnt", "songnum", "song_count", "count"])),
		isOwned: true,
	};
}

// 把 QQ 的曲目行规整成统一形状。
// 取播放地址与歌词都只认 songmid；media_mid 是拼下载文件名用的补充凭据。
function rowTrack(row) {
	const mid = String(pick(row, ["songmid", "mid", "songMid"]) ?? "");
	const file = row?.file || {};
	const mediaMid = String(file.media_mid || pick(row, ["media_mid", "mediaMid"]) || "");
	const songId = String(pick(row, ["songid", "songId", "id"]) ?? "");
	const id = mid || songId;
	if (!id) return null;
	const album = row?.album || {};
	const name = String(pick(row, ["songname", "name", "title"]) ?? "");
	// interval 是秒；个别端点给的是毫秒，做个兜底。
	const secs = num(pick(row, ["interval", "duration"]));
	const ms = secs > 0 && secs < 20000 ? secs * 1000 : secs;
	return {
		id,
		name,
		artist: artistOf(row),
		album: String(pick(row, ["albumname", "album_name"]) ?? album?.name ?? ""),
		duration: num(ms),
		cover: coverOf(pick(row, ["albummid", "album_mid"]) ?? album?.mid, pick(row, ["cover", "pic"])),
		hash: mediaMid,
		albumAudioId: songId,
	};
}

// qr 是正在等的二维码：key 前缀区分通道，值里留着轮询要用的凭据。
// QQ 用 qrsig + ptqrtoken，微信用 uuid。
function hash33(s) {
	let h = 0;
	for (let i = 0; i < String(s).length; i++) {
		h += (h << 5) + String(s).charCodeAt(i);
	}
	return 2147483647 & h;
}

function create() {
	// 会话：cookie 是 y.qq.com 的一串 k=v，uin 是登录的 QQ 号。
	const state = {
		cookie: "",
		uin: "",
		nickname: "",
		avatar: "",
		isVip: false,
	};
	const pending = new Map();
	// seenFirst 记住每份歌单第一页的第一首，用来认出「上游没认 song_begin」。
	const seenFirst = new Map();

	function sessionCookie() {
		return state.cookie || "";
	}

	// withCookie 把会话 cookie 交给包：既走 options.cookie，也走 header，
	// 两条路包内部各有一处认（sdk 自己走的是 header）。
	function withCookie(option = {}) {
		const cookie = sessionCookie();
		if (!cookie) return option;
		return {
			...option,
			cookie,
			headers: { ...(option.headers || {}), Cookie: cookie, Referer: "https://y.qq.com/" },
		};
	}

	// adoptSession 收下包换回来的登录态：cookie 串 + uin。
	function adoptSession(session) {
		if (!session) return false;
		const cookie = String(session.cookie || "").trim();
		if (!cookie) return false;
		state.cookie = cookie;
		state.uin = String(session.uin || session.loginUin || "").replace(/^o/, "");
		return true;
	}

	async function start(body) {
		const mode = body?.mode === "wx" ? "wx" : "qq";
		if (mode === "wx") {
			const data = unwrap(await sdk.getWXLoginQr({}));
			const uuid = String(data.uuid || "");
			if (!uuid) return { error: "无法从微信登录页面获取 uuid" };
			pending.set(`wx:${uuid}`, { mode, uuid, at: Date.now() });
			return { key: `wx:${uuid}`, image: String(data.img || "") };
		}
		const data = unwrap(await sdk.getQQLoginQr({}));
		const qrsig = String(data.qrsig || "");
		if (!qrsig) return { error: "无法从 QQ 登录页获取 qrsig" };
		const ptqrtoken = data.ptqrtoken ?? hash33(qrsig);
		pending.set(`qq:${qrsig}`, { mode: "qq", qrsig, ptqrtoken, at: Date.now() });
		return { key: `qq:${qrsig}`, image: String(data.img || "") };
	}

	// wait 是二维码的寿命：超过这个时间就当它过期，不再向上游轮询。
	const qrLifeMs = 5 * 60 * 1000;

	async function check(body) {
		const key = String(body?.key ?? "");
		const held = pending.get(key);
		if (!held) return { state: "expired" };
		if (Date.now() - held.at > qrLifeMs) {
			pending.delete(key);
			return { state: "expired" };
		}
		if (held.mode === "wx") {
			const data = unwrap(await sdk.checkWXLoginQr({ uuid: held.uuid }));
			if (data.isOk) {
				if (!adoptSession(data.session)) return { error: "登录态解析失败" };
				pending.delete(key);
				return { state: "confirmed" };
			}
			if (data.refresh) {
				pending.delete(key);
				return { state: "expired" };
			}
			return { state: data.scanned ? "scanned" : "waiting" };
		}
		const data = unwrap(
			await sdk.checkQQLoginQr({ ptqrtoken: held.ptqrtoken, qrsig: held.qrsig }),
		);
		if (data.isOk) {
			if (!adoptSession(data.session)) return { error: "登录态解析失败" };
			pending.delete(key);
			return { state: "confirmed" };
		}
		if (data.refresh) {
			pending.delete(key);
			return { state: "expired" };
		}
		return { state: "waiting" };
	}

	// profileOf 问一次上游拿昵称与头像；离线就退回上次记住的。
	async function profileOf() {
		if (!state.uin) return null;
		// 头像地址是算出来的，不联网也能给。
		if (!state.avatar) state.avatar = `https://q.qlogo.cn/headimg_dl?dst_uin=${state.uin}&spec=140`;
		try {
			const data = unwrap(await svc.getUserDetail({ uin: state.uin, cookie: sessionCookie() }));
			const creator = data?.data?.creator || data?.creator || {};
			const nick = String(creator.nick || creator.hostname || creator.name || "").trim();
			if (nick) state.nickname = nick;
			const pic = String(creator.headpic || creator.avatarUrl || "").trim();
			if (pic) state.avatar = pic;
			state.isVip = Boolean(creator.isVip ?? creator.vip ?? false);
		} catch {
			// 离线：用记住的那份。
		}
		if (!state.nickname) state.nickname = state.uin ? `QQ ${state.uin}` : "";
		if (!state.nickname && !state.avatar) return null;
		return {
			id: String(state.uin || ""),
			nickname: state.nickname,
			avatar: state.avatar,
			isVip: Boolean(state.isVip),
		};
	}

	// playableOf 一律由桥接把整首取回本地再给播放器：QQ 的直链有时效，
	// 也只认自家来路，手里的地址交给播放器去拉常被 CDN 回 403。
	async function playableOf(url, key) {
		const made = await media.prepare(url, key);
		return { url: media.url(key), type: made.type };
	}

	return {
		name: "qq",
		start,
		check,

		async status() {
			if (!sessionCookie()) return null;
			return profileOf();
		},

		// 当前账号的歌单（含「我喜欢」以外的自建与收藏）。
		// 搜索：QQ 的搜索接口按关键词给歌曲。
		async search(body) {
			const keyword = String(body?.keyword ?? "").trim();
			if (!keyword) return { items: [] };
			const limit = Math.min(50, Math.max(1, Number(body?.limit) || 30));
			const offset = Math.max(0, Number(body?.offset) || 0);
			const page = Math.floor(offset / limit) + 1;
			let data;
			try {
				data = unwrap(
					await svc.getSearchByKey({
						params: { w: keyword, n: limit, p: page, catZhida: 1, remoteplace: "txt.yqq.song" },
						option: withCookie(),
					}),
				);
			} catch (err) {
				return { error: "QQ音乐：" + errText(err) };
			}
			const rows = data?.response?.data?.song?.list || findList(data?.response?.data) || [];
			if (!Array.isArray(rows)) return { error: "无法解析搜索结果" };
			const items = rows.map(rowTrack).filter(Boolean);
			return { items, hasMore: rows.length >= limit, nextOffset: offset + rows.length };
		},

		async playlists(body) {
			if (!sessionCookie()) return { error: "QQ音乐：还没有登录" };
			const limit = Math.min(100, Math.max(1, Number(body?.limit) || 100));
			const offset = Math.max(0, Number(body?.offset) || 0);
			const data = unwrap(
				await svc.getUserPlaylists({
					uin: state.uin,
					offset,
					limit,
					cookie: sessionCookie(),
				}),
			);
			const rows = data?.response?.data?.playlists ?? (findList(data) || []);
			if (!Array.isArray(rows)) return { error: "无法解析歌单列表" };
			const items = rows.map(rowPlaylist).filter(Boolean);
			return {
				items,
				hasMore: rows.length >= limit,
				nextOffset: offset + rows.length,
			};
		},

		// 一份歌单里的曲目，分页（QQ 用 song_begin / song_num）。
		async playlist(body) {
			if (!sessionCookie()) return { error: "QQ音乐：还没有登录" };
			const id = String(body?.id ?? "");
			if (!id) return { error: "缺少歌单 ID" };
			const limit = Math.min(200, Math.max(1, Number(body?.limit) || 100));
			const offset = Math.max(0, Number(body?.offset) || 0);
			const data = unwrap(
				await svc.songListDetail({
					method: "get",
					params: { disstid: id, song_begin: offset, song_num: limit },
					option: withCookie(),
				}),
			);
			const res = data?.response || data;
			if (num(res?.code) !== 0 && res?.code !== undefined) {
				return { error: "QQ音乐：" + String(res.msg || res.message || "歌单读取失败") };
			}
			// 曲目有新旧两种形态：老式扁平的回包把整份歌单放在 cdlist[0]，
			// 新式放在 response.data.songlist。以老式为主——c.y.qq.com 这条
			// 老接口至今仍回 cdlist，按新式找会得到一份空歌单。
			const cd = Array.isArray(res?.cdlist) ? res.cdlist[0] : null;
			const rows = (cd && Array.isArray(cd.songlist) && cd.songlist) ||
				res?.data?.songlist || findList(res?.data) || [];
			if (!Array.isArray(rows)) return { error: "无法解析歌单曲目" };
			// 上游偶尔不认 song_begin，把第一页原样再给一遍；认出来就收手，
			// 免得整份歌单被复制成一串重复的曲目。
			const first = rows[0] ? String(pick(rows[0], ["songmid", "mid", "songid", "id"]) ?? "") : "";
			if (offset > 0 && first && seenFirst.get(id) === first) {
				return { items: [], hasMore: false, nextOffset: offset };
			}
			if (offset === 0 && first) seenFirst.set(id, first);
			const page = rows.slice(0, limit);
			const next = offset + page.length;
			const total = num(cd?.songnum ?? res?.data?.dirinfo?.songnum);
			const items = page.map(rowTrack).filter(Boolean);
			return {
				items,
				hasMore: rows.length >= limit && (!total || next < total),
				nextOffset: next,
			};
		},

		// 一首歌的播放地址：凭 songmid（media_mid 作为补充）。
		//
		// 音质逐级降档：320 与 FLAC 要绿钻，没开通时上游给的是一份空 purl
		// （midurlinfo.result 104003），而同一首歌的 128 往往能听；照着能拿到
		// 的那一档给播放器，而不是让整首歌播不出来。
		async url(body) {
			if (!sessionCookie()) throw new Error("QQ音乐：请先登录再播放这首歌");
			const songmid = String(body?.id ?? "");
			if (!songmid) throw new Error("QQ音乐：缺少歌曲标识");
			const mediaId = String(body?.hash ?? "");
			const asked = String(body?.quality ?? "");
			const want = ["128", "320", "flac"].includes(asked) ? asked : "128";
			const ladder = want === "flac" ? ["flac", "320", "128"] : want === "320" ? ["320", "128"] : ["128"];
			let served = "";
			let source = "";
			let lastErr = "";
			for (const quality of ladder) {
				let data;
				try {
					data = unwrap(
						await sdk.getPlayUrl({ songmid, mediaId, quality, cookie: sessionCookie() }),
					);
				} catch (err) {
					lastErr = errText(err);
					continue;
				}
				const entry = data?.data?.playUrl?.[songmid] || {};
				const url = String(entry.url || "");
				if (url) {
					served = quality;
					source = url;
					break;
				}
				lastErr = String(entry.error || "");
			}
			if (!source) {
				if (lastErr && !/登录|Cookie|uin|权限|暂无/.test(lastErr)) {
					throw new Error("QQ音乐：" + lastErr);
				}
				throw new Error("QQ音乐：这首歌需要会员或没有版权，无法播放");
			}
			const key = media.keyOf("qq", songmid, mediaId, served);
			const playable = await playableOf(source, key);
			return { url: playable.url, type: playable.type, quality: served, preview: false };
		},

		// 一首歌的歌词：QQ 的歌词接口按 songmid 给 LRC。
		async lyric(body) {
			const songmid = String(body?.id ?? "");
			if (!songmid) return { error: "这首歌没有歌词" };
			try {
				const data = unwrap(
					await sdk.lyric({ songmid, isFormat: "json", cookie: sessionCookie() }),
				);
				const node = data?.response?.lyric ?? data?.lyric ?? "";
				let lrc = typeof node === "string" ? node : String(node?.lyric ?? "");
				lrc = lrc.replace(/&apos;/g, "'").replace(/&quot;/g, '"').replace(/&amp;/g, "&");
				// 有些回包给的是 base64 过的歌词。
				if (lrc && !lrc.includes("[") && /^[A-Za-z0-9+/=\s]+$/.test(lrc)) {
					try {
						lrc = Buffer.from(lrc, "base64").toString("utf8");
					} catch {}
				}
				if (lrc.includes("[")) return { lrc, format: "lrc" };
			} catch {
				// 落到下面的「没有歌词」。
			}
			return { error: "这首歌没有歌词" };
		},

		session: () => ({ ...state }),
		restore: (s) => {
			if (s && typeof s === "object") {
				Object.assign(state, s);
				return Boolean(state.cookie);
			}
			return false;
		},
		// 包里的登出只是本地忘记：y.qq.com 的 cookie 留在本地，清掉即可。
		async logout() {
			state.cookie = "";
			state.uin = "";
			state.nickname = "";
			state.avatar = "";
			state.isVip = false;
			return { ok: true };
		},
	};
}

module.exports = { create };
