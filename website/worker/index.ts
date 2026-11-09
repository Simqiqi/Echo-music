// Everything on the site is a static file, except the download addresses.
// Those are answered by this Worker, which streams the file itself: the
// browser never leaves this domain, so a flaky third-party accelerator can
// no longer leave a click with nothing coming down. The file is fetched from
// where the releases live — reachable from the edge, though not always from
// a visitor — and an accelerator is only the fallback if even that fails.
const upstream = 'https://github.com/Simqiqi/Echo-music'
const accelerator = 'https://gh-proxy.com/'

type Env = { ASSETS: { fetch(request: Request): Promise<Response> } }

// The file, streamed through with its own headers kept (length, ranges) and
// an attachment disposition added, so the browser saves rather than opens it.
async function relay(url: string, name: string, request: Request): Promise<Response> {
  try {
    const range = request.headers.get('range')
    const r = await fetch(url, { redirect: 'follow', headers: range ? { range } : undefined })
    if (!r.ok || !r.body) throw new Error(`upstream ${r.status}`)
    const h = new Headers()
    h.set('content-type', r.headers.get('content-type') ?? 'application/octet-stream')
    for (const k of ['content-length', 'content-range', 'etag', 'last-modified']) {
      const v = r.headers.get(k)
      if (v) h.set(k, v)
    }
    h.set('accept-ranges', r.headers.get('accept-ranges') ?? 'bytes')
    h.set('content-disposition', `attachment; filename="${name}"`)
    h.set('cache-control', 'public, max-age=86400')
    return new Response(r.body, { status: r.status, headers: h })
  } catch {
    // The edge could not reach the file after all; send the browser to the
    // accelerator, the way it went before.
    return Response.redirect(accelerator + url, 302)
  }
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const { pathname } = new URL(request.url)

    // /dl/<tag>/<file>, the installer a build on the download page links to.
    const file = /^\/dl\/([^/]+)\/(.+)$/.exec(pathname)
    if (file) {
      const [, tag, name] = file
      return relay(`${upstream}/releases/download/${tag}/${decodeURIComponent(name)}`, decodeURIComponent(name), request)
    }

    // /install.sh and /install.ps1, the one-line installers.
    if (pathname === '/install.sh' || pathname === '/install.ps1') {
      return relay(`${upstream}/releases/latest/download${pathname}`, pathname.slice(1), request)
    }

    // 改版首页：/ 与 /download 都由静态文件 /echo-redesign.html 提供。
    // 这个文件在 website/public 下，随构建一起进 dist/client。
    if (pathname === '/' || pathname === '/download') {
      return env.ASSETS.fetch(new URL('/echo-redesign.html', request.url).toString())
    }

    return env.ASSETS.fetch(request)
  },
}
