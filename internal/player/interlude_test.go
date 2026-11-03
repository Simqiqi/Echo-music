package player

import (
	"math"
	"testing"
	"time"
)

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
