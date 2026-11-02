package app

import (
	"math"
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// The week's report gathers the days up: minutes and songs from the Monday
// of the week, the songs heard most at their head, and the artists their
// songs ran to.
func TestWeeklyReport(t *testing.T) {
	w := newWeeklyAt("")
	now := time.Date(2026, 11, 2, 15, 0, 0, 0, time.Local) // a Monday
	a := netease.Track{ID: 1, Name: "晴天", Album: netease.Album{PicURL: "p1"}}
	b := netease.Track{ID: 2, Name: "七里香", Album: netease.Album{PicURL: "p2"}}
	w.add(now, a, 600)                   // ten minutes
	w.add(now, b, 1200)                  // twenty
	w.add(now.AddDate(0, 0, -1), a, 300) // last week's day is left out
	w.add(now.AddDate(0, 0, -8), a, 900) // last week, left out too
	r := w.report(now)
	if r.LastWeekMinutes != 5 {
		t.Errorf("last week ran %d minutes, want 5", r.LastWeekMinutes)
	}
	// The adds ran at 15:00: the week's story is the afternoon.
	if r.Story != "这一周，你在下午听得最多" {
		t.Errorf("the week's story is %q, want the afternoon", r.Story)
	}
	if r.Songs != 2 {
		t.Errorf("the week heard %d songs, want 2", r.Songs)
	}
	if r.Minutes != 30 {
		t.Errorf("the week ran %d minutes, want 30", r.Minutes)
	}
	if len(r.Tops) != 2 || r.Tops[0].Track.Name != "七里香" || r.Tops[0].Minutes != 20 {
		t.Errorf("the week's head is %v, want 七里香 at 20 minutes", r.Tops)
	}
	// A fortnight is all the record keeps.
	if len(w.data.Days) > 14 {
		t.Errorf("the record keeps %d days, want no more than a fortnight", len(w.data.Days))
	}
}

// The equalizer passes the sound through its bands: at every band at
// nought it stands aside, and lifting the bass lifts the low.
func TestEQFilters(t *testing.T) {
	eq := player.NewEQ()
	eq.SetStream(44100, 2)
	if !eqFlat(eq) {
		t.Fatal("a fresh equalizer is not flat")
	}
	// Nothing to do at nought: the samples come back as they went.
	buf := []float32{0.5, -0.5, 0.25, -0.25}
	eq.Process(buf)
	if buf[0] != 0.5 {
		t.Errorf("a flat equalizer changed the sound to %v", buf[0])
	}
	// The bass lifted: a low tone comes through standing.
	var gains [10]float64
	gains[0] = 12
	eq.Set(gains)
	eq.SetStream(44100, 1)
	n := 4410
	one := make([]float32, n)
	for i := range one {
		one[i] = float32(math.Sin(2 * math.Pi * 60 * float64(i) / 44100))
	}
	eq.Process(one)
	var peak float64
	for _, v := range one {
		if a := math.Abs(float64(v)); a > peak {
			peak = a
		}
	}
	if peak < 0.9 {
		t.Errorf("the bass lifted to a peak of %v, want the low tone standing", peak)
	}
}

// The week's shapes beyond its totals: the hour it ran at, the average
// day, the fullest day, and the days it has run without pause.
func TestWeeklyStats(t *testing.T) {
	w := newWeeklyAt("")
	now := time.Date(2026, 11, 2, 21, 0, 0, 0, time.Local) // a Monday night
	a := netease.Track{ID: 1, Name: "晴天"}
	w.add(now, a, 1200)                                           // today: twenty minutes at nine
	w.add(now.AddDate(0, 0, -1), a, 600)                          // Sunday evening: ten
	w.add(now.AddDate(0, 0, -2), a, 600)                          // Saturday: ten
	w.add(now.AddDate(0, 0, -4), a, 300)                          // Thursday: five, the streak breaks
	w.add(time.Date(2026, 11, 2, 8, 0, 0, 0, time.Local), a, 600) // this morning: ten
	st := w.stats(now)
	if st.Streak != 3 {
		t.Errorf("the streak ran %d days, want 3", st.Streak)
	}
	// The week ran thirty minutes over one day: that is the average and
	// the fullest day alike.
	if st.AvgDay < 29.9 || st.AvgDay > 30.1 {
		t.Errorf("the average day ran %v minutes, want 30", st.AvgDay)
	}
	if st.BestDayMin != 30 {
		t.Errorf("the fullest day ran %d minutes, want 30", st.BestDayMin)
	}
	// Two hours of the clock carry the week: eight in the morning
	// and nine at night.
	if st.Hours24[21] <= 0 || st.Hours24[8] <= 0 {
		t.Errorf("the hours came to %v, want eight and nine in the record", st.Hours24)
	}
	if st.Fortnight[13] < 29.9 || st.Fortnight[13] > 30.1 {
		t.Errorf("today ran %v minutes, want 30", st.Fortnight[13])
	}
}

func eqFlat(e interface{ Gains() [10]float64 }) bool {
	for _, g := range e.Gains() {
		if g != 0 {
			return false
		}
	}
	return true
}
