package player

import (
	"testing"
	"time"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// openable is the silence the song a handover brings in plays: it opens
// without a network, as the songs of the test never play.
type openable struct{ silence }

func (openable) Close() error                   { return nil }
func (openable) source() (*download, *spectrum) { return nil, nil }

// crossing_ starts a player on a two-song queue whose first song is four
// seconds long, with the smart transitions on: the handover begins at
// once, since it takes the song's last seconds.
func crossing_(t *testing.T) (chan func(), *Player) {
	t.Helper()
	ch, p := posted()
	p.Cue([]netease.Track{
		{ID: -1, Name: "一", Duration: 4000},
		{ID: -1, Name: "二", Duration: 4000},
	})
	p.Transition = func() bool { return true }
	p.BridgeLyrics = func(netease.Track) (string, string, error) { return "", "", errNoLyrics }
	p.Cross = func(netease.Track, string) (*netease.Stream, stream, error) {
		return nil, openable{}, nil
	}
	p.armCross(p.gen)
	return ch, p
}

var errNoLyrics = &lyricsError{}

type lyricsError struct{}

func (*lyricsError) Error() string { return "no lyrics" }

// wait runs what the player posts to the main thread until it says done.
func wait(t *testing.T, ch chan func(), done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		select {
		case fn := <-ch:
			fn()
			if done() {
				return
			}
		case <-time.After(10 * time.Millisecond):
			if done() {
				return
			}
		}
	}
	t.Fatal("the handover did not begin")
}

// The song after this one opens before this one ends, and plays under it.
func TestTransitionOpensNextUnder(t *testing.T) {
	ch, p := crossing_(t)
	defer func() {
		p.cancelCross()
		if p.out != nil {
			p.out.Close()
		}
	}()
	wait(t, ch, func() bool { return p.cross != nil && p.cross.ready && p.cross.began })
	c := p.cross
	if c.at != 1 {
		t.Errorf("handed over to %d", c.at)
	}
	if !c.out.Playing() {
		t.Error("the next one is not playing under this one")
	}
	// Its beginning plays on, under the song ending: it does not wait for
	// that one to be over.
	time.Sleep(300 * time.Millisecond)
	if c.out.Position() <= 0 {
		t.Error("the next one has not played its beginning")
	}
	if !p.playing || p.track == nil || p.track.Name != "一" {
		t.Error("the song playing changed before it ended")
	}
	// As it ends, the next one takes over where it is: nothing of it is
	// played again.
	at := c.out.Position()
	p.ended()
	if p.track == nil || p.track.Name != "二" {
		t.Fatalf("did not hand over: %v", p.track)
	}
	if p.out != c.out {
		t.Error("opened the next one again rather than taking it over")
	}
	if p.out.Position() < at-100*time.Millisecond {
		t.Errorf("played its beginning again: %v after %v", p.out.Position(), at)
	}
	if p.err != "" || p.loading {
		t.Errorf("loading after the handover: %q", p.err)
	}
}

// A repeat of the one song has nothing to hand over to.
func TestTransitionNotForRepeatOne(t *testing.T) {
	_, p := crossing_(t)
	defer p.out.Close()
	p.repeat = RepeatOne
	if _, ok := p.crossNext(); ok {
		t.Fatal("handed over to itself")
	}
}

// Pausing lets the song opened under this one go: none of the two plays
// on, and the music waits where it was.
func TestTransitionCancelOnPause(t *testing.T) {
	ch, p := crossing_(t)
	defer func() {
		p.cancelCross()
		if p.out != nil {
			p.out.Close()
		}
	}()
	wait(t, ch, func() bool { return p.cross != nil && p.cross.ready })
	c := p.cross
	p.TogglePlay()
	if p.cross != nil {
		t.Error("kept the next one open while paused")
	}
	time.Sleep(150 * time.Millisecond)
	if c.out.Playing() {
		t.Error("the next one plays on while paused")
	}
}
