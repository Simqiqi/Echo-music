package app

import (
	"testing"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/dj"
	"netease-music-mygo/internal/netease"
)

// The host opens the song playing: when its line is ready, the voice
// starts; while the music rests it keeps quiet; and a cancel stops the
// voice at once.
func TestDJArm(t *testing.T) {
	a := newTestApp()
	a.dj.clips = map[int64]*djClip{}
	a.dj.voice = dj.New(t.TempDir())
	a.settings.DJ = true
	a.pl.AttachRadio("fm:推荐", nil)
	a.pl.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 240000}})
	// The queue rests: nothing speaks.
	clip := &djClip{ready: true, sound: audio.NewSound(22050, 1, make([]float32, 22050))}
	a.dj.clips[1] = clip
	a.djArm()
	if a.dj.player != nil {
		t.Fatal("spoke while the music was not playing")
	}
	// Playing, it speaks, and the music ducks under it.
	a.pl.Hold(0)
	a.djArm()
	if a.dj.player == nil {
		t.Fatal("did not speak for the song playing")
	}
	if a.dj.track != 1 || a.dj.said != 1 {
		t.Fatalf("spoke of the wrong track: %d", a.dj.track)
	}
	a.djCancel()
	if a.dj.player != nil {
		t.Fatal("the cancel left a voice")
	}
	// A line that failed says nothing, and is not asked again.
	a.dj.clips[1] = &djClip{ready: true, err: dj.ErrNothing}
	a.dj.said = 0
	a.djArm()
	if a.dj.player != nil {
		t.Fatal("spoke with no sound")
	}
}

// A song whose line is not ready yet does not speak, and the next look
// is scheduled. The line is written for the song playing too, so that it
// can catch the song's first bars.
func TestDJArmWaitsForTheLine(t *testing.T) {
	a := newTestApp()
	a.dj.clips = map[int64]*djClip{}
	a.dj.voice = dj.New(t.TempDir())
	a.settings.DJ = true
	a.pl.AttachRadio("fm:推荐", nil)
	a.pl.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 240000}})
	a.pl.Hold(0)
	a.djArm()
	if a.dj.player != nil {
		t.Fatal("spoke before the line was made")
	}
	if a.dj.timer == nil {
		t.Fatal("did not plan another look")
	}
	a.dj.gen++
	a.dj.timer.Stop()
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
