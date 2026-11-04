import { createSignal } from 'solid-js'

// The site's background music: a short clip that loops quietly under the
// page. It steps aside whenever something else on the page plays — a demo
// player, a video — and comes back when that stops. The visitor's choice is
// kept between visits.
const key = 'bgm'
const level = 0.3

const [enabled, setEnabled] = createSignal(
  typeof localStorage === 'undefined' || localStorage.getItem(key) !== 'off',
)
// Whether sound is actually coming out, which is not the same thing: a
// browser may refuse the first play until the visitor touches the page.
const [audible, setAudible] = createSignal(false)
export { enabled, audible }

let audio: HTMLAudioElement | undefined
// Another player on the page has the floor.
let stepped = false
let armed = false
// The clip has played to its end; it waits to be asked for again.
let finished = false

function element() {
  if (audio) return audio
  const el = new Audio('/bgm.mp3')
  // Plays once, then stays quiet: a loop would wear the clip out.
  el.loop = false
  el.volume = level
  el.preload = 'auto'
  el.addEventListener('playing', () => setAudible(true))
  el.addEventListener('pause', () => setAudible(false))
  el.addEventListener('ended', () => {
    finished = true
    setAudible(false)
  })
  audio = el
  return el
}

// Fetching begins with the page's own code, not with the first play, so by
// the time the visitor is ready the clip has nothing left to wait for.
if (typeof window !== 'undefined') element()

function start() {
  if (!enabled() || stepped || finished || document.hidden) return
  element()
    .play()
    .catch(() => arm())
}

// Browsers hold the first play until the visitor does something; the first
// click or key is enough.
function arm() {
  if (armed) return
  armed = true
  const go = () => {
    for (const type of ['pointerdown', 'keydown', 'touchstart'])
      document.removeEventListener(type, go)
    start()
  }
  for (const type of ['pointerdown', 'keydown', 'touchstart'])
    document.addEventListener(type, go)
}

export function stop() {
  audio?.pause()
}

// One button, two jobs: silence what is playing, or start the clip again —
// from the top, even if it has already run to its end.
export function tap() {
  if (audible()) {
    setEnabled(false)
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
  start()
}
