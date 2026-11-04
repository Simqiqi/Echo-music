import { Link, createFileRoute } from '@tanstack/solid-router'
import { For, createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import { Layout } from '../components/Layout'
import { AppWindow } from '../components/AppWindow'
import { Stage } from '../components/Stage'
import { LyricStage } from '../components/LyricStage'
import { Icon, type IconName } from '../components/Icon'
import { ThemeSwitch, flipTone } from '../theme'
import { mygo, release, site } from '../site'

export const Route = createFileRoute('/')({
  component: Home,
})

const version = release.tag_name.replace(/^v/, '')

function Home() {
  useScrollReveal()
  return (
    <Layout>
      <Hero />
      <Ticker />
      <Desk />
      <Lyric />
      <Bento />
      <Closing />
    </Layout>
  )
}

// Sections fade up as they arrive, but only when the visitor hasn't asked
// the system for less motion. The hidden state is added here, so a page
// without JavaScript still reads fine.
function useScrollReveal() {
  onMount(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    const targets = Array.from(document.querySelectorAll<HTMLElement>('[data-reveal]'))
    if (!targets.length) return
    for (const el of targets) el.classList.add('will-reveal')
    const watcher = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          entry.target.classList.add('shown')
          watcher.unobserve(entry.target)
        }
      },
      { rootMargin: '0px 0px -10% 0px', threshold: 0.1 },
    )
    for (const el of targets) watcher.observe(el)
    onCleanup(() => watcher.disconnect())
  })
}

// One character per span, so each can arrive on its own beat.
function Chars(props: { text: string; from?: number }) {
  return (
    <>
      {Array.from(props.text).map((ch, i) => (
        <span class="lp-ch" style={{ '--i': `${(props.from ?? 0) + i}` }}>
          {ch}
        </span>
      ))}
    </>
  )
}

function Hero() {
  return (
    <section class="lp-hero">
      <Stage />
      <div class="lp-hero-inner">
        <p class="lp-eyebrow">
          <i class="lp-dot" aria-hidden="true" />
          原生 · 轻量 · 没有 WebView
        </p>
        <h1 class="lp-title">
          <span class="lp-line">
            <Chars text="声音，" />
          </span>
          <span class="lp-line lp-line-accent">
            <Chars text="不该住在浏览器里" from={3} />
          </span>
        </h1>
        <p class="lp-lede">
          用 Go 与 <a href={mygo}>MyGo</a>{' '}
          写成的第三方网易云音乐客户端。原生界面，冷启动不到一秒；Hi-Res 边下边播，逐字歌词跟着人声浮起，闲下来就安静地待在 Dock 里。
        </p>
        <div class="lp-actions">
          <Link class="btn btn-primary lp-btn" to="/download">
            <Icon name="download" size={17} />
            下载
          </Link>
          <button class="btn btn-ghost lp-btn" type="button" onClick={(e) => flipTone(e)}>
            <Icon name="sparkle" size={17} />
            换个主题
          </button>
        </div>
        <p class="lp-fine">
          版本 {version} · 免费 · 与网易云音乐官方无关
        </p>
      </div>
      <div class="lp-scroll" aria-hidden="true">
        <span>向下</span>
        <i />
      </div>
    </section>
  )
}

const ticker = [
  '逐字歌词',
  'Hi-Res 无损',
  '边下边播',
  '刘海播放器',
  '私人 FM',
  '心动模式',
  '离线缓存',
  '扫码登录',
  '每日推荐',
  '律动视图',
]

function Ticker() {
  return (
    <section class="lp-ticker" aria-hidden="true">
      <div class="lp-ticker-track">
        <For each={[...ticker, ...ticker]}>
          {(word) => (
            <span class="lp-ticker-item">
              {word}
              <i />
            </span>
          )}
        </For>
      </div>
    </section>
  )
}

