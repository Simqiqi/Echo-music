import { For } from 'solid-js'

// Three lines of the app's 「律动」 view. Every character rises and catches
// the light on its own beat, which is what the real player does with the
// singing voice's timestamps.
const lines = [
  { text: '夜色漫过窗台', note: 'the night spills over the sill' },
  { text: '我不必说话', note: "I don't have to say a word" },
  { text: '让旋律替我回答', note: 'let the melody answer for me' },
]

export function LyricStage() {
  // Running character count, so the wave travels across lines instead of
  // restarting on each one.
  let seen = 0
  const staged = lines.map((line) => {
    const item = { ...line, offset: seen }
    seen += line.text.length
    return item
  })

  return (
    <div class="lyric-demo">
      <div class="lyric-glow" aria-hidden="true" />
      <For each={staged}>
        {(line) => (
          <div class="lyric-line">
            <p class="lyric-cn">
              <For each={Array.from(line.text)}>
                {(ch, i) => (
                  <span class="lyric-ch" style={{ '--i': `${line.offset + i()}` }}>
                    {ch}
                  </span>
                )}
              </For>
            </p>
            <p class="lyric-note">{line.note}</p>
          </div>
        )}
      </For>
      <div class="lyric-rule" aria-hidden="true">
        <i />
      </div>
      <p class="lyric-caption">示例歌词 · 真机上跟着人声逐字浮起</p>
    </div>
  )
}
