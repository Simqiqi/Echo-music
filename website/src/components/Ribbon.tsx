import { For, onCleanup, onMount } from 'solid-js'

// The ribbon: three lines of the app's own words, laid end to end, that the
// page pulls sideways as the visitor scrolls. The section is taller than the
// screen and its contents stick, so a scroll that feels small moves the type
// a long way; the ends are masked out instead of cut.
const rows = [
  ['逐字歌词', '跟着人声浮起', '每个字都落在拍子上', '原文与翻译同屏', '点一句就跳过去'],
  ['边下边播', 'Hi-Res 无损', '刘海播放器', '私人 FM', '心动模式', '每日推荐', '离线缓存'],
  ['冷启动不到一秒', '没有 WebView', '原生界面', '浅色与深色', '扫码登录'],
]

export function Ribbon() {
  let wrap!: HTMLDivElement
  const tracks: HTMLDivElement[] = []

  onMount(() => {
    let frame = 0
    const measure = () => {
      frame = 0
      const box = wrap.getBoundingClientRect()
      const span = Math.max(1, box.height - window.innerHeight)
      const p = Math.min(1, Math.max(0, -box.top / span))
      wrap.style.setProperty('--p', p.toFixed(4))
      // How near the middle of the section the scroll is: what the big word
      // behind the rows brightens on.
      wrap.style.setProperty('--near', (1 - Math.abs(p - 0.5) * 2).toFixed(4))
      for (const track of tracks) {
        const travel = Math.max(0, track.scrollWidth - window.innerWidth + 96)
        const back = track.dataset.back === '1'
        track.style.transform = `translate3d(${(back ? travel * (p - 1) : -travel * p).toFixed(1)}px,0,0)`
      }
    }
    const onScroll = () => {
      if (frame) return
      frame = requestAnimationFrame(measure)
    }
    measure()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', measure)
    onCleanup(() => {
      cancelAnimationFrame(frame)
      window.removeEventListener('scroll', onScroll)
      window.removeEventListener('resize', measure)
    })
  })

  return (
    <section class="v2-ribbon" ref={wrap} aria-label="它记得的那些词">
      <div class="v2-ribbon-sticky">
        <div class="v2-ribbon-halo" aria-hidden="true" />
        <p class="v2-ribbon-kicker">往下滚，字会跟着走</p>
        <div class="v2-ribbon-rows">
          <For each={rows}>
            {(row, i) => (
              <div class="v2-ribbon-clip">
                <div
                  class="v2-ribbon-row"
                  data-back={i() === 1 ? '1' : '0'}
                  ref={(el) => (tracks[i()] = el)}
                >
                  <For each={[...row, ...row, ...row]}>
                    {(word) => (
                      <span class="v2-ribbon-word">
                        {word}
                        <i aria-hidden="true" />
                      </span>
                    )}
                  </For>
                </div>
              </div>
            )}
          </For>
        </div>
        <div class="v2-ribbon-center" aria-hidden="true">
          <span>律</span>
          <span>动</span>
        </div>
      </div>
    </section>
  )
}
