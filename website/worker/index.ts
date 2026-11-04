// Everything on the site is a static file, except the download addresses.
// Those answer with the file's real location instead of a page, so the site
// itself — its markup, its links, the browser's status bar — only ever names
// its own domain. The host the releases are stored on is reachable from here
// through an accelerator, which is where the redirect points.
const upstream = 'https://github.com/Simqiqi/simqiqi-music'
const accelerator = 'https://gh-proxy.com/'

type Env = { ASSETS: { fetch(request: Request): Promise<Response> } }

const away = (from: string) => Response.redirect(accelerator + from, 302)

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const { pathname } = new URL(request.url)

    // /dl/<tag>/<file>, the installer a build on the download page links to.
    const file = /^\/dl\/([^/]+)\/(.+)$/.exec(pathname)
    if (file) {
      const [, tag, name] = file
      return away(`${upstream}/releases/download/${tag}/${decodeURIComponent(name)}`)
    }

    // /install.sh and /install.ps1, the one-line installers.
    if (pathname === '/install.sh' || pathname === '/install.ps1') {
      return away(`${upstream}/releases/latest/download${pathname}`)
    }

    return env.ASSETS.fetch(request)
  },
}
