package app

import (
	"math"
	"testing"
	"time"

	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
)

func newTestApp() *app {
	a := &app{
		playlists: map[int64]*netease.Playlist{}, loadingPl: map[int64]bool{}, plErr: map[int64]string{},
		artists: map[int64]*artistData{}, albums: map[int64]*albumData{}, users: map[int64]*userData{},
		podcasts: map[int64]*podcastData{}, podcastTab: &podcastTab{}, searches: map[route]*searchData{},
		lyricAt: -1, queueAt: -1,
		// A wallpaper of the same colors every run, so that a picture of
		// the app is the same picture: see TestWallpaperIsDrawn.
		wash: wallpaper{hue: 214, next: 262, angle: 132, lift: 0.5},
	}
	a.art = newArtCache(func(func()) {})
	a.pl = player.New(func(func()) {})
	a.pl.SetVolume(0.5)
	return a
}

// The home page's record keeps its angle across a pause and a late frame,
// so it neither snaps back to its start nor sticks there when playing
// starts again from a media key.
func TestHeroTurnHoldsAngle(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var s heroTurn
	frame := func(playing bool) bool {
		at = at.Add(time.Second / 60)
		return s.step(at, playing)
	}
	for i := 0; i < 60; i++ {
		frame(true)
	}
	if s.angle <= 0 {
		t.Fatal("the record did not turn")
	}
	// Pause: the speed glides down until the record comes to rest.
	for i := 0; i < 600 && frame(false); i++ {
	}
	if s.speed != 0 {
		t.Fatal("the record never came to rest:", s.speed)
	}
	stopped := s.angle
	for i := 0; i < 60; i++ {
		frame(false)
	}
	if s.angle != stopped {
		t.Fatal("the record moved while paused:", s.angle, stopped)
	}
	// A window hidden for ten seconds must not jump the record on return.
	at = at.Add(10 * time.Second)
	s.step(at, false)
	if math.Abs(s.angle-stopped) > 0.001 {
		t.Fatal("a late frame jumped the record:", s.angle, stopped)
	}
	// Playing again carries on from the angle it stopped at.
	frame(true)
	if s.angle < stopped || s.angle-stopped > 1 {
		t.Fatal("playing again jumped to another angle:", s.angle, stopped)
	}
}
