import { onCleanup, onMount } from 'solid-js'
import { tone } from '../theme'

// The hero's field: a few hundred filaments of colour drifting through a
// slow current, laid down a hair at a time so they smear like ink in water.
// The pointer opens a vortex where it goes, and the whole field swells and
// settles on a beat of its own. Nothing here reads the audio — the beat is
// composed, so the picture never depends on whether the clip was allowed to
// play.
export function Ink() {
  let canvas!: HTMLCanvasElement

  onMount(() => {
    const ctx = canvas.getContext('2d', { alpha: true })
    if (!ctx) return

    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    let width = 0
    let height = 0
    let raf = 0
    let visible = true
    let started = false

    type Filament = { x: number; y: number; px: number; py: number; v: number; hue: number }
    let filaments: Filament[] = []

    let pointerX = 0.5
    let pointerY = 0.45
    let holdX = pointerX
    let holdY = pointerY

    let accent = 'oklch(0.66 0.2 22)'
    let accent2 = 'oklch(0.66 0.2 320)'

    const readColors = () => {
      const css = getComputedStyle(document.documentElement)
      const a = css.getPropertyValue('--accent').trim()
      if (a) accent = a
      // A second colour, a step round the wheel from the accent, so the
      // field has two temperatures rather than one flat wash.
      accent2 = a ? `color-mix(in oklab, ${a} 55%, oklch(0.6 0.18 300))` : accent2
    }
    readColors()
    const observer = new MutationObserver(readColors)
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    })

    const seed = () => {
      const count = width < 760 ? 300 : 720
      filaments = Array.from({ length: count }, () => {
        const x = Math.random() * width
        const y = Math.random() * height
        return { x, y, px: x, py: y, v: 0.5 + Math.random() * 0.9, hue: Math.random() }
      })
    }

    const resize = () => {
      const rect = canvas.getBoundingClientRect()
      const dpr = Math.min(window.devicePixelRatio || 1, 1.75)
      width = Math.max(1, rect.width)
      height = Math.max(1, rect.height)
      canvas.width = Math.round(width * dpr)
      canvas.height = Math.round(height * dpr)
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
      // A clean sheet, then the new seeds: resizing mid-flight would
      // otherwise leave the old ones stretched across the new size.
      ctx.clearRect(0, 0, width, height)
      seed()
    }
    resize()
    window.addEventListener('resize', resize)

    const onPointer = (e: PointerEvent) => {
      const rect = canvas.getBoundingClientRect()
      pointerX = (e.clientX - rect.left) / Math.max(rect.width, 1)
      pointerY = (e.clientY - rect.top) / Math.max(rect.height, 1)
    }
    window.addEventListener('pointermove', onPointer, { passive: true })

    // The current: waves short enough that neighbours drift apart, summed
    // with each filament's own bias, so the field never settles into one
    // direction and streak the whole hero flat.
    const angleAt = (x: number, y: number, t: number) => {
      const a = Math.sin(x * 0.0046 + t * 1.7) * 1.25
      const b = Math.cos(y * 0.0052 - t * 1.15) * 1.2
      const c = Math.sin((x + y) * 0.0021 + t * 0.7) * 0.9
      return a + b + c
    }

    const drawOnce = (now: number) => {
      const dark = tone() === 'dark'
      // The trails are what make it ink rather than lines: each frame takes
      // a little of the last one away instead of painting over it.
      ctx.globalCompositeOperation = 'destination-out'
      // The fade has to outrun the painting, or the field saturates to a
      // white smear within a few seconds — this ratio is the whole trick.
      ctx.fillStyle = `rgba(0,0,0,${dark ? 0.16 : 0.24})`
      ctx.fillRect(0, 0, width, height)

      const t = now * 0.00021
      // A beat made of two periods that never line up: the field breathes
      // without ever looking like it is looping.
      const pulse = 0.62 + 0.38 * Math.sin(now * 0.0016 + Math.sin(now * 0.00043) * 2.2)

      holdX += (pointerX - holdX) * 0.05
      holdY += (pointerY - holdY) * 0.05
      const vortexX = holdX * width
      const vortexY = holdY * height
      const reach = Math.min(width, height) * 0.42

      ctx.globalCompositeOperation = dark ? 'lighter' : 'source-over'
      ctx.lineCap = 'round'

      for (const f of filaments) {
        f.px = f.x
        f.py = f.y

        const dx = f.x - vortexX
        const dy = f.y - vortexY
        const dist = Math.hypot(dx, dy)

        let angle = angleAt(f.x, f.y, t) + (f.hue - 0.5) * 1.5
        if (dist < reach) {
          // Inside the pointer's reach the current turns around it: a
          // vortex near the middle, a nudge at its edge.
          const pull = (1 - dist / reach) ** 2
          angle += (Math.atan2(dy, dx) + Math.PI / 2) * pull * 2.1
          f.x += (-dx / (dist + 1)) * pull * 0.6
          f.y += (-dy / (dist + 1)) * pull * 0.6
        }

        const speed = f.v * (0.7 + pulse * 1.0)
        // A slow vertical sway of each filament's own, so the current can
        // never flatten into horizontal streaks.
        f.x += Math.cos(angle) * speed
        f.y += Math.sin(angle) * speed + Math.sin(t * 2.3 + f.hue * 6.283) * 0.55

        // Off the edge, come back on the other side: a current, not a
        // population that thins out. The previous point comes along, or the
        // respawn would draw one long line across the whole field.
        if (f.x < -20) {
          f.x = width + 20
          f.px = f.x
        }
        if (f.x > width + 20) {
          f.x = -20
          f.px = f.x
        }
        if (f.y < -20) {
          f.y = height + 20
          f.py = f.y
        }
        if (f.y > height + 20) {
          f.y = -20
          f.py = f.y
        }

        const heat = Math.sin((f.x * 0.7 + f.y) * 0.004 + t * 3) * 0.5 + 0.5
        ctx.strokeStyle = heat > 0.5 ? accent : accent2
        if (dark) {
          const alpha = 0.06 + 0.22 * heat * pulse
          ctx.globalAlpha = alpha * 0.5
          ctx.lineWidth = 3.2 + heat * 3
          ctx.beginPath()
          ctx.moveTo(f.px, f.py)
          ctx.lineTo(f.x, f.y)
          ctx.stroke()
          ctx.globalAlpha = alpha
          ctx.lineWidth = 1 + heat * 1.2
        } else {
          // Ink on paper: one thin pass, and quietly — the paper shows
          // every stroke it is given, so it gets fewer.
          ctx.globalAlpha = 0.05 + 0.13 * heat * pulse
          ctx.lineWidth = 1 + heat * 1.1
        }
        ctx.beginPath()
        ctx.moveTo(f.px, f.py)
        ctx.lineTo(f.x, f.y)
        ctx.stroke()
      }
      ctx.globalAlpha = 1
    }

    const loop = (now: number) => {
      raf = 0
      if (!visible || document.hidden) return
      drawOnce(now)
      raf = requestAnimationFrame(loop)
    }

    const gate = new IntersectionObserver(
      (entries) => {
        visible = entries.some((entry) => entry.isIntersecting)
        if (visible && !started && !reduce) {
          started = true
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
      } else if (visible && started && !raf && !reduce) {
        loop(performance.now())
      }
    }
    document.addEventListener('visibilitychange', onVisibility)

    // Less motion asked for: one still frame, laid down a few times so the
    // filaments have some length to them, and no loop at all.
    if (reduce) {
      ctx.clearRect(0, 0, width, height)
      for (let i = 0; i < 170; i++) drawOnce(performance.now() + i * 16)
    }

    onCleanup(() => {
      cancelAnimationFrame(raf)
      window.removeEventListener('resize', resize)
      window.removeEventListener('pointermove', onPointer)
      document.removeEventListener('visibilitychange', onVisibility)
      observer.disconnect()
      gate.disconnect()
    })
  })

  return <canvas class="v2-ink" ref={canvas} aria-hidden="true" />
}