// The app window, tilted a few degrees toward the pointer.
function Desk() {
  let frame: HTMLDivElement | undefined

  const move = (e: PointerEvent) => {
    if (!frame) return
    const box = frame.getBoundingClientRect()
    const x = (e.clientX - box.left) / box.width - 0.5
    const y = (e.clientY - box.top) / box.height - 0.5
    frame.style.setProperty('--ry', `${x * 4.5}deg`)
    frame.style.setProperty('--rx', `${-y * 3.2}deg`)
    frame.style.setProperty('--mx', `${(x + 0.5) * 100}%`)
    frame.style.setProperty('--my', `${(y + 0.5) * 100}%`)
  }
  const rest = () => {
    if (!frame) return
    frame.style.setProperty('--ry', '0deg')
    frame.style.setProperty('--rx', '0deg')
  }

  return (
    <section id="demo" class="lp-desk" data-reveal>
      <div class="lp-desk-head">
        <p class="eyebrow">不是效果图</p>
        <h2 class="display">
          <span>下面这一屏，</span>
          <span>就是它本身。</span>
        </h2>
        <p class="lede">
          用 HTML 照着重画了一遍：播放键、进度条、喜欢都能真的点。封面是光，歌是真的在放。
        </p>
      </div>
      <div
        class="lp-desk-frame"
        ref={frame}
        onPointerMove={move}
        onPointerLeave={rest}
        style={{ '--rx': '0deg', '--ry': '0deg' }}
      >
        <div class="lp-desk-glow" aria-hidden="true" />
        <AppWindow />
      </div>
      <p class="lp-desk-hint">试着拖动进度条，或者点一颗心</p>
    </section>
  )
}

function Lyric() {
  return (
    <section id="lyric" class="lp-lyric" data-reveal>
      <div class="lp-lyric-copy">
        <p class="eyebrow">律动</p>
        <h2 class="display">
          <span>每个字，</span>
          <span>都跟着唱。</span>
        </h2>
        <p class="lede">
          逐字高亮与翻译同屏，长句也不会跳字。切到「律动」视图，每个字随演唱浮起、发光、落定，像声音在纸上被写了一遍。
        </p>
        <ul class="lp-notes">
          <li>逐字时间轴，跟唱不抢拍</li>
          <li>原文与翻译同屏对照</li>
          <li>点一句歌词，直接跳过去</li>
        </ul>
      </div>
      <LyricStage />
    </section>
  )
}

type Feature = { icon: IconName; title: string; body: string; span?: number; extra?: () => JSX.Element }

const features: Feature[] = [
  {
    icon: 'wave',
    title: '边下边播，直到 Hi-Res',
    body: '不必等整首下载完。标准、较高、极高、无损（FLAC）与 Hi-Res，按耳朵和网速选。',
    span: 4,
    extra: () => <Quality />,
  },
  {
    icon: 'notch',
    title: '刘海播放器',
    body: '在屏幕顶部的刘海处显示迷你播放器，指针移过去就展开。',
    span: 2,
  },
  {
    icon: 'lyrics',
    title: '逐字歌词与「律动」',
    body: '每个字随演唱浮起、发光、落定。还能读这首歌的评论。',
    span: 3,
  },
  {
    icon: 'volume',
    title: '温柔的音量',
    body: '音量均衡，暂停与继续时淡出淡入。',
    span: 3,
    extra: () => <Fade />,
  },
  {
    icon: 'disk',
    title: '听过的，存在本机',
    body: '听过的歌缓存在本机，再次播放无需等待。',
  },
  {
    icon: 'history',
    title: '从上次停下的地方',
    body: '退出后再打开，接着上次的队列与进度。',
  },
  {
    icon: 'radio',
    title: '私人 FM 与心动模式',
    body: '推荐、熟悉、探索三种口味；或者从你喜欢的歌出发。',
  },
  {
    icon: 'calendar',
    title: '每日推荐',
    body: '每天 6:00 更新。还有推荐歌单、排行榜与搜索。',
  },
  {
    icon: 'qr',
    title: '扫码登录',
    body: '用网易云音乐 App 扫一扫，登录态存在系统凭据里。',
  },
  {
    icon: 'sun',
    title: '浅色、深色、跟随系统',
    body: '和这个网站一样——现在就试试。',
    extra: () => <ThemeSwitch labels class="lp-theme" />,
  },
]

