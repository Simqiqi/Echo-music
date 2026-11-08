// Qishui (汽水音乐) login bridge.
//
// The signing lives inside qishui-music-api; we only drive its QR flow and
// keep the session it produces. A session store backs the package so the
// credentials it derives from a confirmed scan survive inside the process
// and can be handed back to the host to persist.
const { createQishuiApi, createQishuiDeviceId } = require('qishui-music-api');
const media = require('../media.cjs');

class SessionStore {
	constructor() {
		this._deviceId = createQishuiDeviceId();
		this._revision = 0;
		this._session = null;
	}
	get deviceId() {
		return this._deviceId;
	}
	get revision() {
		return this._revision;
	}
	get() {
		return this._session;
	}
	set(session) {
		this._session = session;
		this._revision++;
	}
	clear() {
		this._session = null;
		this._revision++;
	}
}

// 抖音系图片有三种长相：完整地址的字符串；头像那样的 urls[0] 已是完整
// 地址（uri 为空）；封面那样的 urls 只是主机前缀，要拼上 uri 再加
// 「~模板-image.image」后缀才取得到（直接拼主机+uri 会 400）。
function imgUrl(value) {
	if (typeof value === "string") return value;
	if (value && typeof value === "object") {
		const urls = Array.isArray(value.urls) ? value.urls : [];
		const host = urls.find((u) => typeof u === "string" && u.startsWith("http")) || "";
		if (!host) return "";
		const uri = typeof value.uri === "string" ? value.uri : "";
		// urls[0] 已含 uri（头像）或带查询参数时它就是完整地址；只有真正
		// 的「主机前缀 + uri」（封面）才需要拼接与模板后缀。
		if (!uri || host.includes(uri) || host.includes("?")) return host;
		if (/^https?:\/\//.test(uri)) return uri;
		const tpl = value.template_prefix ? `~${value.template_prefix}-image.image` : "";
		return host + uri + tpl;
	}
	return "";
}

const avatarUrl = imgUrl;

// 歌单/曲目的字段分散在 base_info 等子对象里，各端点不一，逐个挑第一个有的。
function firstOf(obj, keys) {
	const o = obj && typeof obj === "object" ? obj : {};
	for (const k of keys) {
		const v = o[k];
		if (v != null && v !== "") return v;
	}
	return undefined;
}

// 安全数值：转不动（NaN/undefined）就给 0，避免 NaN 序列化成 null。
function num(value) {
	const n = Number(value);
	return Number.isFinite(n) ? n : 0;
}

// 把包返回的歌单行规整成桥接的统一形状。
function rowPlaylist(row) {
	const pl = row?.playlist || row || {};
	const base = pl.base_info || pl;
	const id = String(firstOf(base, ["id", "playlist_id"]) ?? "");
	if (!id) return null;
	const cnt = pl.resource_cnt && typeof pl.resource_cnt === "object"
		? firstOf(pl.resource_cnt, ["track_cnt", "media_count"])
		: firstOf(base, ["track_count", "media_count", "count"]);
	return {
		id,
		name: String(firstOf(base, ["title", "name"]) ?? "未命名歌单"),
		cover: imgUrl(pl.display_info?.url_cover ?? firstOf(base, ["cover_url", "cover", "pic"])),
		count: num(cnt),
		isOwned: Boolean(pl.isOwned ?? row?.isOwned),
	};
}

// 把包返回的曲目行规整成桥接的统一形状（汽水只需 id，URL 由 audio 取）。
function rowTrack(item) {
	const t = item?.track_entity || item?.entity?.media?.track_entity || item?.entity?.track || item?.track || item;
	const base = t?.base_info || t || {};
	const id = String(firstOf(base, ["id"]) ?? "");
	if (!id) return null;
	const rel = t?.related_info || {};
	const artists = (Array.isArray(rel.artist_links) ? rel.artist_links : [])
		.map(a => a && a.name).filter(Boolean).join("/");
	return {
		id,
		name: String(firstOf(base, ["name", "title"]) ?? ""),
		artist: artists,
		album: String(rel.album_link?.name ?? ""),
		duration: num(firstOf(base, ["duration_ms", "duration"])),
		cover: imgUrl(t.display_info?.cover_url ?? rel.album_link?.url_cover),
		hash: "",
		albumAudioId: "",
	};
}

function create() {
	const store = new SessionStore();
	const api = createQishuiApi({
		deviceId: store.deviceId,
		sessions: store,
		log: () => {},
		warn: () => {},
	});

	return {
		name: 'qishui',

		// Begin a QR login: ask Douyin for a code, then read its image back.
		async start() {
			const key = await api.request('login_qr_key');
			if (!key.ok) return { error: key.error };
			const created = await api.request('login_qr_create', { key: key.data.key });
			if (!created.ok) return { error: created.error };
			const imageUrl = created.data.imageUrl || '';
			const comma = imageUrl.indexOf(',');
			const image = comma >= 0 ? imageUrl.slice(comma + 1) : imageUrl;
			return { key: key.data.key, image };
		},

		// Poll the scan state; 'confirmed' means the session store now holds
		// the account credentials.
		async check(body) {
			const r = await api.request('login_qr_check', { key: body.key });
			if (!r.ok) return { state: 'error', error: r.error };
			return { state: r.data.state };
		},

		// Who is signed in. The cached session answers at once, but its
		// avatar is a URL the image CDN signed with an expiry, so ask the
		// account endpoint for a fresh one; offline, the cache still shows.
		async status() {
			const s = store.get();
			if (!s || !s.userId) return null;
			let u = s.user || s;
			try {
				const fresh = await api.request("login_status");
				if (fresh.ok && fresh.data && fresh.data.id) u = fresh.data;
			} catch {}
			return {
				id: String(u.id ?? s.userId ?? ""),
				nickname: String(u.nickname ?? ""),
				avatar: imgUrl(u.avatar ?? s.avatar ?? ""),
				isVip: Boolean(u.isVip ?? s.isVip ?? false),
			};
		},

		// 当前账号的歌单（自己创建的与收藏的），分页。
		async playlists(body) {
			const r = await api.request("user_playlists", {
				limit: Math.min(Number(body?.limit) || 30, 100),
				offset: Number(body?.offset) || 0,
			});
			if (!r.ok) return { error: r.error?.message || r.error?.code || "获取歌单失败" };
			const items = (r.data?.items || []).map(rowPlaylist).filter(Boolean);
			return { items, hasMore: Boolean(r.data?.hasMore), nextOffset: Number(r.data?.nextOffset) || 0 };
		},

		// 一份歌单里的曲目，分页；ownerId 可不传。
		async playlist(body) {
			const r = await api.request("playlist_tracks", {
				id: String(body?.id ?? ""),
				ownerId: String(body?.ownerId ?? ""),
				limit: Math.min(Number(body?.limit) || 100, 100),
				offset: Number(body?.offset) || 0,
			});
			if (!r.ok) return { error: r.error?.message || r.error?.code || "获取歌单曲目失败" };
			const items = (r.data?.items || []).map(rowTrack).filter(Boolean);
			return { items, hasMore: Boolean(r.data?.hasMore), nextOffset: Number(r.data?.nextOffset) || 0 };
		},

		// 一首歌的播放地址。quality: standard|high|lossless|hires。
		//
		// 汽水给的是 MP4 容器里的 AAC，播放器解不了；而 CDN 直链既带时效，
		// 又只认抖音自家的请求头，播放器拿着它去拉常被回 403（用户看到的是
		// "加载失败，再点一次试试"）。所以不论什么容器，都先由桥接把整首取
		// 回本地（MP4/AAC 顺便转成 FLAC），播放器只跟本机地址打交道。
		async url(body) {
			const quality = ["standard", "high", "lossless", "hires"].includes(body?.quality) ? body.quality : "high";
			const id = String(body?.id ?? "");
			const r = await api.request("audio", { id, quality });
			if (!r.ok) return { error: r.error?.message || r.error?.code || "获取播放地址失败" };
			if (!r.data?.url) return { error: "这首歌暂时无法播放" };
			const src = String(r.data.url);
			const preview = Boolean(r.data.preview);
			const key = media.keyOf("qishui", id, quality);
			const made = await media.prepare(src, key);
			return {
				url: media.url(key),
				type: made.type,
				quality: String(r.data.quality || ""),
				preview,
			};
		},

		// 一首歌的歌词。汽水给的是逐字时间轴：一行是
	// 「[起,长]<偏移,长,0>字<偏移,长,0>字…」，偏移相对行首；换成网易云
	// YRC 的写法（圆括号、字的起点为绝对毫秒），应用现有的解析器直接认。
	async lyric(body) {
		const id = String(body?.id ?? "");
		if (!id) return { error: "缺少歌曲" };
		const r = await api.request("lyrics", { id });
		if (!r.ok) return { error: r.error?.message || r.error?.code || "获取歌词失败" };
		const content = String(r.data?.lyric_entity?.content ?? "");
		if (!content) return { error: "这首歌没有歌词" };
		// 逐行换算：把 <偏移,长,0> 变成 (行起+偏移,长,0)。
		const yrc = content.split("\n").map((line) => {
			const head = line.match(/^\[(\d+),(\d+)\]/);
			if (!head) return line;
			const start = Number(head[1]);
			return line.replace(/<(\d+),(\d+),\d+>/g, (_, off, dur) => `(${start + Number(off)},${dur},0)`);
		}).join("\n");
		return { lrc: yrc, format: "yrc" };
	},

	// The credentials to persist. Null when nobody is signed in.
		session() {
			return store.get();
		},

		// Restore a previously persisted session into the store.
		restore(session) {
			if (session && typeof session === 'object' && session.userId) {
				store.set(session);
				return true;
			}
			return false;
		},

		async logout() {
			await api.request('logout');
			return { ok: true };
		},
	};
}

module.exports = { create };
