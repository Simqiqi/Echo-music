import { createFileRoute } from '@tanstack/solid-router'
import { For, Show, createSignal, onMount } from 'solid-js'
import { Layout } from '../components/Layout'
import { Icon } from '../components/Icon'
import { origin, release, site } from '../site'

export const Route = createFileRoute('/download')({
  head: () => ({ meta: [{ title: `下载 · ${site.name}` }] }),
  component: Download,
})

type OS = 'mac' | 'windows' | 'linux'

// A download of a platform: the asset of the latest release it is, found by
// its name, as `mygo build` names installers.
type Build = { label: string; kind: string; match: (name: string) => boolean; primary?: boolean }

const has = (...parts: string[]) => (name: string) => {
  const n = name.toLowerCase()
  return parts.every((p) => n.includes(p))
}

const platforms: { os: OS; name: string; note: string; builds: Build[] }[] = [
  {
    os: 'mac',
    name: 'macOS',
    note: 'macOS 13 或更新。本版没有 Apple 公证，首次打开要放行一次（见下）',
    builds: [
      { label: 'Apple 芯片', kind: '.dmg', match: has('arm64', '.dmg'), primary: true },
      { label: 'Intel', kind: '.dmg', match: has('amd64', '.dmg') },
    ],
  },
  {
    os: 'windows',
    name: 'Windows',
    note: '安装到当前用户，无需管理员权限。首次运行会被 SmartScreen 拦一下',
    builds: [
      { label: 'x64', kind: '安装程序', match: has('setup', 'amd64', '.exe'), primary: true },
      { label: 'ARM64', kind: '安装程序', match: has('setup', 'arm64', '.exe') },
    ],
  },
  {
    os: 'linux',
    name: 'Linux',
    note: 'Debian 与 Ubuntu 用 .deb；其他发行版用压缩包或下面的命令',
    builds: [
      { label: 'x64', kind: '.deb', match: has('amd64', '.deb'), primary: true },
      { label: 'ARM64', kind: '.deb', match: has('arm64', '.deb') },
      { label: 'x64', kind: '.tar.gz', match: has('linux', 'amd64', '.tar.gz') },
      { label: 'ARM64', kind: '.tar.gz', match: has('linux', 'arm64', '.tar.gz') },
    ],
  },
]

const installSh = `curl -fsSL ${origin}/install.sh | sh`
const installPs = `irm ${origin}/install.ps1 | iex`
const unquarantine = `xattr -dr com.apple.quarantine "/Applications/${site.name}.app"`

// A build's asset; a build the release has none of links to the release.
const asset = (b: Build) => release.assets.find((a) => b.match(a.name))

// The file's own address on this site: the Worker resolves it to the release
// it really lives in, so the page never names the host behind it.
const link = (b: Build) => {
  const a = asset(b)
  return a ? `/dl/${release.tag_name}/${encodeURIComponent(a.name)}` : '/download'
}

function detectOS(): OS | undefined {
  const ua = navigator.userAgent
  if (/iPhone|iPad|Android/.test(ua)) return undefined
  if (/Mac/.test(ua)) return 'mac'
  if (/Windows/.test(ua)) return 'windows'
  if (/Linux|X11/.test(ua)) return 'linux'
}

function megabytes(bytes: number) {
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function day(iso: string) {
  const d = new Date(iso)
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`
}

function Download() {
  // The page is rendered at build time; the system of the visitor comes
  // once it runs in the browser.
  const [os, setOS] = createSignal<OS>()
  onMount(() => setOS(detectOS()))

  return (
    <Layout>
      <section class="download">
        <div class="section-head download-head">
          <p class="eyebrow">下载</p>
          <h1 class="display">
            <span>把音乐，</span>
            <span>装进桌面。</span>
          </h1>
          <p class="fine download-version">
            版本 {release.tag_name.replace(/^v/, '')} · 发布于 {day(release.published_at)}
          </p>
        </div>

        <div class="platforms">
          <For each={platforms}>
            {(p) => (
              <article class="platform">
                <div class="platform-head">
                  <h2>{p.name}</h2>
                  <Show when={os() === p.os}>
                    <span class="pill">你的系统</span>
                  </Show>
                </div>
                <p class="platform-note">{p.note}</p>
                <div class="platform-builds">
                  <For each={p.builds}>
                    {(b) => (
                      <a
                        class="build"
                        classList={{ primary: b.primary }}
                        href={link(b)}
                      >
                        <Icon name="download" size={16} />
                        <span class="build-label">{b.label}</span>
                        <span class="build-kind">{b.kind}</span>
                        <Show when={asset(b)}>{(a) => <span class="build-size">{megabytes(a().size)}</span>}</Show>
                      </a>
                    )}
                  </For>
                </div>
              </article>
            )}
          </For>
        </div>

        <p class="fine download-mirror">
          链接都走本站自己的地址，文件也由本站直接送达，国内通常能跑满带宽。
        </p>

        <section class="gatekeeper" aria-labelledby="gatekeeper-title">
          <div class="gatekeeper-head">
            <Icon name="shield" size={18} />
            <h2 id="gatekeeper-title">第一次打开要放行一次</h2>
          </div>
          <p class="gatekeeper-lede">
            安装包没有 Apple 开发者证书与 Windows 代码签名证书，系统和浏览器会多问一句。放行一次就好，应用照常运行，之后的自动更新也不受影响。
          </p>
          <div class="gatekeeper-grid">
            <div>
              <h3>macOS</h3>
              <ol>
                <li>
                  把应用拖进「应用程序」，按住 <kbd>Control</kbd> 点按图标，选「打开」，再点一次「打开」。
                </li>
                <li>
                  也可以在系统弹出的提示里点「仍要打开」，或到「系统设置 → 隐私与安全性」底部点「仍要打开」。
                </li>
                <li>若提示「已损坏，无法打开」，多半是下载时带上了隔离标记，执行下面这行再去打开。</li>
              </ol>
              <Command label="解除隔离标记" text={unquarantine} />
            </div>
            <div>
              <h3>Windows</h3>
              <ol>
                <li>SmartScreen 提示「Windows 已保护你的电脑」时，点「更多信息」，再点「仍要运行」。</li>
                <li>浏览器若把安装程序标成「不常下载」，在下载栏里选「保留」即可。</li>
                <li>安装到当前用户目录，不需要管理员权限，也不会改系统的 PATH 之外的东西。</li>
              </ol>
            </div>
          </div>
          <p class="gatekeeper-fine">
            用下面的命令行安装的话，文件不经过浏览器，一般不会触发这些提示。
          </p>
        </section>

        <div class="install">
          <h2>用一行命令安装</h2>
          <p>下载最新版本，装到你的用户目录，以后同样自动更新。</p>
          <Command label="macOS 与 Linux" text={installSh} />
          <Command label="Windows PowerShell" text={installPs} />
        </div>

        <p class="fine download-more">
          装好之后会自动更新，不用再回来下载一次。
        </p>
      </section>
    </Layout>
  )
}

function Command(props: { label: string; text: string }) {
  const [copied, setCopied] = createSignal(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  const copy = () => {
    navigator.clipboard?.writeText(props.text).then(() => {
      setCopied(true)
      clearTimeout(timer)
      timer = setTimeout(() => setCopied(false), 1600)
    })
  }
  return (
    <div class="command">
      <span class="command-label">{props.label}</span>
      <div class="command-line">
        <code>{props.text}</code>
        <button type="button" class="command-copy" onClick={copy} aria-label={`拷贝${props.label}`}>
          {copied() ? '已拷贝' : '拷贝'}
        </button>
      </div>
    </div>
  )
}