function Bento() {
  return (
    <section id="features" class="lp-bento" data-reveal>
      <div class="lp-bento-head">
        <p class="eyebrow">功能</p>
        <h2 class="display">
          <span>该有的，</span>
          <span>都在。</span>
        </h2>
        <p class="lede">从发现到播放，每一处都为桌面重新做过。喜欢、队列、随机与循环，一样不少。</p>
      </div>
      <div class="lp-grid">
        <For each={features}>
          {(f) => <Cell feature={f} />}
        </For>
      </div>
    </section>
  )
}

// Each card carries the pointer position as --mx/--my for its own light.
function Cell(props: { feature: Feature }) {
  let card: HTMLElement | undefined
  const move = (e: PointerEvent) => {
    if (!card) return
    const box = card.getBoundingClientRect()
    card.style.setProperty('--mx', `${e.clientX - box.left}px`)
    card.style.setProperty('--my', `${e.clientY - box.top}px`)
  }
  return (
    <article
      class="lp-cell"
      classList={{
        'lp-cell-mid': props.feature.span === 3,
        'lp-cell-wide': props.feature.span === 4,
      }}
      ref={card}
      onPointerMove={move}
    >
      <div class="lp-cell-icon">
        <Icon name={props.feature.icon} size={20} />
      </div>
      <h3>{props.feature.title}</h3>
      <p>{props.feature.body}</p>
      {props.feature.extra?.()}
    </article>
  )
}

const qualities = [
  { name: '标准', spec: '128 kbps · MP3' },
  { name: '较高', spec: '192 kbps · MP3' },
  { name: '极高', spec: '320 kbps · MP3' },
  { name: '无损', spec: '最高 48 kHz / 16 bit · FLAC' },
  { name: 'Hi-Res', spec: '最高 192 kHz / 24 bit · FLAC' },
]

// The app's own quality picker, working.
function Quality() {
  const [q, setQ] = createSignal(3)
  return (
    <div class="quality">
      <div class="segmented" role="radiogroup" aria-label="音质">
        <For each={qualities}>
          {(item, i) => (
            <button
              type="button"
              role="radio"
              aria-checked={q() === i()}
              classList={{ on: q() === i() }}
              onClick={() => setQ(i())}
            >
              <span>{item.name}</span>
            </button>
          )}
        </For>
      </div>
      <p class="quality-spec">{qualities[q()].spec}</p>
    </div>
  )
}

// A wave fading out at a pause and back in, drawn.
function Fade() {
  const bars = Array.from({ length: 44 }, (_, i) => {
    const env = i < 16 ? 1 : i < 22 ? 1 - (i - 16) / 6 : i < 26 ? 0 : Math.min((i - 26) / 6, 1)
    return Math.max(0.06, env * (0.45 + 0.55 * Math.abs(Math.sin(i * 1.7) * Math.cos(i * 0.6))))
  })
  return (
    <div class="fade" aria-hidden="true">
      <For each={bars}>{(h) => <i style={{ height: `${h * 100}%` }} />}</For>
      <span class="fade-mark">暂停</span>
    </div>
  )
}

function Closing() {
  return (
    <section id="get" class="lp-closing" data-reveal>
      <div class="lp-closing-glow" aria-hidden="true" />
      <p class="eyebrow">准备好了</p>
      <h2 class="lp-closing-title">
        现在，
        <br />
        把它放进 Dock。
      </h2>
      <div class="lp-actions">
        <Link class="btn btn-primary lp-btn" to="/download">
          <Icon name="download" size={17} />
          下载 {site.name}
        </Link>
      </div>
      <p class="lp-fine">macOS · Windows · Linux · {version}</p>
    </section>
  )
}
