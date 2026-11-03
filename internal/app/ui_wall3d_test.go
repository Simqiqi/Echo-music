package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// wall3App is an app showing the wall turning in depth, of a queue of n
// songs, the at-th playing.
func wall3App(t *testing.T, n, at int) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	colors := []ui.Color{ui.RGB(220, 120, 160), ui.RGB(90, 140, 220), ui.RGB(240, 200, 110),
		ui.RGB(120, 200, 170), ui.RGB(170, 120, 230), ui.RGB(230, 140, 100), ui.RGB(120, 170, 210)}
	var tracks []netease.Track
	for i := range n {
		url := fmt.Sprintf("art%d", i)
		a.art.art[netease.Sized(url, 240)] = &artwork{bmp: solidBitmap(colors[i%len(colors)]), tint: colors[i%len(colors)]}
		tracks = append(tracks, netease.Track{ID: int64(i + 1), Name: fmt.Sprintf("歌%03d", i),
			Duration: 200000, Album: netease.Album{PicURL: url}})
	}
	a.pl.Cue(tracks)
	if at > 0 {
		a.pl.Jump(at)
	}
	a.pl.SetLyrics(nil)
	a.nowPlaying, a.npPane = true, paneWall3D
	tt := ui.NewTester(a.view, 1200, 800)
	tt.SetDark(true)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	return a, tt
}

// The wall stands the song playing in the middle, its neighbours turned
// either side of it, and turns to the one played next.
func TestWall3DStandsTheSongPlayingInTheMiddle(t *testing.T) {
	a, tt := wall3App(t, 60, 0)
	middle := func(s string) ui.Rect {
		t.Helper()
		r := find(t, tt, s)
		// The wall stands turning while the pane shows, so the poster
		// playing has drifted a little past the middle.
		if x := r.X + r.W/2; x < 540 || x > 660 {
			t.Fatalf("%s stands at %v across, want it about the middle", s, x)
		}
		return r
	}
	mid := middle("歌000")
	// Either side of it, a poster turned away: smaller, and nearer the
	// middle than it stands.
	for _, s := range []string{"歌001", "歌003"} {
		if r := find(t, tt, s); r.W >= mid.W {
			t.Errorf("%s is %v wide, as wide as the one in front", s, r.W)
		}
	}
	a.pl.Jump(9)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	middle("歌009")
}

// A click on a poster plays its song, and the wall has no end: turned
// past the last poster, the first comes round again to be clicked.
func TestWall3DPlaysThePosterClicked(t *testing.T) {
	a, tt := wall3App(t, 60, 0)
	if err := tt.Click("歌003"); err != nil {
		t.Fatalf("no poster beside the one in front: %v", err)
	}
	tt.Frame()
	if got := a.pl.Current().Name; got != "歌003" {
		t.Errorf("playing %s, want 歌003", got)
	}
	// Turned most of the way round the wall, the first poster stands
	// there again, and plays.
	a.pl.Jump(5)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	tt.Scroll(600, 400, -55*float32(wall3Tile+wall3Gap), 0)
	tt.Frame()
	tt.Advance(3 * time.Second)
	tt.Frame()
	if err := tt.Click("歌000"); err != nil {
		t.Fatalf("the first poster never came round: %v", err)
	}
	if got := a.pl.Current().Name; got != "歌000" {
		t.Errorf("playing %s, want 歌000, come round the wall", got)
	}
}

// The wall turns of itself, slowly and for ever, whether or not the
// pointer is about; a hand upon it takes it over, and it goes on from
// where the hand leaves it.
func TestWall3DTurnsOfItself(t *testing.T) {
	a, tt := wall3App(t, 60, 0)
	at := a.wall3.off
	tt.Advance(time.Second)
	tt.Frame()
	if a.wall3.off <= at {
		t.Errorf("the wall stood at %v, it had not turned on of itself", a.wall3.off)
	}
	// And it never stands still.
	last := a.wall3.off
	tt.Advance(3 * time.Second)
	tt.Frame()
	if a.wall3.off <= last {
		t.Errorf("the wall went from %v to %v, not on round the wall", last, a.wall3.off)
	}
	// A hand upon it takes it over, and turns it backwards.
	tt.Press(600, 400)
	tt.Move(600-float32(wall3Tile+wall3Gap), 400)
	tt.Frame()
	if a.wall3.off >= last {
		t.Errorf("under the hand the wall stood at %v, not turned back from %v", a.wall3.off, last)
	}
	// And when the hand lets go, it goes on of itself from there.
	tt.Release(600-float32(wall3Tile+wall3Gap), 400)
	tt.Frame()
	held := a.wall3.off
	tt.Advance(5 * time.Second)
	tt.Frame()
	if a.wall3.off <= held {
		t.Errorf("after the hand, the wall went from %v to %v, not onwards", held, a.wall3.off)
	}
}

// Two fingers on the trackpad turn the wall, and it comes to rest with a
// poster in the middle, not between two.
func TestWall3DTurnsWithTheTrackpad(t *testing.T) {

	_, tt := wall3App(t, 60, 0)
	at := find(t, tt, "歌001")
	tt.Scroll(600, 400, -float32(wall3Tile+wall3Gap), 0)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	if got := find(t, tt, "歌001"); got.X >= at.X {
		t.Errorf("the poster beside it went from %v to %v, not towards the middle", at.X, got.X)
	}
}

// With NM_SHOTS set, saves the wall at rest and turned.
func TestWall3DFrames(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	_, tt := wall3App(t, 40, 2)
	// The pointer in the pane, so that the song playing shows along the
	// foot.
	tt.Move(600, 400)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	save := func(name string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	save("wall3d-rest.png")
	tt.Scroll(600, 400, -float32(wall3Tile+wall3Gap)/2, 0)
	tt.Frame()
	save("wall3d-turning.png")
}
