package player

import (
	"math"
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
)

// An interlude keeps the queue at a natural end, and Resume goes on from
// there: to the next track, or to a stop at the queue's end.
func TestInterludeHoldsAndResumes(t *testing.T) {
	ch, p := posted()
	defer p.out.Close()
	p.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 600000}, {ID: 2, Name: "二", Duration: 600000}})
	held := false
	p.Interlude = func(next *netease.Track) bool {
		if next != nil && next.ID == 2 {
			held = true
			return true
		}
		return false
	}
	p.ended()
	if !held || !p.held {
		t.Fatal("did not hold at the natural end")
	}
	if p.at != 0 || p.Current().ID != 1 {
		t.Fatalf("moved on while held: at %d", p.at)
	}
	p.Resume()
	if p.held {
		t.Fatal("still held")
	}
	if p.at != 1 || p.Current().ID != 2 {
		t.Fatalf("did not resume to the next track: at %d", p.at)
	}
	// What was posted of the load runs, and nothing is left behind.
	for i := 0; i < 4; i++ {
		select {
		case fn := <-ch:
			fn()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// No interlude, the next track loads at once, as before.
func TestInterludeLetsTheQueueGo(t *testing.T) {
	_, p := posted()
	defer p.out.Close()
	p.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 600000}, {ID: 2, Name: "二", Duration: 600000}})
	p.Interlude = func(*netease.Track) bool { return false }
	p.ended()
	if p.at != 1 || p.Current().ID != 2 {
		t.Fatalf("did not go on: at %d", p.at)
	}
}

// Held at the queue's end, with nothing after and no repeat, Resume
// stops.
func TestInterludeEndsTheQueue(t *testing.T) {
	_, p := posted()
	defer p.out.Close()
	p.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 600000}})
	p.Interlude = func(*netease.Track) bool { return true }
	p.ended()
	if !p.held {
		t.Fatal("did not hold")
	}
	p.Resume()
	if p.playing {
		t.Fatal("played on past the end")
	}
}

// Ducking lowers what is heard, and letting go brings it back.
func TestDuck(t *testing.T) {
	_, p := posted()
	defer p.out.Close()
	p.applyGain() // the fader's gain, of the listener's volume
	p.Duck(0.25, 0)
	if v := p.out.Volume(); math.Abs(v-0.25*p.volume*p.volume) > 0.01 {
		t.Fatalf("ducked to %v", v)
	}
	p.Duck(1, 0)
	if v := p.out.Volume(); math.Abs(v-p.volume*p.volume) > 0.01 {
		t.Fatalf("unducked to %v", v)
	}
}

// A duck ramps to its level over d, and stays until it is raised again:
// the app raises it when the voice is done.
func TestDuckRamps(t *testing.T) {
	_, p := posted()
	defer p.out.Close()
	p.Duck(0.2, 300*time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	p.fade.mu.Lock()
	d := p.fade.duck
	p.fade.mu.Unlock()
	if d >= 0.9 || d <= 0.1 {
		t.Fatalf("did not ramp: %v", d)
	}
	time.Sleep(400 * time.Millisecond)
	p.fade.mu.Lock()
	d = p.fade.duck
	p.fade.mu.Unlock()
	if math.Abs(d-0.2) > 0.001 {
		t.Fatalf("did not settle at the level: %v", d)
	}
	p.Duck(1, 0)
	p.fade.mu.Lock()
	d = p.fade.duck
	p.fade.mu.Unlock()
	if d != 1 {
		t.Fatalf("did not come back: %v", d)
	}
}
