import { createSignal } from 'solid-js'

// The song the window in the hero plays: the top of the hot chart, so the
// demo sounds like what people are listening to right now.
export const track = {
  title: 'Hong Kong',
  artist: 'The Weeknd, 王菲',
  src: '/hong-kong.mp3',
  duration: 217,
}

const [playing, setPlaying] = createSignal(false)
const [position, setPosition] = createSignal(62)
const [liked, setLiked] = createSignal(false)
export { playing, position, liked, setLiked }

// Made on the first play, so the page doesn't download the song until then.
let audio: HTMLAudioElement | undefined
let frame = 0

function element() {
  if (audio) return audio
  audio = new Audio(track.src)
  audio.loop = true
  audio.currentTime = position()
  // The browser can pause it on its own, from the media keys or another tab.
  audio.addEventListener('pause', () => {
    setPlaying(false)
    cancelAnimationFrame(frame)
  })
  return audio
}

// timeupdate only fires a few times a second, too coarse for the progress bar.
function tick() {
  if (audio) setPosition(audio.currentTime)
  frame = requestAnimationFrame(tick)
}

// The hero's spectrum reads the very song the window plays. Building a
// MediaElementSource takes the element off the default output, so the graph
// has to reach the speakers again on its own.
let context: AudioContext | undefined
let spectrum: AnalyserNode | undefined

export function analyser() {
  if (spectrum) return spectrum
  if (typeof AudioContext === 'undefined') return undefined
  try {
    context = new AudioContext()
    const node = context.createAnalyser()
    node.fftSize = 2048
    node.smoothingTimeConstant = 0.82
    context.createMediaElementSource(element()).connect(node)
    node.connect(context.destination)
    spectrum = node
  } catch {
    // No Web Audio: the page still plays, just without the bars.
  }
  return spectrum
}

export function play() {
  if (playing()) return
  setPlaying(true)
  analyser()
  // Browsers start the graph asleep until a gesture wakes it.
  if (context?.state === 'suspended') void context.resume()
  element()
    .play()
    .then(() => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(tick)
    })
    .catch(() => setPlaying(false))
}

export function pause() {
  audio?.pause()
}

export function toggle() {
  playing() ? pause() : play()
}

export function seek(fraction: number) {
  const p = Math.max(0, Math.min(1, fraction)) * track.duration
  setPosition(p)
  if (audio) audio.currentTime = p
}

export function clock(seconds: number) {
  const s = Math.floor(seconds)
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}
