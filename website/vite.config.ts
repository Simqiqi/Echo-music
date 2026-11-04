import { defineConfig } from 'vite'
import { tanstackStart } from '@tanstack/solid-start/plugin/vite'
import solid from 'vite-plugin-solid'

// The latest release, with its installers, for the download page to link
// to. It's fetched once, as the site is built: a release that is published
// later is linked to once the site is built again. GH_TOKEN or GITHUB_TOKEN,
// if set, is sent with the request, for the API's rate limit.
async function latestRelease() {
  const token = process.env.GH_TOKEN || process.env.GITHUB_TOKEN
  const res = await fetch('https://api.github.com/repos/Simqiqi/simqiqi-music/releases/latest', {
    headers: token ? { authorization: `Bearer ${token}` } : {},
  })
  if (!res.ok) throw new Error(`fetching the latest release: GitHub answered ${res.status}`)
  const r = await res.json()
  return {
    tag_name: r.tag_name,
    published_at: r.published_at,
    // Only what the page shows: the file's name and size. Where the release
    // lives is the Worker's business, so it never reaches the browser.
    assets: r.assets.map((a: any) => ({ name: a.name, size: a.size })),
  }
}

export default defineConfig(async () => ({
  define: { __RELEASE__: JSON.stringify(await latestRelease()) },
  plugins: [
    // Every page is rendered at build time, so the site is plain files
    // that Cloudflare serves as static assets.
    tanstackStart({
      prerender: {
        enabled: true,
        crawlLinks: true,
        failOnError: true,
        // Links to a section of a page are that page; download addresses are
        // answered by the Worker, not by a file, so they aren't pages either.
        filter: ({ path }) =>
          !path.includes('#') && !path.startsWith('/dl/') && !path.startsWith('/install.'),
      },
      // Cloudflare serves 404.html for paths that have no file.
      pages: [{ path: '/404', prerender: { autoSubfolderIndex: false } }],
    }),
    solid({ ssr: true }),
  ],
}))
