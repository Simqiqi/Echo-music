package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func TestVinylFrame(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, shot := range []struct {
		name    string
		playing bool
	}{{"vinyl-play.png", true}, {"vinyl-pause.png", false}} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.Vinyl = true
		a.art.art[netease.Sized("art1", 512)] = &artwork{bmp: ui.NewBitmap(stripedCover()), tint: stripeColors[2]}
		a.pl.Cue([]netease.Track{{ID: 1, Name: "歌", Duration: 200000, Album: netease.Album{PicURL: "art1"}}})
		a.pl.SetLyrics(nil)
		a.nowPlaying = true
		if shot.playing {
			a.pl.TogglePlay()
		}
		tt := ui.NewTester(a.view, 1000, 680)
		tt.SetDark(true)
		tt.Advance(800 * time.Millisecond)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, shot.name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}
