// Where the site lives. Download links are handed out from this address and
// never from the host the releases are stored on: the Worker behind it (see
// worker/index.ts) answers /dl/* and /install.* with the real file location,
// so nothing upstream shows up in the page or in the browser's status bar.
export const origin = 'https://simqiqiecho.dpdns.org'

export type Asset = { name: string; size: number }
export type Release = { tag_name: string; published_at: string; assets: Asset[] }

declare const __RELEASE__: Release
// The latest release, as it was when the site was built (see vite.config.ts).
export const release = __RELEASE__
export const mygo = 'https://mygo.egoist.dev'

export const site = {
  name: 'Echo云音乐',
  title: 'Echo云音乐 — 原生的网易云音乐桌面客户端',
  description:
    '用 Go 与 MyGo 写成的第三方网易云音乐桌面客户端。原生界面，没有 WebView；边下边播，无损与 Hi-Res，逐字歌词与「律动」，刘海播放器。',
}
