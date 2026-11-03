// Package dj is the voice between songs: it writes what a radio host
// would say about a track, why the personal FM picked it, where it comes
// from, and how it sounds, and speaks it in a warm Mandarin voice as the
// song begins.
//
// The voice is the speech service's natural one when the network has it,
// and the system's own synthesizer when it does not; what is said is
// kept on disk, so a line is made once.
package dj

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/egoist/mygo/audio"
)

// ErrUnavailable reports that this system has no voice to speak with.
var ErrUnavailable = errors.New("dj: no voice on this system")

// ErrNothing reports that there is nothing to say.
var ErrNothing = errors.New("dj: nothing to say")

// Available reports whether a voice can be made on this system.
func Available() bool { return available() }

// Facts is what the host knows of a track, to speak about it.
type Facts struct {
	Title  string
	Artist string
	Album  string
	Year   int    // when the album came out, 0 if unknown
	Note   string // a sentence of the album's own description, if any
	Reason string // why the FM picked it, as NetEase said, if it did
	Mode   string // 推荐, 熟悉, 探索 or 心动: how it was picked
}

// Script returns what the host says about a track: at most max runes, so
// that it does not hold the next song back for long. It says what it
// knows, and no more: the recommendation, the album and its year, and a
// word of the style when the album's own description names one.
func Script(f Facts, r *rand.Rand, max int) string {
	title := strings.TrimSpace(f.Title)
	if title == "" {
		return ""
	}
	artist := strings.TrimSpace(f.Artist)
	var parts []string
	open := pick(r, "接下来这首", "下一首", "接着听", "给你放一首", "换一首")
	if artist != "" {
		parts = append(parts, open+"《"+title+"》，"+artist+"。")
	} else {
		parts = append(parts, open+"《"+title+"》。")
	}
	if line := reasonLine(f, r); line != "" {
		parts = append(parts, line)
	}
	if line := backgroundLine(f, r); line != "" {
		parts = append(parts, line)
	}
	if line := styleLine(f.Note, r); line != "" {
		parts = append(parts, line)
	}
	body := strings.Join(parts, "")
	// A closing word only when there is room for it.
	if coda := pickClose(r); coda != "" && runes(body)+runes(coda) <= max {
		body += coda
	}
	return capRunes(body, max)
}

// reasonLine says why the FM plays the track.
func reasonLine(f Facts, r *rand.Rand) string {
	if reason := cleanSentence(f.Reason); reason != "" && runes(reason) <= 26 {
		return "网易云说，" + strings.TrimSuffix(reason, "。") + "。"
	}
	switch f.Mode {
	case "熟悉":
		return pick(r, "它就在你熟悉的声线里。", "这首离你常听的风格不远。", "还是你熟悉的那种味道。")
	case "探索":
		return pick(r, "换个方向，也许会遇见新的喜欢。", "这首你大概还没听过。", "往没走过的地方，多听一点。")
	case "心动":
		return pick(r, "从你红心的歌出发，找到了这个相似的声音。", "这是从你喜欢的那首，延伸出来的。")
	default:
		return pick(r, "私人 FM 顺着你的口味挑的。", "它觉得这首合你的胃口。", "算法在你常听的附近，捞到了这首。")
	}
}

// backgroundLine says where the track comes from.
func backgroundLine(f Facts, r *rand.Rand) string {
	album := strings.TrimSpace(f.Album)
	switch {
	case album != "" && f.Year > 0:
		return fmt.Sprintf("收录在 %d 年的专辑《%s》里。", f.Year, album)
	case album != "":
		return "收录在专辑《" + album + "》里。"
	case f.Year > 0:
		return fmt.Sprintf("%d 年的作品。", f.Year)
	}
	return ""
}

