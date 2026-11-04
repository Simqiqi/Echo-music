export const repo = 'https://github.com/Simqiqi/simqiqi-music'

// GitHub 的发行文件在国内常常连不上或只有几十 KB/s：下载链接先过一道
// 公共加速，原始地址仍原样留着，加速失效时可以从发行页手动取。更新清单
// 里带的也是这条前缀，所以应用内更新走的是同一条路（见 .github/workflows/
// feed.yml）。
export const mirror = 'https://gh-proxy.com/'

// fast puts a GitHub URL through the accelerator; anything else is left as
// it is.
export const fast = (url: string) => (url.startsWith('https://github.com/') ? mirror + url : url)

export type Asset = { name: string; size: number; browser_download_url: string }
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
