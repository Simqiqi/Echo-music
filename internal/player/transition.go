package player

import (
	"time"

	"github.com/egoist/mygo/audio"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// How the songs hand over, when the listener keeps the smart transitions
// on: the one after opens before the one playing ends, and its first
// seconds rise under the last seconds of this one, so that the music goes
// on rather than stopping and starting again.
const (
	crossLong       = 5 * time.Second
	crossShort      = 2500 * time.Millisecond
	crossShortUnder = 45 * time.Second
	// An overlap never takes less than this, or it would be a cut.
	crossLeast = 600 * time.Millisecond
	// crossLead is how long before the overlap the next song opens: long
	// enough for its stream to arrive over the network and start playing.
	crossLead = 1500 * time.Millisecond
)

// crossFor is how long a track of length d hands over to the next: short
// songs get a shorter overlap, so that they are not all handover.
func crossFor(d time.Duration) time.Duration {
	if d < crossShortUnder {
		return crossShort
	}
	return crossLong
}

// transitions reports whether the smart transitions are on, as the
// settings say.
func (p *Player) transitions() bool {
	return p.Transition != nil && p.Transition()
}

// crossing is the song after the one playing, opened and playing its
// beginning under it, until the handover is done.
type crossing struct {
	gen    int // the load of the track it plays under
	at     int // where in order it stands
	track  netease.Track
	out    *audio.Player
	stream stream
	info   *netease.Stream
	gain   float64 // the track's normalizing gain
	level  fader   // its own, so that it rises as the other falls
	ready  bool    // opened and playing, silent
	began  bool    // the two overlap
}

// armCross schedules the handover of the track playing to the one after
// it: the next one opens before this one ends, and the two overlap over
// the last seconds. A new arm of a track left behind is overtaken, and
// does nothing.
func (p *Player) armCross(gen int) {
	p.crossGen++
	if !p.transitions() || p.out == nil || !p.playing {
		debugCross("armCross: skipped (on=%v out=%v playing=%v)", p.transitions(), p.out != nil, p.playing)
		return
	}
	dur := p.Duration()
	if dur <= 0 {
		debugCross("armCross: no duration")
		return
	}
	over := crossFor(dur)
	wait := dur - p.Position() - over - crossLead
	if wait < 0 {
		wait = 0
	}
	debugCross("armCross: dur=%v pos=%v over=%v wait=%v", dur, p.Position(), over, wait)
	mine := p.crossGen
	go func() {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		<-timer.C
		p.post(func() {
			if mine != p.crossGen || gen != p.gen || p.out == nil || !p.playing || p.seeking {
				debugCross("fire: dropped (arm=%v/%v gen=%v/%v playing=%v seeking=%v)", mine, p.crossGen, gen, p.gen, p.playing, p.seeking)
				return
			}
			// Too little left to overlap with: it goes as it is, and the
			// next one opens after it, as without transitions.
			if p.Duration()-p.Position() <= crossLeast || p.cross != nil {
				debugCross("fire: too late (left=%v) or already crossing=%v", p.Duration()-p.Position(), p.cross != nil)
				return
			}
			p.crossOpen(gen, over)
		})
	}()
}

// crossNext is where in order the song after this one stands, if there is
// one to hand over to: a queue that goes on by itself, not a repeat of
// this one, and not a radio whose next songs its feed has yet to give.
func (p *Player) crossNext() (int, bool) {
	n := len(p.order)
	if n == 0 || p.repeat == RepeatOne {
		return 0, false
	}
	next := p.at + 1
	if next >= n {
		if p.feed != nil || p.repeat == RepeatOff {
			return 0, false
		}
		next = 0
	}
	if next == p.at {
		return 0, false
	}
	return next, true
}

// crossOpen opens the song after this one, and starts it silent under the
// one playing: it comes up as this one goes down. A stream that fails to
// open leaves nothing behind, and the queue goes on as without
// transitions when this song ends.
func (p *Player) crossOpen(gen int, over time.Duration) {
	at, ok := p.crossNext()
	if !ok {
		return
	}
	c := &crossing{gen: gen, at: at, track: p.queue[p.order[at]]}
	p.cross = c
	level := "exhigh"
	if p.Quality != nil {
		level = p.Quality()
	}
	open := p.open
	if p.Cross != nil {
		open = p.Cross
	}
	debugCross("open: at=%d %q level=%s", at, c.track.Name, level)
	go func() {
		start := time.Now()
		info, s, err := open(c.track, level)
		debugCross("open done in %v: err=%v", time.Since(start), err)
		p.post(func() {
			if p.cross != c || gen != p.gen || !p.playing || err != nil {
				debugCross("open dropped: cross=%v gen=%v/%v playing=%v err=%v", p.cross == c, gen, p.gen, p.playing, err)
				if s != nil {
					s.Close()
				}
				if p.cross == c {
					p.cross = nil
				}
				return
			}
			c.stream, c.info, c.gain = s, info, normGain(info)
			if p.EQ != nil {
				p.EQ.SetStream(s.SampleRate(), s.Channels())
				c.out = audio.NewPlayer(eqDecoder{stream: s, eq: p.EQ})
			} else {
				c.out = audio.NewPlayer(s)
			}
			c.level.attach(c.out, 0)
			c.level.setGain(p.crossGain(c))
			c.ready = true
			c.out.Play()
			left := p.Duration() - p.Position()
			if left <= 0 {
				p.cancelCross()
				return
			}
			// The two overlap over what is left of this one, at most the
			// overlap itself.
			c.began = true
			d := min(over, left)
			debugCross("begin: left=%v d=%v", left, d)
			c.level.fade(1, d, nil)
			p.fade.fade(0, d, nil)
		})
	}()
}

// crossGain is the volume the song handing over plays at: the listener's,
// which the ear hears in powers, and the track's gain, if normalizing.
func (p *Player) crossGain(c *crossing) float64 {
	g := p.volume * p.volume
	if p.Normalize != nil && p.Normalize() {
		g *= c.gain
	}
	return g
}

// cancelCross lets go of the song opened to hand over to: none of it
// plays on, since the listener left, paused, jumped or sought.
func (p *Player) cancelCross() {
	c := p.cross
	if c == nil {
		return
	}
	p.cross = nil
	c.level.attach(nil, 1)
	if out := c.out; out != nil {
		out.Pause()
		time.AfterFunc(100*time.Millisecond, out.Close)
		return
	}
	if c.stream != nil {
		c.stream.Close()
	}
}

// adopt hands the player over to the song opened under the one playing,
// as that one ends: it has played its first seconds already, so the
// handover takes nothing of it away, and the music never stops.
func (p *Player) adopt(c *crossing) {
	debugCross("adopt: at=%d %q pos=%v", c.at, c.track.Name, c.out.Position())
	p.cross = nil
	c.level.attach(nil, 1)
	if out := p.out; out != nil {
		out.Pause()
		time.AfterFunc(100*time.Millisecond, out.Close)
	}
	p.gen++
	gen := p.gen
	p.at = c.at
	t := p.queue[p.order[c.at]]
	p.track = &t
	p.out, p.stream, p.info, p.gain = c.out, c.stream, c.info, c.gain
	p.loading, p.err, p.pending, p.awaiting, p.natural = false, "", 0, false, false
	p.lyrics, p.lyricsLoaded = nil, false
	p.fade.attach(p.out, 1)
	p.applyGain()
	p.announce(&t, nil)
	p.refill()
	p.fetchArt(gen, t)
	p.fetchLyrics(gen, t)
	p.out.OnEnd(func() {
		p.post(func() {
			if gen == p.gen {
				p.ended()
			}
		})
	})
	if p.playing {
		p.out.Play()
	}
	p.armCross(gen)
	p.sync()
}

// rearm brings the music back up when a seek took it out of the song's
// tail, then arms the handover again where the song now stands.
func (p *Player) rearm() {
	if p.out == nil || !p.playing {
		return
	}
	if d, at := p.Duration(), p.Position(); d > 0 && d-at > crossFor(d) {
		if p.fade.silent() {
			p.fade.fade(1, fadeIn, nil)
		}
	}
	p.armCross(p.gen)
}
