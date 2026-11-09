import { Link, createFileRoute } from '@tanstack/solid-router'
import { For, createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import { Layout } from '../components/Layout'
import { Ink } from '../components/Ink'
import { Ribbon } from '../components/Ribbon'
import { Listen } from '../components/Listen'
import { Icon, type IconName } from '../components/Icon'
import { ThemeSwitch, flipTone } from '../theme'
import { aloud, audible, blocked, tap } from '../music'
import { mygo, release, site } from '../site'

export const Route = createFileRoute('/')({
  component: Home,
})

const version = release.tag_name.replace(/^v/, '')

const installSize = (() => {
  const dmg = release.assets.find((a) => /darwin-arm64\.dmg$/.test(a.name))
  return dmg ? Math.round(dmg.size / 1048576) : 21
})()

function Home() {
  useReveal()
  return (
    <Layout>
      <Hero />
      <Marquee />
      <Stats />
      <Showcase />
      <Ribbon />
      <Bento />
      <Closing />
    </Layout>
  )
}

// Sections arrive as they are reached: a short lift and a fade, and nothing
// at all for a visitor who asked the system for less motion.
function useReveal() {
  onMount(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    const targets = Array.from(document.querySelectorAll<HTMLElement>('[data-reveal]'))
    if (!targets.length) return
    for (const el of targets) el.classList.add('v2-waiting')
    const watcher = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          entry.target.classList.add('v2-here')
          watcher.unobserve(entry.target)
        }
      },
      { rootMargin: '0px 0px -12% 0px', threshold: 0.08 },
    )
    for (const el of targets) watcher.observe(el)
    onCleanup(() => watcher.disconnect())
  })
}

function Chars(props: { text: string; from?: number }) {
  return (
    <>
      {Array.from(props.text).map((ch, i) => (
        <span class="v2-ch" style={{ '--i': `${(props.from ?? 0) + i}` }}>
          {ch}
        </span>
      ))}
    </>
  )
}

function SoundChip() {
  return (
    <button
      type="button"
      class="v2-sound"
      classList={{ on: audible() }}
      onClick={() => (blocked() ? aloud() : tap())}
      aria-pressed={audible()}
    >
      <span class="v2-sound-eq" aria-hidden="true">
        <i />
        <i />
        <i />
        <i />
      </span>
      <span>{audible() ? '背景音乐 · 开着' : blocked() ? '轻点，让它响起来' : '背景音乐 · 已关'}</span>
    </button>
  )
}

function Hero() {
  return (
    <section class="v2-hero">
      <Ink />
      <div class="v2-hero-fade" aria-hidden="true" />
      <div class="v2-hero-inner">
        <div class="v2-hero-top">
          <p class="v2-eyebrow">
            <i class="v2-dot" aria-hidden="true" />
            原生 · 轻量 · 没有 WebView
          </p>
          <SoundChip />
        </div>
        <h1 class="v2-title">
          <span class="v2-title-line">
            <Chars text="声音，" />
          </span>
          <span class="v2-title-line v2-title-accent">
            <Chars text="不该住在浏览器里" from={3} />
          </span>
        </h1>
        <div class="v2-hero-foot">
          <p class="v2-lede">
            用 Go 与 <a href={mygo}>MyGo</a>{' '}
            写成的第三方网易云音乐客户端。原生界面，冷启动不到一秒；Hi-Res 边下边播，逐字歌词跟着人声浮起，闲下来就安静地待在
            Dock 里。
          </p>
          <div class="v2-actions">
            <Link class="btn btn-primary v2-btn" to="/download">
              <Icon name="download" size={17} />
              下载
            </Link>
            <button
              class="btn btn-ghost v2-btn"
              type="button"
              onClick={(e) => flipTone(e)}
            >
              <Icon name="sparkle" size={17} />
              换个主题
            </button>
          </div>
        </div>
        <p class="v2-fine">版本 {version} · 免费 · 与网易云音乐官方无关</p>
      </div>
      <div class="v2-cue" aria-hidden="true">
        <span>把光标放进光里，再往下</span>
        <i />
      </div>
    </section>
  )
}

const marquee = [
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
  '墨迹歌词',
  '万花筒',
]

function Marquee() {
  return (
    <section class="v2-marquee" aria-hidden="true">
      <div class="v2-marquee-track">
        <For each={[...marquee, ...marquee]}>
          {(word) => (
            <span class="v2-marquee-item">
              {word}
              <i />
            </span>
          )}
        </For>
      </div>
    </section>
  )
}

// Numbers that count themselves up the first time they are seen.
function Count(props: { to: number; decimals?: number; suffix?: string }) {
  const [n, setN] = createSignal(0)
  let node: HTMLSpanElement | undefined

  onMount(() => {
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    if (reduce) {
      setN(props.to)
      return
    }
    const watcher = new IntersectionObserver(
      (entries) => {
        if (!entries.some((e) => e.isIntersecting)) return
        watcher.disconnect()
        const began = performance.now()
        const step = (now: number) => {
          const k = Math.min(1, (now - began) / 1100)
          const eased = 1 - (1 - k) ** 3
          setN(props.to * eased)
          if (k < 1) requestAnimationFrame(step)
        }
        requestAnimationFrame(step)
      },
      { threshold: 0.4 },
    )
    if (node) watcher.observe(node)
    onCleanup(() => watcher.disconnect())
  })

  return (
    <span ref={node}>
      {n().toFixed(props.decimals ?? 0)}
      {props.suffix}
    </span>
  )
}

