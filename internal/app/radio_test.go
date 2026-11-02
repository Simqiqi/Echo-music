package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func TestCommentTime(t *testing.T) {
	now := time.Date(2026, 10, 25, 22, 0, 0, 0, time.Local)
	for _, c := range []struct {
		t    time.Time
		want string
	}{
		{now.Add(-20 * time.Second), "刚刚"},
		{now.Add(-42 * time.Minute), "42分钟前"},
		{now.Add(-5 * time.Hour), "17:00"},
		{now.Add(-23 * time.Hour), "昨天 23:00"},
		{time.Date(2026, 3, 8, 9, 0, 0, 0, time.Local), "3月8日"},
		{time.Date(2021, 4, 10, 9, 0, 0, 0, time.Local), "2021年4月10日"},
	} {
		if got := commentTime(c.t, now); got != c.want {
			t.Errorf("%v: %q, want %q", c.t, got, c.want)
		}
	}
}

// Heart mode playing, its card and Now Playing offer to leave it, which
// keeps the song playing.
func TestExitHeartMode(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	a.pl.Cue([]netease.Track{{ID: 1, Name: "晚风"}, {ID: 2, Name: "推荐的歌"}})
	a.pl.AttachRadio(heartRadio, func(got func([]netease.Track)) {})
	a.route = route{page: pageFM}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("退出心动模式"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.pl.Radio() != "" || a.pl.Current().ID != 1 || len(a.pl.Queue()) != 1 {
		t.Fatalf("radio %q, current %v, queue %d", a.pl.Radio(), a.pl.Current(), len(a.pl.Queue()))
	}
	if !tt.HasText("开启心动模式") {
		t.Errorf("card still on: %q", tt.Texts())
	}

	a.pl.AttachRadio(heartRadio, func(got func([]netease.Track)) {})
	a.nowPlaying = true
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("退出心动模式"); err != nil {
		t.Fatal(err)
	}
	if a.pl.Radio() != "" || a.pl.Current().ID != 1 {
		t.Fatalf("from Now Playing: radio %q, current %v", a.pl.Radio(), a.pl.Current())
	}
}

// The FM card's draw layer — the light's — must not stand between the
// listener and the controls on it.
func TestFMCardControlsClickable(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	a.pl.Cue([]netease.Track{{ID: 1, Name: "晚风"}})
	a.pl.AttachRadio(fmRadio(netease.FMDefault), func(func([]netease.Track)) {})
	a.route = route{page: pageFM}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("播放/暂停"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.pl.Playing() {
		t.Fatal("play button did not toggle")
	}
	if err := tt.Click("播放/暂停"); err != nil {
		t.Fatal(err)
	}
	if !a.pl.Playing() {
		t.Fatal("play button did not toggle back")
	}
	if err := tt.Click("下一首"); err != nil {
		t.Fatal(err)
	}
}

// The pointer's arrival on the card starts the light, once each time.
func TestFMSweepOnHover(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "晚风"}})
	a.pl.AttachRadio(fmRadio(netease.FMDefault), func(func([]netease.Track)) {})
	a.route = route{page: pageFM}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	tt.Move(600, 300)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if a.sky.sweepAt.IsZero() {
		t.Fatal("hover did not start the light")
	}
	if !a.sky.hover {
		t.Fatal("card not marked hovered")
	}
	// Off the card, the light waits for the pointer to come back.
	tt.Move(600, 700)
	tt.Advance(time.Second)
	tt.Frame()
	if a.sky.hover {
		t.Fatal("card still marked hovered")
	}
}
