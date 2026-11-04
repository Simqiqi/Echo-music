import { onCleanup, onMount } from 'solid-js'

const BARS = 84
const DUST = 46

type Mote = { x: number; y: number; r: number; speed: number; phase: number }

// The hero's backdrop: a ribbon of bars that breathes on its own, dust that
// drifts with the pointer, and a soft lamp that follows it. Everything is
// drawn on one canvas so it stays cheap; the loop sleeps when the tab or the
// hero is out of sight.
export function Stage() {
  let canvas!: HTMLCanvasElement

  onMount(() => {
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    let width = 0
    let height = 0
    let dpr = 1
    let raf = 0
    let visible = true
    let shown = false

    // Pointer position, eased toward so the lamp trails a little.
    let targetX = 0.5
    let targetY = 0.42
    let lampX = targetX
    let lampY = targetY

    const levels = new Float32Array(BARS)
    const motes: Mote[] = Array.from({ length: DUST }, () => ({
      x: Math.random(),
      y: Math.random(),
      r: 0.6 + Math.random() * 1.8,
      speed: 0.006 + Math.random() * 0.016,
      phase: Math.random() * Math.PI * 2,
    }))

    let accent = 'oklch(0.6 0.21 24)'
    let accentSoft = 'oklch(0.6 0.21 24 / 0.5)'

    const readColors = () => {
      const css = getComputedStyle(document.documentElement)
      const a = css.getPropertyValue('--accent').trim()
      if (a) accent = a
      accentSoft = a ? `color-mix(in oklab, ${a} 45%, transparent)` : accentSoft
    }
    readColors()

    // data-theme flips on <html> when the visitor picks light or dark.
    const observer = new MutationObserver(readColors)
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })

    const resize = () => {
      const rect = canvas.getBoundingClientRect()
      dpr = Math.min(window.devicePixelRatio || 1, 2)
      width = rect.width
      height = rect.height
      canvas.width = Math.max(1, Math.round(width * dpr))
      canvas.height = Math.max(1, Math.round(height * dpr))
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    }
    resize()

    const onResize = () => resize()
    window.addEventListener('resize', onResize)

    const onPointer = (e: PointerEvent) => {
      const rect = canvas.getBoundingClientRect()
      targetX = (e.clientX - rect.left) / Math.max(rect.width, 1)
      targetY = (e.clientY - rect.top) / Math.max(rect.height, 1)
    }
    window.addEventListener('pointermove', onPointer, { passive: true })

    // Nothing to animate off screen: the observer keeps the loop honest.
    const gate = new IntersectionObserver(
      (entries) => {
        visible = entries.some((entry) => entry.isIntersecting)
        if (visible && !shown) {
          shown = true
          loop(performance.now())
        }
      },
      { threshold: 0 },
    )
    gate.observe(canvas)

    const onVisibility = () => {
      if (document.hidden) {
        cancelAnimationFrame(raf)
        raf = 0
      } else if (visible && !raf) {
        loop(performance.now())
      }
    }
    document.addEventListener('visibilitychange', onVisibility)

    function loop(now: number) {
      raf = 0
      if (!visible || document.hidden) return
      draw(now)
      if (!reduce) raf = requestAnimationFrame(loop)
      else raf = window.setTimeout(() => loop(performance.now()), 400) as unknown as number
    }

    function draw(now: number) {
      if (!ctx) return
      ctx.clearRect(0, 0, width, height)

      lampX += (targetX - lampX) * 0.06
      lampY += (targetY - lampY) * 0.06

      // Lamp: a wide, weak pool of the accent colour under the pointer.
      const lamp = ctx.createRadialGradient(
        lampX * width,
        lampY * height,
        0,
        lampX * width,
        lampY * height,
        Math.max(width, height) * 0.62,
      )
      lamp.addColorStop(0, accentSoft)
      lamp.addColorStop(0.45, 'transparent')
      ctx.globalAlpha = 0.16
      ctx.fillStyle = lamp
      ctx.fillRect(0, 0, width, height)
      ctx.globalAlpha = 1

      // Dust, drifting up and leaning away from the pointer.
      for (const mote of motes) {
        mote.y -= mote.speed * 0.004
        if (mote.y < -0.05) {
          mote.y = 1.05
          mote.x = Math.random()
        }
        const dx = mote.x - lampX
        const dy = mote.y - lampY
        const push = Math.max(0, 0.28 - Math.hypot(dx, dy)) * 0.02
        const x = (mote.x + dx * push) * width
        const y = (mote.y + Math.sin(now * 0.0004 + mote.phase) * 0.004) * height
        ctx.globalAlpha = 0.5 - Math.abs(dy) * 0.3
        ctx.fillStyle = accent
        ctx.beginPath()
        ctx.arc(x, y, mote.r, 0, Math.PI * 2)
        ctx.fill()
      }
      ctx.globalAlpha = 1

      // The ribbon breathes on its own: two slow waves of different lengths,
      // so the bars never fall into a visible loop.
      for (let i = 0; i < BARS; i++) {
        const idle =
          0.05 + 0.035 * Math.sin(now * 0.0011 + i * 0.42) + 0.018 * Math.sin(now * 0.0007 - i * 0.9)
        levels[i] += (Math.max(0.02, idle) - levels[i]) * 0.06
      }

      // Bars: anchored at the hero's floor, mirrored about the centre.
      const base = height * 0.94
      const span = width * 0.92
      const left = (width - span) / 2
      const gap = span / BARS
      const barWidth = Math.max(1.5, gap * 0.46)
      const ceiling = height * 0.3

      const fill = ctx.createLinearGradient(0, base - ceiling, 0, base)
      fill.addColorStop(0, 'transparent')
      fill.addColorStop(0.55, accentSoft)
      fill.addColorStop(1, accent)

      ctx.fillStyle = fill
      ctx.globalAlpha = 0.75
      ctx.beginPath()
      for (let i = 0; i < BARS; i++) {
        // Taper toward the edges so the ribbon fades instead of stopping.
        const edge = Math.sin((i / (BARS - 1)) * Math.PI) ** 0.7
        const h = Math.max(1.5, levels[i] * ceiling * edge)
        const x = left + i * gap + (gap - barWidth) / 2
        ctx.roundRect(x, base - h, barWidth, h, barWidth / 2)
      }
      ctx.fill()
      ctx.globalAlpha = 1

      // A hairline of light where the ribbon meets the floor.
      const line = ctx.createLinearGradient(left, 0, left + span, 0)
      line.addColorStop(0, 'transparent')
      line.addColorStop(0.5, accent)
      line.addColorStop(1, 'transparent')
      ctx.globalAlpha = 0.35
      ctx.fillStyle = line
      ctx.fillRect(left, base, span, 1)
      ctx.globalAlpha = 1
    }

    onCleanup(() => {
      cancelAnimationFrame(raf)
      window.clearTimeout(raf)
      window.removeEventListener('resize', onResize)
      window.removeEventListener('pointermove', onPointer)
      document.removeEventListener('visibilitychange', onVisibility)
      observer.disconnect()
      gate.disconnect()
    })
  })

  return <canvas class="lp-stage" ref={canvas} aria-hidden="true" />
}