function Stats() {
  return (
    <section class="v2-stats" data-reveal>
      <div class="v2-stat">
        <b>
          <Count to={0.9} decimals={1} suffix="s" />
        </b>
        <span>冷启动</span>
        <p>从点击到出声，多数机器在半秒到一秒之间。</p>
      </div>
      <div class="v2-stat">
        <b>
          <Count to={192} suffix="kHz" />
        </b>
        <span>最高取样率</span>
        <p>Hi-Res 24 bit / 192 kHz，边下边播，不必等整首。</p>
      </div>
      <div class="v2-stat">
        <b>
          <Count to={installSize} suffix="MB" />
        </b>
        <span>安装包</span>
        <p>没有浏览器内核要一起带上，所以只有这么大。</p>
      </div>
    </section>
  )
}

// The app window, tilted a few degrees toward the pointer.
function Showcase() {
  let frame: HTMLDivElement | undefined

  const move = (e: PointerEvent) => {
    if (!frame) return
    const box = frame.getBoundingClientRect()
    const x = (e.clientX - box.left) / box.width - 0.5
    const y = (e.clientY - box.top) / box.height - 0.5
    frame.style.setProperty('--ry', `${x * 5}deg`)
    frame.style.setProperty('--rx', `${-y * 3.6}deg`)
    frame.style.setProperty('--mx', `${(x + 0.5) * 100}%`)
    frame.style.setProperty('--my', `${(y + 0.5) * 100}%`)
  }
  const rest = () => {
    if (!frame) return
    frame.style.setProperty('--ry', '0deg')
    frame.style.setProperty('--rx', '0deg')
  }

  return (
    <section id="demo" class="v2-show" data-reveal>
      <div class="v2-show-head">
        <p class="v2-eyebrow">真机截图</p>
        <h2 class="v2-h2">
          下面这一屏，
          <em>就是它本身。</em>
        </h2>
        <p class="v2-lede">
          截自正在运行的那一份：登录、推荐、播放器都在，连正在播的那首歌都是真的——下面这条，就是那首歌。
        </p>
      </div>
      <div
        class="v2-shot-frame"
        ref={frame}
        onPointerMove={move}
        onPointerLeave={rest}
        style={{ '--rx': '0deg', '--ry': '0deg' }}
      >
        <div class="v2-shot-glow" aria-hidden="true" />
        <img
          class="v2-shot"
          src="/app.png"
          width={1680}
          height={1160}
          alt="Echo云音乐的真机截图：为你推荐首页，正在播放李荣浩的《不将就》"
          loading="lazy"
          decoding="async"
        />
      </div>
      <Listen />
      <div class="v2-bgm">
        <p class="v2-bgm-title">关于这段音乐</p>
        <ul>
          <li>
            <b>一进站就会响。</b>
            声音先起，页面才被听见。浏览器不让自动发声时，它先静音开始，等你轻点页面任意处再补上。
          </li>
          <li>
            <b>它会自己让位。</b>
            页面里任何播放器一响——上面那首，或者一段视频——背景就退到后面；唱完，它再回来。
          </li>
          <li>
            <b>它记得你的选择。</b>
            关掉之后，下一次打开不会再自己响；想听，点顶栏的喇叭，或左上角那个开关。
          </li>
        </ul>
      </div>
    </section>
  )
}

type Feature = {
  icon: IconName
  title: string
  body: string
  span?: number
  extra?: () => JSX.Element
}

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
    body: '每个字随演唱浮起、发光、落定。还有墨迹、霓虹与万花筒几种写法。',
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
    extra: () => <ThemeSwitch labels class="v2-theme" />,
  },
]

function Bento() {
  return (
    <section id="features" class="v2-bento" data-reveal>
      <div class="v2-bento-head">
        <p class="v2-eyebrow">功能</p>
        <h2 class="v2-h2">
          该有的，
          <em>都在。</em>
        </h2>
        <p class="v2-lede">
          从发现到播放，每一处都为桌面重新做过。喜欢、队列、随机与循环，一样不少。
        </p>
      </div>
      <div class="v2-grid">
        <For each={features}>{(f, i) => <Cell feature={f} index={i()} />}</For>
      </div>
    </section>
  )
}

function Cell(props: { feature: Feature; index: number }) {
  let card: HTMLElement | undefined
  const move = (e: PointerEvent) => {
    if (!card) return
    const box = card.getBoundingClientRect()
    card.style.setProperty('--mx', `${e.clientX - box.left}px`)
    card.style.setProperty('--my', `${e.clientY - box.top}px`)
  }
  return (
    <article
      class="v2-cell"
      classList={{
        'v2-cell-mid': props.feature.span === 3,
        'v2-cell-wide': props.feature.span === 4,
      }}
      ref={card}
      onPointerMove={move}
    >
      <span class="v2-cell-n" aria-hidden="true">
        {String(props.index + 1).padStart(2, '0')}
      </span>
      <div class="v2-cell-icon">
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
    <section id="get" class="v2-close" data-reveal>
      <div class="v2-close-light" aria-hidden="true" />
      <p class="v2-eyebrow">准备好了</p>
      <h2 class="v2-close-title">
        现在，
        <br />
        把它放进 Dock。
      </h2>
      <div class="v2-actions">
        <Link class="btn btn-primary v2-btn" to="/download">
          <Icon name="download" size={17} />
          下载 {site.name}
        </Link>
        <button class="btn btn-ghost v2-btn" type="button" onClick={(e) => flipTone(e)}>
          <Icon name="sparkle" size={17} />
          换个主题看看
        </button>
      </div>
      <p class="v2-fine">
        macOS · Windows · Linux · {version} · 安装包 {installSize}MB
      </p>
    </section>
  )
}
