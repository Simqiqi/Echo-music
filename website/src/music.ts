import { createSignal } from 'solid-js'

// The site's background music: a short clip that plays under the page. It
// steps aside whenever something else on the page plays — the demo bar, a
// video — and comes back when that stops. The visitor's choice is kept
// between visits.
const key = 'bgm'
const level = 0.3

const [enabled, setEnabled] = createSignal(
  typeof localStorage === 'undefined' || localStorage.getItem(key) !== 'off',
)
// Whether sound is actually coming out. A browser that has never seen this
// visitor may only allow the clip to start with the mute on, or not at all.
const [audible, setAudible] = createSignal(false)
// The clip is ready but the browser is holding its tongue; one touch frees it.
const [blocked, setBlocked] = createSignal(false)
export { enabled, audible, blocked }

let audio: HTMLAudioElement | undefined
// Another player on the page has the floor.
let stepped = false
let armed = false
// The clip has played to its end; it waits to be asked for again.
let finished = false

// The element has to live in the document, or media events never reach the
// listeners below — and those listeners are what let the clip step aside.
function element() {
  if (audio) return audio
  const el = new Audio('/bgm.mp3')
  // Plays once, then stays quiet: a loop would wear the clip out.
  el.loop = false
  el.volume = level
  el.preload = 'auto'
  el.addEventListener('playing', () => setAudible(!el.muted))
  el.addEventListener('volumechange', () => {
    setAudible(!el.muted && !el.paused)
    if (!el.muted) setBlocked(false)
  })
  el.addEventListener('pause', () => setAudible(false))
  el.addEventListener('ended', () => {
    finished = true
    setAudible(false)
  })
  audio = el
  if (typeof document !== 'undefined' && document.body) document.body.appendChild(el)
  return el
}

// Fetching begins with the page's own code, not with the first play, so by
// the time the visitor is ready the clip has nothing left to wait for.
if (typeof window !== 'undefined') element()

// Try to make noise straight away. Most browsers refuse that until the
// visitor has interacted with the site; the fallback is to start the clip
// anyway, silently, and unmute the moment they touch anything.
function start() {
  if (!enabled() || stepped || finished || document.hidden) return
  const el = element()
  if (el.muted) setBlocked(true)
  el.play()
    .then(() => setBlocked(el.muted))
    .catch(() => {
      el.muted = true
      setBlocked(true)
      el.play().catch(() => arm())
    })
}

// Even muted playback can be refused (battery saver, strict privacy modes).
function arm() {
  if (armed) return
  armed = true
}

// The visitor did something: give the clip its voice — from the top, since
// nobody heard the part that ran while it was mute.
export function aloud() {
  if (!enabled() || stepped) return
  const el = element()
  finished = false
  el.muted = false
  try {
    el.currentTime = 0
  } catch {}
  el.volume = 0
  el.play()
    .then(() => {
      setBlocked(false)
      fadeIn()
    })
    .catch(() => arm())
}

function fadeIn() {
  const el = element()
  const began = performance.now()
  const step = (now: number) => {
    const k = Math.min(1, (now - began) / 1200)
    el.volume = level * k * k
    if (k < 1 && !el.paused) requestAnimationFrame(step)
  }
  requestAnimationFrame(step)
}

export function stop() {
  audio?.pause()
}

// One button, two jobs: give the clip its voice, or silence it. Asking again
// after it has run to its end starts it over.
export function tap() {
  const el = audio
  if (el && el.muted) return aloud()
  if (audible()) {
    setEnabled(false)
    setBlocked(false)
    try {
      localStorage.setItem(key, 'off')
    } catch {}
    return stop()
  }
  setEnabled(true)
  try {
    localStorage.setItem(key, 'on')
  } catch {}
  finished = false
  if (el) el.muted = false
  start()
}

// Something else starts playing: the background steps back until it stops.
export function hold() {
  stepped = true
  audio?.pause()
}

export function release() {
  stepped = false
  start()
}

// Media events don't bubble, but a capture listener on the document still
// hears them, whatever element they come from — so any player added to the
// page later gets this for free.
export function listen() {
  document.addEventListener(
    'play',
    (e) => {
      if (e.target !== audio) hold()
    },
    true,
  )
  document.addEventListener(
    'pause',
    (e) => {
      if (e.target !== audio) release()
    },
    true,
  )
  document.addEventListener(
    'ended',
    (e) => {
      if (e.target !== audio) release()
    },
    true,
  )
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) audio?.pause()
    else release()
  })
  // Any gesture anywhere counts; the browser says so, and this is also the
  // sound-on switch for a clip that had to start with the mute on.
  const wake = () => {
    if (audio?.muted) aloud()
    if (armed) {
      armed = false
      start()
    }
  }
  for (const type of ['pointerdown', 'keydown', 'touchstart'])
    document.addEventListener(type, wake, { passive: true })
  start()
}
