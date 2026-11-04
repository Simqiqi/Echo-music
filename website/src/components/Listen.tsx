import { Show, createSignal, onCleanup, onMount } from 'solid-js'
import { Transport } from './Icon'

// The song in the screenshot, made playable: the picture is the app as it
// was, and this bar hands the visitor the same track. It only appears once
// the file is really there, so a missing recording leaves no dead button.
const src = '/bu-jiang-jiu.mp3'
const song = { title: '不将就', artist: '李荣浩' }

const minutes = (seconds: number) => {
  const s = Math.floor(seconds)
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

export function Listen() {
  const [ready, setReady] = createSignal(false)
  const [on, setOn] = createSignal(false)
  const [at, setAt] = createSignal(0)
  const [length, setLength] = createSignal(0)

  let audio: HTMLAudioElement | undefined
  let frame = 0

  onMount(() => {
    const el = new Audio(src)
    el.preload = 'metadata'
    el.addEventListener('loadedmetadata', () => {
      setLength(el.duration)
      setReady(true)
    })
    el.addEventListener('ended', () => setOn(false))
    el.addEventListener('pause', () => setOn(false))
    el.addEventListener('play', () => setOn(true))
    audio = el

    // The bar is drawn second by second, which timeupdate is too coarse for.
    const tick = () => {
      setAt(el.currentTime)
      frame = requestAnimationFrame(tick)
    }
    el.addEventListener('playing', tick)
    el.addEventListener('pause', () => cancelAnimationFrame(frame))
    el.addEventListener('ended', () => cancelAnimationFrame(frame))
    onCleanup(() => {
      cancelAnimationFrame(frame)
      el.pause()
      el.src = ''
    })
  })

  const toggle = () => {
    if (!audio) return
    if (audio.paused) void audio.play().catch(() => setReady(false))
    else audio.pause()
  }

  const jump = (e: MouseEvent) => {
    if (!audio || !length()) return
    const box = (e.currentTarget as HTMLElement).getBoundingClientRect()
    audio.currentTime = Math.max(0, Math.min(1, (e.clientX - box.left) / box.width)) * length()
    setAt(audio.currentTime)
  }

  return (
    <Show when={ready()}>
      <div class="lp-listen">
        <button type="button" class="lp-listen-btn" onClick={toggle} aria-label={on() ? '暂停' : '播放'}>
          <Transport name={on() ? 'pause' : 'play'} size={15} />
        </button>
        <div class="lp-listen-meta">
          <b>{song.title}</b>
          <span>{song.artist}</span>
        </div>
        <div
          class="lp-listen-track"
          role="slider"
          aria-label="播放进度"
          aria-valuemin={0}
          aria-valuemax={Math.round(length())}
          aria-valuenow={Math.round(at())}
          tabindex="0"
          onClick={jump}
        >
          <i style={{ width: `${length() ? (at() / length()) * 100 : 0}%` }} />
        </div>
        <span class="lp-listen-time">
          {minutes(at())} / {minutes(length())}
        </span>
      </div>
    </Show>
  )
}
