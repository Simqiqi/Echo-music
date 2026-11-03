package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// Until the listener sets the clarity, every pane keeps its own, and the
// setting reads as the middle of the range.
func TestGlassClarityDefaults(t *testing.T) {
	s := defaultSettings()
	if s.clarity() != 0.5 {
		t.Fatal("the clarity without a setting is not the middle:", s.clarity())
	}
	a := newTestApp()
	a.settings = s
	if got := a.glassClarity(0.4); got != 0.4 {
		t.Fatal("a pane's own clarity is overridden:", got)
	}
	v := 0.8
	a.settings.GlassClarity = &v
	if got := a.glassClarity(0.4); got != 0.8 {
		t.Fatal("the set clarity does not reach the panes:", got)
	}
	// Outside the slider's ends, it holds to them.
	w := 3.0
	a.settings.GlassClarity = &w
	if got := a.settings.clarity(); got != clarityMost {
		t.Fatal("the clarity passes its end:", got)
	}
}

// The clarity slider sits under the glass switch, and goes with it: gone
// when the glass is.
func TestClarityRowFollowsTheGlass(t *testing.T) {
	on := true
	off := false
	mk := func(glass *bool) *ui.Tester {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.Glass = glass
		tt := ui.NewTester(a.settingsTestView, 520, 900)
		tt.Frame()
		return tt
	}
	if tt := mk(&on); !tt.HasText("玻璃清晰度") {
		t.Fatal("the clarity row is missing while the glass is on")
	}
	if tt := mk(&off); tt.HasText("玻璃清晰度") {
		t.Fatal("the clarity row shows while the glass is off")
	}
}

// The mini player shows what plays and its controls.
func TestMiniPlayerView(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tr := netease.Track{ID: 7, Name: "海屿你", Artists: []netease.Artist{{Name: "马也_Crabbit"}}, Duration: 200000}
	a.pl.Cue([]netease.Track{tr})
	a.pl.Seek(42000 * time.Millisecond)
	tt := ui.NewTester(a.miniView, miniWidth, miniHeight)
	tt.Frame()
	for _, s := range []string{"海屿你", "马也_Crabbit", "上一首", "下一首", "展开"} {
		if !tt.HasText(s) {
			t.Fatal("the mini player is missing:", s)
		}
	}
	if err := tt.Click("展开"); err != nil {
		t.Fatal("the mini player does not hand back:", err)
	}
}

// Without a track the mini player shows nothing to play.
func TestMiniPlayerWithoutATrack(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.miniView, miniWidth, miniHeight)
	tt.Frame()
	if tt.HasText("展开") {
		t.Fatal("the mini player shows controls without a track")
	}
	if !tt.HasText("未在播放") {
		t.Fatal("the mini player does not say nothing plays")
	}
}
