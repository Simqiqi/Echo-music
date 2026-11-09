package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The neon pane hangs the words up as a sign: this is what says it draws
// without a panic, with the line being sung lit and the one after it dark.
func TestNeonPaneRenders(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "歌", Duration: 200000}})
	a.pl.SetLyrics([]lyrics.Line{
		{At: 0, End: 2 * time.Second, Text: "灯光照在墙上", Words: words("灯", "光", "照", "在", "墙", "上")},
		{At: 2 * time.Second, End: 4 * time.Second, Text: "夜还没有走", Words: words("夜", "还", "没", "有", "走")},
	})
	a.nowPlaying, a.npPane = true, paneNeon
	tt := ui.NewTester(a.view, 560, 420)
	tt.SetDark(true)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if _, ok := tt.Find("灯"); !ok {
		t.Error("the line being sung is not drawn")
	}
	// And the tabs stay over it, so the pane can be left.
	if _, ok := tt.Find("歌词"); !ok {
		t.Error("the row of tabs is gone")
	}
}