// styles are the words a genre is named by, and what the host makes of
// them: only when the album's own description names one.
var styles = []struct {
	keys []string
	said []string
}{
	{[]string{"摇滚", "rock"}, []string{"骨子里是摇滚的劲儿。", "吉他一推，摇滚的底色就出来了。"}},
	{[]string{"民谣", "folk"}, []string{"有民谣讲故事的那种松弛。", "民谣的路子，词比旋律更近。"}},
	{[]string{"后摇", "post-rock"}, []string{"是后摇层层推进的路子。"}},
	{[]string{"电子", "electronic", "synth", "合成器"}, []string{"铺着一层电子的肌理。", "电子声场，空间感很足。"}},
	{[]string{"爵士", "jazz"}, []string{"带着爵士的即兴味道。", "爵士的松弛，藏在节拍的空隙里。"}},
	{[]string{"说唱", "嘻哈", "rap", "hip-hop"}, []string{"是说唱的律动。", "押韵的落点，都踩在鼓上。"}},
	{[]string{"r&b", "节奏布鲁斯"}, []string{"有节奏布鲁斯的转音和呼吸。"}},
	{[]string{"古典", "classical", "交响", "弦乐"}, []string{"有古典的编制和层次。"}},
	{[]string{"city pop", "城市流行"}, []string{"是城市流行的夜色。"}},
	{[]string{"氛围", "ambient"}, []string{"氛围铺得很开，适合放空。"}},
	{[]string{"放克", "funk"}, []string{"放克的贝斯线，会先抓住脚。"}},
}

// styleLine says a word of the style, when the description names one, or
// when the title itself says what kind of take it is.
func styleLine(note string, r *rand.Rand) string {
	low := strings.ToLower(note)
	for _, st := range styles {
		for _, k := range st.keys {
			if strings.Contains(low, k) {
				return pick(r, st.said...)
			}
		}
	}
	return ""
}

// pickClose is the host's last word, or none.
func pickClose(r *rand.Rand) string {
	if r != nil && r.IntN(3) == 0 {
		return ""
	}
	return pick(r, "听听看。", "希望你喜欢。", "这就放给你听。")
}

// cleanSentence returns the first sentence of s, without markup or the
// noise of a blurb, or "" when it says nothing worth hearing.
func cleanSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Descriptions come as HTML or with links: keep the words only.
	for _, cut := range []string{"<", "http", "展开", "版权", "暂无"} {
		if i := strings.Index(s, cut); i >= 0 {
			s = s[:i]
		}
	}
	s = strings.TrimSpace(strings.Trim(s, "。；;，,、 \n\t"))
	if s == "" {
		return ""
	}
	for i, c := range s {
		if strings.ContainsRune("。！？!?", c) {
			s = s[:i]
			break
		}
	}
	s = strings.TrimSpace(s)
	if n := runes(s); n < 6 || n > 34 {
		return ""
	}
	return s + "。"
}

func runes(s string) int { return utf8.RuneCountInString(s) }

// capRunes cuts s to at most max runes, at a sentence's end when one is
// near, and closes it.
func capRunes(s string, max int) string {
	if max <= 0 || runes(s) <= max {
		return s
	}
	rs := []rune(s)
	cut := max
	for i := max - 1; i > max/2; i-- {
		if strings.ContainsRune("。！？；", rs[i]) {
			cut = i + 1
			break
		}
	}
	if cut >= len(rs) {
		return s
	}
	out := strings.TrimRight(string(rs[:cut]), "，、 ")
	if !strings.HasSuffix(out, "。") {
		// Make room for the closing mark, so the cap holds.
		if tail := []rune(out); len(tail) >= max {
			out = string(tail[:len(tail)-1])
		}
		out += "。"
	}
	if n := runes(out); n > max {
		out = string([]rune(out)[:max])
	}
	return out
}

func pick(r *rand.Rand, xs ...string) string {
	if len(xs) == 0 {
		return ""
	}
	if r == nil {
		return xs[0]
	}
	return xs[r.IntN(len(xs))]
}

// A Voice speaks text, and keeps what it said: on disk, so a line is
// made once, and in memory, so it plays at once. It asks the speech
// service for its natural voice first, and falls back to the system's
// own synthesizer when the network has none.
type Voice struct {
	dir string

	mu   sync.Mutex
	said map[string]*audio.Sound
	gen  sync.Mutex // one line is made at a time
}

// New returns a voice that keeps what it says in dir, or in memory alone
// when dir is "".
func New(dir string) *Voice {
	return &Voice{
		dir:  dir,
		said: map[string]*audio.Sound{},
	}
}

