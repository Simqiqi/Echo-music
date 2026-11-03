package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// peekApp is an app with a track cued and lyrics set, as the sidebar's
// lyric peek needs.
func peekApp(t *testing.T, pos time.Duration) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	tr := netease.Track{ID: 7, Name: "海屿你", Artists: []netease.Artist{{Name: "马也_Crabbit"}}, Duration: 200000}
	a.pl.Cue([]netease.Track{tr})
	a.pl.SetLyrics(lyrics.Build(
		"[00:13.00]我从不主动示弱\n[00:30.56]曾经的那些发生过的开心和难过\n\n[00:41.00]我们是否还在原地",
		"", ""))
	a.pl.Seek(pos)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	return a, tt
}

// The peek shows the line being sung and the next with words in it, and
// a click on it opens the lyrics pane.
func TestLyricPeekShowsTheLineSung(t *testing.T) {
	a, tt := peekApp(t, 14000*time.Millisecond)
	if !tt.HasText("我从不主动示弱") {
		t.Fatal("the line being sung is missing")
	}
	if !tt.HasText("曾经的那些发生过的开心和难过") {
		t.Fatal("the next line is missing")
	}
	// The blank line between verses is skipped, so the second row is the
	// line after it.
	if tt.HasText("我们是否还在原地") {
		t.Fatal("a line past the next shows")
	}
	if err := tt.Click("歌词速览"); err != nil {
		t.Fatal("the peek is not clickable:", err)
	}
	if !a.nowPlaying || a.npPane != paneLyrics {
		t.Fatal("a click does not open the lyrics pane")
	}
}

// Before the first line nothing is sung, so only what comes shows.
func TestLyricPeekBeforeTheFirstLine(t *testing.T) {
	_, tt := peekApp(t, time.Second)
	if tt.HasText("曾经的那些发生过的开心和难过") {
		t.Fatal("a later line shows before the first")
	}
	if !tt.HasText("我从不主动示弱") {
		t.Fatal("the first line does not show")
	}
}

// With nothing to show — no lyrics, an instrumental — the peek is not
// there at all.
func TestLyricPeekWithoutLyrics(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 7, Name: "海屿你", Duration: 200000}})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if _, ok := tt.Find("歌词速览"); ok {
		t.Fatal("the peek shows without lyrics")
	}
}
