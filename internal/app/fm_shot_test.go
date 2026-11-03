package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
	"netease-music-mygo/internal/lyrics"

	"netease-music-mygo/internal/dj"
	"netease-music-mygo/internal/netease"
)

// TestFMFrame renders the personal FM as it looks while playing, with a
// few songs already behind it, so the new touches — the reason, the lyric
// peek, and the just-played list — show in the shot.
func TestFMFrame(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	// The host between songs, so its switch shows on the page.
	a.dj.voice = dj.New(t.TempDir())
	a.dj.clips = map[int64]*djClip{}
	tracks := []netease.Track{
		{ID: 1, Name: "晴天", Artists: []netease.Artist{{Name: "周杰伦"}}, Album: netease.Album{Name: "叶惠美", PicURL: "pic1"}},
		{ID: 2, Name: "七里香", Artists: []netease.Artist{{Name: "周杰伦"}}, Album: netease.Album{Name: "七里香", PicURL: "pic2"}},
		{ID: 3, Name: "如也", Artists: []netease.Artist{{Name: "陈粒"}}, Album: netease.Album{Name: "如也", PicURL: "pic3"}},
		{ID: 4, Name: "易碎品", Artists: []netease.Artist{{Name: "汪苏泷"}}, Album: netease.Album{Name: "月光", PicURL: "pic4"}},
	}
	a.pl.Cue(tracks)
	a.pl.AttachRadio(fmRadio(netease.FMDefault), func(func([]netease.Track)) {})
	// A couple of songs already played, so the just-played list has them.
	a.pl.SetAt(2)
	// Striped covers, so the rows read.
	for i, id := range []int64{1, 2, 3, 4} {
		pic := fmt.Sprintf("pic%d", id)
		aw := &artwork{bmp: ui.NewBitmap(stripedCover()), tint: stripeColors[i%len(stripeColors)], used: a.art.now()}
		for _, size := range []int{160, 300, 500} {
			key := netease.Sized(pic, size)
			a.art.art[key] = aw
			a.art.art[key+"#backdrop"] = aw
		}
	}
	// The current song's lines, for the lyric peek's two rows.
	a.pl.SetLyrics([]lyrics.Line{
		{At: 0, Text: "故事的小黄花"},
		{At: 5 * time.Second, Text: "从出生那年就飘着"},
		{At: 10 * time.Second, Text: "童年的荡秋千"},
	})
	a.route = route{page: pageFM}
	h := 1000
	if v := os.Getenv("NM_H"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			h = n
		}
	}
	tt := ui.NewTester(a.view, 1100, h)
	tt.SetDark(os.Getenv("NM_DARK") != "")
	os.MkdirAll(dir, 0o755)
	render := func(name string) {
		tt.Advance(400 * time.Millisecond)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	name := "fm.png"
	if os.Getenv("NM_DARK") != "" {
		name = "fm-dark.png"
	}
	render(name)
	// The host speaking, so the card shows what it says between songs.
	a.dj.player = audio.NewSound(22050, 1, make([]float32, 22050)).NewPlayer()
	a.dj.next = &tracks[3]
	render(strings.TrimSuffix(name, ".png") + "-dj.png")
}
