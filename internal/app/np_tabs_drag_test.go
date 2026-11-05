package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// Holding a tab and sliding across the row chooses the tab the pointer
// passes, without a click on it; letting go leaves the choice there.
func TestNowPlayingTabsDragToChoose(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 200000}})
	a.pl.SetLyrics(nil)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()

	from, ok := tt.Find("歌词")
	if !ok {
		t.Fatal("no 歌词 tab")
	}
	to, ok := tt.Find("律动")
	if !ok {
		t.Fatal("no 律动 tab")
	}
	x0, y0 := from.X+from.W/2, from.Y+from.H/2
	x1, y1 := to.X+to.W/2, to.Y+to.H/2

	tt.Press(x0, y0)
	for i := 1; i <= 6; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Move(x0+(x1-x0)*float32(i)/6, y0+(y1-y0)*float32(i)/6)
		tt.Frame()
	}
	if a.npPane != paneStage {
		t.Errorf("dragging to 律动: npPane = %d, want the stage", a.npPane)
	}
	tt.Release(x1, y1)
	tt.Advance(time.Second)
	tt.Frame()
	if a.npPane != paneStage {
		t.Errorf("after letting go: npPane = %d, want the stage", a.npPane)
	}
}
