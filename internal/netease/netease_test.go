package netease

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPlayableFollowsPrivilege(t *testing.T) {
	tracks := []Track{{ID: 1, Fee: 1}, {ID: 2, Fee: 8}, {ID: 3, Fee: 8}, {ID: 4, Fee: 1}}
	if tracks[0].Playable() || !tracks[1].Playable() {
		t.Fatal("without privileges, VIP songs do not play and free ones do")
	}
	withPrivileges(tracks, []Privilege{
		{ID: 1, Pl: 320000},           // VIP song, the listener subscribes
		{ID: 2, Pl: 0},                // nothing to play
		{ID: 3, St: -200, Pl: 128000}, // taken down
	})
	want := []bool{true, false, false, false}
	for i, tr := range tracks {
		if tr.Playable() != want[i] {
			t.Errorf("track %d: playable %v", tr.ID, tr.Playable())
		}
	}
}

// A song comes in two shapes: the newer endpoints call its length dt, the
// older ones duration. Both must leave it with a length.
func TestTrackReadsEitherDuration(t *testing.T) {
	var newer, older Track
	if err := json.Unmarshal([]byte(`{"id":1,"name":"晴天","dt":270738}`), &newer); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"id":2,"name":"起风了","duration":325000}`), &older); err != nil {
		t.Fatal(err)
	}
	if newer.Length() != 270738*time.Millisecond {
		t.Errorf("dt came to %v", newer.Length())
	}
	if older.Length() != 325000*time.Millisecond {
		t.Errorf("duration came to %v", older.Length())
	}
}

// A song the catalogue had no length for takes the one its stream told as
// it played, and keeps it; the catalogue's own length still wins.
func TestLengthLearnedFromStream(t *testing.T) {
	// The book is kept between runs too, so a fixed id would come to the
	// test already knowing its length.
	tr := Track{ID: time.Now().UnixNano()}
	if tr.Length() != 0 {
		t.Fatalf("a song with no length came to %v", tr.Length())
	}
	RememberLength(tr.ID, 3*time.Minute+20*time.Second)
	if got := tr.Length(); got != 200*time.Second {
		t.Errorf("the learned length came to %v, want 3m20s", got)
	}
	known := Track{ID: tr.ID, Duration: 1000}
	if got := known.Length(); got != time.Second {
		t.Errorf("the catalogue's length came to %v, want 1s", got)
	}
}
