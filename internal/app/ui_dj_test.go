package app

import (
	"testing"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/dj"
	"netease-music-mygo/internal/netease"
)

// The host speaks over the end of a track, and the queue waits for it: a
// voice for the track next in line holds it, one for another does not,
// and a cancel lets it go at once.
func TestDJInterlude(t *testing.T) {
	a := newTestApp()
	a.dj.clips = map[int64]*djClip{}
	a.pl.Cue([]netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}})
	next := &netease.Track{ID: 2, Name: "二"}
	if a.djInterlude(next) {
		t.Fatal("held with no voice")
	}
	a.dj.voice = dj.New(t.TempDir())
	a.dj.at, a.dj.next = 2, next
	a.dj.player = audio.NewSound(8000, 1, nil).NewPlayer()
	if !a.djInterlude(next) {
		t.Fatal("did not hold for the voice")
	}
	if !a.dj.held {
		t.Fatal("the hold was not marked")
	}
	if a.djInterlude(&netease.Track{ID: 3, Name: "三"}) {
		t.Fatal("held for another track")
	}
	a.djCancel()
	if a.dj.player != nil || a.dj.held {
		t.Fatal("the cancel left a voice or a hold")
	}
}

// The host speaks only when the listener asked for it and there is a
// voice to speak with.
func TestDJEnabled(t *testing.T) {
	a := newTestApp()
	a.dj.clips = map[int64]*djClip{}
	a.settings.DJ = true
	if a.djEnabled() {
		t.Fatal("enabled with no voice")
	}
	a.dj.voice = dj.New("")
	if !a.djEnabled() {
		t.Fatal("not enabled with a voice")
	}
	a.settings.DJ = false
	if a.djEnabled() {
		t.Fatal("enabled while off")
	}
}

// The facts the host is given name the track, and say how the FM picked
// it.
func TestDJFacts(t *testing.T) {
	a := newTestApp()
	a.fmMode = 2 // 探索
	tr := &netease.Track{
		ID: 7, Name: "如也", Artists: []netease.Artist{{Name: "陈粒"}},
		Album:  netease.Album{Name: "如也"},
		Reason: "因为你听过陈粒",
	}
	f := a.djFacts(tr)
	if f.Title != "如也" || f.Artist != "陈粒" || f.Album != "如也" {
		t.Fatalf("the facts miss the track: %+v", f)
	}
	if f.Mode != fmModes[2].label {
		t.Fatalf("mode = %q", f.Mode)
	}
	if f.Reason != "因为你听过陈粒" {
		t.Fatalf("reason = %q", f.Reason)
	}
}
