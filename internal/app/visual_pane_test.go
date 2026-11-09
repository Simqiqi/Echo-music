package app

import (
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The shaders are strings the Go compiler cannot see: this is what says
// they are well formed before a frame of them is shaded.
func TestVisualShadersCompile(t *testing.T) {
	nebulaShader()
	kaleidoShader()
}

// The kaleidoscope draws a field of light over the whole view and keeps
// the tabs and the words over it, in a window small enough for the
// software renderer of the tests to shade in reasonable time. (The nebula
// is no longer its own tab — it lives behind every pane as a backdrop —
// and is covered by TestNebulaBackdropRenders.)
func TestVisualPanesRender(t *testing.T) {
	for _, tc := range []struct {
		name string
		pane int
	}{
		{"kaleido", paneKaleido},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			a.art.art[netease.Sized("art1", 512)] = &artwork{
				bmp: ui.NewBitmap(stripedCover()), tint: stripeColors[2],
			}
			a.pl.Cue([]netease.Track{{ID: 1, Name: "歌", Duration: 200000,
				Album: netease.Album{PicURL: "art1"}}})
			a.pl.SetLyrics(nil)
			a.nowPlaying, a.npPane = true, tc.pane
			tt := ui.NewTester(a.view, 120, 90)
			tt.SetDark(true)
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			shot := tt.Image()
			// A little off the middle, where the field is brightest.
			px := color.NRGBAModel.Convert(shot.At(63, 43)).(color.NRGBA)
			if px.R+px.G+px.B < 12 {
				t.Errorf("the field is dark at its middle: %v", px)
			}
			if _, ok := tt.Find("歌词"); !ok {
				t.Error("the row of tabs is gone")
			}
		})
	}
}

// TestNebulaBackdropRenders: the nebula, no longer a tab, paints as a
// backdrop behind every pane when the listener picks it. This is what
// exercises drawNebula on the backdrop canvas (the GPU path the tab used is
// gone with the tab), so it must shade and set the cache to nebula.
func TestNebulaBackdropRenders(t *testing.T) {
	a := newTestApp()
	s := defaultSettings()
	s.setBackdrop("nebula")
	a.settings = s
	url := "https://p1.music.126.net/nebula.jpg"
	a.art.art[netease.Sized(url, 600)+"#backdrop"] = &artwork{tint: ui.RGB(52, 84, 176)}
	a.pl.Cue([]netease.Track{{ID: 7, Name: "Nebula", Duration: 240000,
		Album: netease.Album{ID: 1, Name: "Space", PicURL: url}, Artists: []netease.Artist{{Name: "Bowie"}}}})
	a.pl.SetLyrics(nil)
	a.nowPlaying, a.npPane = true, paneLyrics
	tt := ui.NewTester(a.view, 1000, 700)
	tt.SetDark(true)
	tt.Frame()
	tt.Advance(1200 * time.Millisecond)
	tt.Frame()
	if a.backdropCache.style != "nebula" {
		t.Errorf("the backdrop is %q, want nebula", a.backdropCache.style)
	}
	if !tt.HasText("Nebula") {
		t.Fatal("now playing did not open on the song")
	}
}