// Speed is how fast the voice plays its clips: one, the natural pace.
func (v *Voice) Speed() float64 { return 1 }

// Say returns text as a sound, made once and kept.
func (v *Voice) Say(text string) (*audio.Sound, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrNothing
	}
	if s := v.remember(text); s != nil {
		return s, nil
	}
	// The shelf keeps what past sessions said, whichever voice said it.
	if v.dir != "" {
		for _, engine := range []string{"edge", "say"} {
			if b, err := os.ReadFile(filepath.Join(v.dir, v.key(engine, text)+".wav")); err == nil {
				if s, err := audio.DecodeSound(b); err == nil {
					v.keep(text, s)
					return s, nil
				}
			}
		}
	}
	v.gen.Lock()
	defer v.gen.Unlock()
	// Another line may have been made while this one waited.
	if s := v.remember(text); s != nil {
		return s, nil
	}
	rate, samples, engine, err := v.make(text)
	if err != nil {
		return nil, err
	}
	s := audio.NewSound(rate, 1, samples)
	if v.dir != "" {
		path := filepath.Join(v.dir, v.key(engine, text)+".wav")
		if b, err := encodeWAV(rate, samples); err == nil {
			if os.MkdirAll(v.dir, 0o700) == nil {
				tmp := path + ".tmp"
				if os.WriteFile(tmp, b, 0o600) == nil {
					os.Rename(tmp, path)
				}
			}
		}
	}
	v.keep(text, s)
	return s, nil
}

// make turns text into mono samples: the service's voice when it
// answers, the system's own when it does not.
func (v *Voice) make(text string) (rate int, samples []float32, engine string, err error) {
	if raw, e := edgeSynthesize(text); e == nil {
		if r, s, e := decodeMP3(raw); e == nil {
			masterNatural(r, s)
			return r, s, "edge", nil
		}
	}
	if !available() {
		return 0, nil, "", ErrUnavailable
	}
	raw, e := synthesize(text, sayRate, sayPitch)
	if e != nil {
		return 0, nil, "", e
	}
	r, s, e := mastered(raw)
	if e != nil {
		return 0, nil, "", e
	}
	return r, s, "say", nil
}

// The two voices' settings, which name what a cache file holds.
const (
	sayRate  = 178
	sayPitch = 40
)

func (v *Voice) key(engine, text string) string {
	params := edgeVoice + "|" + edgePitch + "|" + edgeRate
	if engine != "edge" {
		params = voiceName() + "|" + strconv.Itoa(sayRate) + "|" + strconv.Itoa(sayPitch)
	}
	h := sha1.Sum([]byte(engine + "|" + params + "|" + text))
	return hex.EncodeToString(h[:])
}

func (v *Voice) remember(text string) *audio.Sound {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.said[text]
}

func (v *Voice) keep(text string, s *audio.Sound) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.said[text] = s
}

// mastered decodes a WAV the synthesizer made into mono samples, and
// masters them to sit close and warm.
func mastered(raw []byte) (int, []float32, error) {
	rate, s, err := decodeWAV(raw)
	if err != nil {
		return 0, nil, err
	}
	master(rate, s)
	return rate, s, nil
}

// decodeWAV reads a WAV into mono samples, whatever its channels.
func decodeWAV(raw []byte) (int, []float32, error) {
	d, _, err := audio.NewDecoder(bytes.NewReader(raw))
	if err != nil {
		return 0, nil, err
	}
	ch, rate := d.Channels(), d.SampleRate()
	if ch < 1 || rate <= 0 {
		return 0, nil, errors.New("dj: the clip has no audio")
	}
	var out []float32
	buf := make([]float32, 8192*ch)
	for {
		n, err := d.Read(buf)
		n -= n % ch
		for i := 0; i < n; i += ch {
			var sum float32
			for k := 0; k < ch; k++ {
				sum += buf[i+k]
			}
			out = append(out, sum/float32(ch))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, nil, err
		}
		if n == 0 {
			return 0, nil, io.ErrNoProgress
		}
	}
	return rate, out, nil
}
