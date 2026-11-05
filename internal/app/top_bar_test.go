package app

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// topBarApp is an app at rest with one song cued, and the tester that
// builds its window: the top bar is what these tests watch.
func topBarApp(t *testing.T, tr netease.Track) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{tr})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	return a, tt
}

// The top bar says what is being sung, not only what plays: the line of the
// lyrics the song has reached, and it moves on as the song does.
func TestTopBarShowsTheLineBeingSung(t *testing.T) {
	a, tt := topBarApp(t, netease.Track{ID: 1, Name: "晴天", Duration: 200000})
	a.pl.SetLyrics([]lyrics.Line{
		{At: 0, Text: "故事的小黄花"},
		{At: 8 * time.Second, Text: "从出生那年就飘着"},
	})
	a.pl.Hold(9 * time.Second)
	tt.Frame()
	sung, _, _, ok := a.sungLine()
	if !ok || sung != "从出生那年就飘着" {
		t.Fatalf("sungLine = %q, %v: want the line at nine seconds", sung, ok)
	}
	if !tt.HasText("从出生那年就飘着") {
		t.Error("the top bar does not show the line being sung")
	}
}

// Without words — no lyrics, an instrumental — the card is what it was.
func TestTopBarWithoutLyricsIsQuiet(t *testing.T) {
	a, tt := topBarApp(t, netease.Track{ID: 1, Name: "晴天", Duration: 200000})
	a.pl.SetLyrics(nil)
	tt.Frame()
	if _, _, _, ok := a.sungLine(); ok {
		t.Error("sungLine answers with no lyrics")
	}
	if !tt.HasText("晴天") {
		t.Error("the card lost its name")
	}
}

// The heart in the top bar keeps what plays without opening Now Playing.
func TestTopBarHeartLikesInPlace(t *testing.T) {
	a, tt := topBarApp(t, netease.Track{ID: 7, Name: "三年二班", Duration: 200000})
	a.profile = &netease.Profile{UserID: 1, Nickname: "听者"}
	a.liked = map[int64]bool{}
	card, ok := tt.Find("正在播放")
	if !ok {
		t.Fatal("no card of what plays in the top bar")
	}
	// The heart is the card's last thing, at its right.
	tt.ClickAt(card.X+card.W-13, card.Y+card.H/2)
	tt.Frame()
	if !a.isLiked(&netease.Track{ID: 7}) {
		t.Error("the heart does not like the song")
	}
	if a.nowPlaying {
		t.Error("the heart opens Now Playing too")
	}
}

// The lamp beside a page's name only shows when there is something to say:
// it is absent while all is well, and it says what failed when it is not.
func TestTopBarLampShowsFailures(t *testing.T) {
	resetDiagnostics()
	defer resetDiagnostics()
	_, tt := topBarApp(t, netease.Track{ID: 1, Name: "晴天", Duration: 200000})
	if _, ok := tt.Find("网络状态"); ok {
		t.Fatal("the lamp shows with nothing wrong")
	}
	noteDiagnostic("播放", errors.New("dial tcp: timeout"))
	tt.Frame()
	dot, ok := tt.Find("网络状态")
	if !ok {
		t.Fatal("the lamp does not show after a failure")
	}
	tt.ClickAt(dot.X+dot.W/2, dot.Y+dot.H/2)
	tt.Frame()
	// The toast is drawn and cleared as the frame is made.
	if !tt.HasText("timeout") {
		t.Error("a click on the lamp says nothing")
	}
}

// The mark for a newer release shows only when the release is newer than
// the build, and a click asks the updater for its window.
func TestUpdateDotOnlyForANewerRelease(t *testing.T) {
	realVersion := appVersion
	appVersion = func() string { return "0.3.3" }
	defer func() { appVersion = realVersion }()
	a, tt := topBarApp(t, netease.Track{ID: 1, Name: "晴天", Duration: 200000})
	a.updatePeek = updatePeek{tag: "v0.0.1", at: time.Now()}
	tt.Frame()
	if _, ok := tt.Find("有新版本 v0.0.1"); ok {
		t.Fatal("the mark shows for an older release")
	}
	a.updatePeek = updatePeek{tag: "v99.0.0", at: time.Now()}
	asked := 0
	real := checkForUpdates
	checkForUpdates = func() { asked++ }
	defer func() { checkForUpdates = real }()
	tt.Frame()
	dot, ok := tt.Find("有新版本 v99.0.0")
	if !ok {
		t.Fatal("the mark does not show for a newer release")
	}
	tt.ClickAt(dot.X+dot.W/2, dot.Y+dot.H/2)
	tt.Frame()
	if asked != 1 {
		t.Errorf("the mark opened the updater %d times, want once", asked)
	}
}

// Versions compare by their numbers, so that 0.3.10 is newer than 0.3.9 and
// a tag's "v" or pre-release tail does not matter.
func TestVersionComparison(t *testing.T) {
	cases := []struct {
		now, tag string
		out      bool
	}{
		{"0.3.3", "v0.3.3", false},
		{"0.3.3", "v0.3.4", true},
		{"0.3.9", "v0.3.10", true},
		{"0.3.3", "v0.2.9", false},
		{"0.3.3", "v0.4.0-beta.1", true},
	}
	for _, c := range cases {
		if got := versionLess(c.now, c.tag); got != c.out {
			t.Errorf("versionLess(%q, %q) = %v, want %v", c.now, c.tag, got, c.out)
		}
	}
}
