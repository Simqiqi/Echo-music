package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The capsule under Now Playing's tabs is a material beneath them, not a
// plate over them: a tab still takes its click with Liquid Glass on or off.
func TestNowPlayingTabsClickThroughGlass(t *testing.T) {
	for _, on := range []bool{true, false} {
		name := "glass off"
		if on {
			name = "glass on"
		}
		t.Run(name, func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			a.settings.Glass = &on
			a.pl.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 200000}})
			a.pl.SetLyrics(nil)
			a.nowPlaying = true

			tt := ui.NewTester(a.view, 1200, 800)
			tt.Frame()
			tt.Advance(time.Second)
			tt.Frame()
			if err := tt.Click("动态"); err != nil {
				t.Fatalf("click on 律动: %v", err)
			}
			tt.Frame()
			if a.npPane != paneStage {
				t.Errorf("npPane = %d, want the stage (%d)", a.npPane, paneStage)
			}
		})
	}
}
