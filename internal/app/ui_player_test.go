package app

import (
	"math"
	"testing"
	"time"
)

// The rainbow of a paused bar runs the rim and comes back: it leaves the
// top, reaches the end of its travel half a period on, and turns, never
// wrapping round as a wheel does.
func TestSweepRunsBackAndForth(t *testing.T) {
	const total = 1000
	at := func(d time.Duration) (float32, float32) {
		return sweepAt(time.Unix(0, int64(d)), total, sweepPause, 0.2)
	}
	start, length := at(0)
	if start != 0 {
		t.Errorf("at the start of the period the arc stands at %v, want 0", start)
	}
	if length != total*0.2 {
		t.Errorf("the arc is %v long, want a fifth of the rim", length)
	}
	if mid, _ := at(sweepPause / 2); math.Abs(float64(mid-(total-length))) > 0.5 {
		t.Errorf("half a period on the arc stands at %v, want its travel's end %v", mid, total-length)
	}
	// Everywhere in between it stays on the rim, and never goes round.
	for d := time.Duration(0); d < sweepPause; d += sweepPause / 60 {
		s, l := at(d)
		if s < 0 || s+l > total+0.5 {
			t.Fatalf("%v in: the arc runs from %v to %v, off a rim of %v", d, s, s+l, total)
		}
	}
	// Back at the start of the next period it is where it began.
	if again, _ := at(sweepPause); again != 0 {
		t.Errorf("a period on the arc stands at %v, want 0 again", again)
	}
}
