package dj

import (
	"math"
	"math/rand/v2"
	"strings"
	"testing"
)

func seeded(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 7)) }

func TestScriptSaysTitleAndArtist(t *testing.T) {
	f := Facts{Title: "晴天", Artist: "周杰伦", Album: "叶惠美", Year: 2003, Mode: "推荐"}
	got := Script(f, seeded(1), 60)
	if !strings.Contains(got, "《晴天》") {
		t.Fatalf("the script does not name the song: %q", got)
	}
	if !strings.Contains(got, "周杰伦") {
		t.Fatalf("the script does not name the artist: %q", got)
	}
	if !strings.Contains(got, "2003") {
		t.Fatalf("the script does not say the year: %q", got)
	}
}

func TestScriptCapped(t *testing.T) {
	f := Facts{
		Title: "晴天", Artist: "周杰伦", Album: "叶惠美", Year: 2003,
		Note: "《叶惠美》是周杰伦的第四张专辑，收录了多首经典作品，风格横跨摇滚与民谣。",
		Mode: "推荐",
	}
	for max := 12; max <= 80; max += 4 {
		got := Script(f, seeded(uint64(max)), max)
		if n := runes(got); n > max {
			t.Fatalf("Script(max=%d) said %d runes: %q", max, n, got)
		}
	}
}

func TestScriptDropsBlurbNoise(t *testing.T) {
	if got := cleanSentence("<p>暂无介绍</p>"); got != "" {
		t.Fatalf("a placeholder was said: %q", got)
	}
	if got := cleanSentence("http://music.163.com 专辑介绍"); got != "" {
		t.Fatalf("a link was said: %q", got)
	}
	if got := cleanSentence("《叶惠美》是周杰伦的第四张专辑。另有更多"); got != "《叶惠美》是周杰伦的第四张专辑。" {
		t.Fatalf("cleanSentence = %q", got)
	}
}

func TestStyleLineOnlyWhenNamed(t *testing.T) {
	if got := styleLine("这是一张摇滚专辑", seeded(3)); !strings.Contains(got, "摇滚") {
		t.Fatalf("styleLine = %q", got)
	}
	if got := styleLine("一张很好听的专辑", seeded(3)); got != "" {
		t.Fatalf("styleLine invented a style: %q", got)
	}
}

func TestScriptIsStableForTheSameSeed(t *testing.T) {
	f := Facts{Title: "晴天", Artist: "周杰伦", Album: "叶惠美", Year: 2003, Mode: "探索"}
	a := Script(f, seeded(42), 60)
	b := Script(f, seeded(42), 60)
	if a != b {
		t.Fatalf("the same seed said different things:\n%q\n%q", a, b)
	}
}

func TestMasterKeepsTheLengthAndLevel(t *testing.T) {
	rate := 22050
	s := make([]float32, rate) // one second
	for i := range s {
		s[i] = float32(0.2 * math.Sin(2*math.Pi*220*float64(i)/float64(rate)))
	}
	master(rate, s)
	if len(s) != rate {
		t.Fatalf("master changed the length: %d", len(s))
	}
	var peak float64
	for _, x := range s {
		if math.IsNaN(float64(x)) {
			t.Fatal("master made a NaN")
		}
		peak = math.Max(peak, math.Abs(float64(x)))
	}
	if peak > 0.9 {
		t.Fatalf("master left a peak of %.3f", peak)
	}
	// The fades leave the very ends near silence.
	if math.Abs(float64(s[0])) > 0.01 || math.Abs(float64(s[len(s)-1])) > 0.01 {
		t.Fatal("the ends were not faded")
	}
}

func TestEncodeWAVRoundTrip(t *testing.T) {
	rate := 22050
	s := []float32{0, 0.5, -0.5, 1, -1}
	b, err := encodeWAV(rate, s)
	if err != nil {
		t.Fatal(err)
	}
	gotRate, got, err := decodeWAV(b)
	if err != nil {
		t.Fatal(err)
	}
	if gotRate != rate {
		t.Fatalf("rate = %d", gotRate)
	}
	if len(got) != len(s) {
		t.Fatalf("samples = %d, want %d", len(got), len(s))
	}
	for i := range s {
		if math.Abs(float64(got[i]-s[i])) > 0.001 {
			t.Fatalf("sample %d = %f, want %f", i, got[i], s[i])
		}
	}
}
