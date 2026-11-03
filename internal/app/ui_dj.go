package app

import (
	"math"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/dj"
	"netease-music-mygo/internal/netease"
)

// The DJ between songs: in the personal FM, the app lowers the music a
// little as the last seconds of a track play, and lets a warm Mandarin
// voice say what comes next, why it was picked, and where it is from,
// before the next song begins. The voice is made on the device, from the
// track's own facts, and kept, so that a line is made once.

const (
	// djMax is how much the host says, in runes: long enough for a
	// reason and a little background, short enough not to hold the next
	// song back for long.
	djMax = 52
	// djDuck is how much of the music is heard while the voice speaks.
	djDuck = 0.22
	// djRetry is how long the app waits for the next track, or its line,
	// to be ready before looking again.
	djRetry = 2 * time.Second
)

// A djClip is what the host says about a track, once made.
type djClip struct {
	ready bool
	text  string
	sound *audio.Sound
	err   error
}

// djState is the voice between songs, and what it is saying.
type djState struct {
	voice  *dj.Voice
	clips  map[int64]*djClip // by the track introduced

	timer    *time.Timer
	gen      int // counts schedules, to drop those overtaken
	player   *audio.Player
	track    int64 // the track the voice speaks over
	at       int64 // the track it introduces
	next     *netease.Track
	held     bool // the queue waits for the voice
	said     int64
	pausedBy bool // the voice is paused with the music
}

// initDJ makes the voice, and the shelf of lines it keeps.
func (a *app) initDJ() {
	a.dj.clips = map[int64]*djClip{}
	if !dj.Available() {
		return
	}
	dir, err := mygo.App.Path(mygo.PathCache)
	if err != nil {
		a.dj.voice = dj.New("")
		return
	}
	a.dj.voice = dj.New(filepath.Join(dir, "dj"))
}

// djEnabled reports whether the host may speak: the listener asked for
// it, there is a voice, and a radio plays.
func (a *app) djEnabled() bool {
	return a.settings.DJ && a.dj.voice != nil && a.pl != nil
}

// djActive reports whether a voice is wanted between the songs now: the
// host speaks in the personal FM and in heart mode, which play on without
// end.
func (a *app) djActive() bool {
	if !a.djEnabled() {
		return false
	}
	r := a.pl.Radio()
	return strings.HasPrefix(r, "fm:") || r == heartRadio
}

// djNext returns the track next in line, or nil.
func (a *app) djNext() *netease.Track {
	if a.pl == nil {
		return nil
	}
	order, at := a.pl.PlayOrder()
	q := a.pl.Queue()
	if at < 0 || at+1 >= len(order) {
		return nil
	}
	i := order[at+1]
	if i < 0 || i >= len(q) {
		return nil
	}
	return &q[i]
}

// djFacts is what the host knows of a track before it looks anything up.
func (a *app) djFacts(tr *netease.Track) dj.Facts {
	f := dj.Facts{
		Title:  tr.Name,
		Artist: tr.ArtistNames(),
		Album:  tr.Album.Name,
		Reason: tr.Reason,
	}
	if a.pl.Radio() == heartRadio {
		f.Mode = "心动"
	} else if a.fmMode >= 0 && a.fmMode < len(fmModes) {
		f.Mode = fmModes[a.fmMode].label
	}
	return f
}

// djPrefetch writes and makes what the host will say about the next
// track, while the one playing has minutes to go.
func (a *app) djPrefetch() {
	if !a.djActive() {
		return
	}
	next := a.djNext()
	if next == nil {
		return
	}
	if _, ok := a.dj.clips[next.ID]; ok {
		return
	}
	clip := &djClip{}
	a.dj.clips[next.ID] = clip
	f := a.djFacts(next)
	tr := *next
	go func() {
		// Where the track comes from, as its album says: the year, and a
		// word of what the album is about.
		if tr.Album.ID != 0 {
			if al, err := netease.AlbumDetail(tr.Album.ID); err == nil {
				if al.PublishTime > 0 {
					f.Year = time.UnixMilli(al.PublishTime).Year()
				}
				f.Note = al.Description
			}
		}
		r := rand.New(rand.NewPCG(uint64(tr.ID), uint64(time.Now().UnixNano())))
		text := dj.Script(f, r, djMax)
		if text == "" {
			a.post(func() {
				clip.err, clip.ready = dj.ErrNothing, true
			})
			return
		}
		snd, err := a.dj.voice.Say(text)
		a.post(func() {
			clip.text, clip.sound, clip.err, clip.ready = text, snd, err, true
			// Now that the line is ready, it can be timed.
			a.djSchedule()
		})
	}()
}

// djLead is how long before the song ends the voice begins: it speaks
// over the song's own fade, and goes on into the gap, so that the next
// song is not held back for the whole line.
func djLead(d time.Duration) time.Duration {
	return min(900*time.Millisecond+d/4, 2600*time.Millisecond)
}

// djSchedule times the host's line for the track playing: it begins a
// little before the end, so that it crosses the gap.
func (a *app) djSchedule() {
	a.dj.gen++
	gen := a.dj.gen
	if a.dj.timer != nil {
		a.dj.timer.Stop()
		a.dj.timer = nil
	}
	if !a.djActive() || !a.pl.Playing() {
		return
	}
	tr := a.pl.Current()
	if tr == nil || a.dj.said == tr.ID {
		return
	}
	next := a.djNext()
	if next == nil {
		a.djLater(gen)
		return
	}
	clip := a.dj.clips[next.ID]
	switch {
	case clip == nil, !clip.ready:
		// Not made yet: look again in a moment.
		a.djLater(gen)
		return
	case clip.sound == nil:
		// It failed: nothing to say, and no use asking again.
		return
	}
	left := a.pl.Duration() - a.pl.Position()
	if left <= 0 {
		return
	}
	delay := left - djLead(clip.sound.Duration())
	if delay < 0 {
		delay = 0
	}
	id := tr.ID
	a.dj.timer = time.AfterFunc(delay, func() {
		a.post(func() {
			if gen != a.dj.gen {
				return
			}
			cur := a.pl.Current()
			if cur == nil || cur.ID != id || !a.pl.Playing() {
				return
			}
			a.djStart(id, next, clip)
		})
	})
}

// djLater looks again in a moment, for a next track or its line not yet
// ready.
func (a *app) djLater(gen int) {
	a.dj.timer = time.AfterFunc(djRetry, func() {
		a.post(func() {
			if gen == a.dj.gen {
				a.djSchedule()
			}
		})
	})
}

// djStart lets the host speak over the end of the track playing.
func (a *app) djStart(track int64, next *netease.Track, clip *djClip) {
	if a.dj.player != nil {
		return
	}
	a.dj.track, a.dj.at, a.dj.next = track, next.ID, next
	a.dj.said, a.dj.held, a.dj.pausedBy = track, false, false
	p := clip.sound.Play()
	p.SetSpeed(a.dj.voice.Speed())
	p.SetVolume(a.volume * a.volume)
	a.dj.player = p
	a.pl.Duck(djDuck, 360*time.Millisecond)
	p.OnEnd(func() {
		a.post(func() {
			if a.dj.player != p {
				return
			}
			a.djFinish()
		})
	})
	a.redraw()
}

// djFinish ends the host's line, and lets the music go on.
func (a *app) djFinish() {
	p := a.dj.player
	a.dj.player, a.dj.pausedBy = nil, false
	if p != nil {
		p.Close()
	}
	a.pl.Duck(1, 520*time.Millisecond)
	held := a.dj.held
	a.dj.held = false
	if held {
		a.pl.Resume()
	}
	a.djSchedule()
	a.redraw()
}

// djCancel stops the host at once, as the listener skips or turns it off.
func (a *app) djCancel() {
	a.dj.gen++
	if a.dj.timer != nil {
		a.dj.timer.Stop()
		a.dj.timer = nil
	}
	if a.dj.player == nil && !a.dj.held {
		return
	}
	p := a.dj.player
	a.dj.player, a.dj.held, a.dj.pausedBy = nil, false, false
	if p != nil {
		p.Close()
	}
	a.pl.Duck(1, 280*time.Millisecond)
	a.redraw()
}

// djInterlude keeps the queue where it is when a track ends while the
// host still speaks: the next song waits for the voice. A voice for
// another track than the one next in line says nothing more.
func (a *app) djInterlude(next *netease.Track) bool {
	if next == nil || a.dj.player == nil {
		return false
	}
	if a.dj.at != next.ID {
		a.djCancel()
		return false
	}
	a.dj.held = true
	return true
}

// djChanged follows the player: it makes the next line, times it, lets
// the voice pause and resume with the music, and stops it when the
// listener goes elsewhere.
func (a *app) djChanged() {
	if a.pl == nil {
		return
	}
	if a.dj.player != nil {
		cur := a.pl.Current()
		if cur == nil || cur.ID != a.dj.track {
			a.djCancel()
			return
		}
		switch {
		case !a.pl.Playing() && !a.dj.pausedBy && a.dj.player.Playing():
			a.dj.player.Pause()
			a.dj.pausedBy = true
		case a.pl.Playing() && a.dj.pausedBy:
			a.dj.player.Play()
			a.dj.pausedBy = false
		}
		a.dj.player.SetVolume(a.volume * a.volume)
	}
	a.djPrefetch()
	a.djSchedule()
}

// djSpeaking reports whether the host speaks now.
func (a *app) djSpeaking() bool { return a.dj.player != nil && a.dj.next != nil }

// djToggle turns the host on or off, as the FM page's button does.
func (a *app) djToggle() {
	a.settings.DJ = !a.settings.DJ
	a.settings.save()
	if !a.settings.DJ {
		a.djCancel()
	}
	a.djSchedule()
	a.redraw()
}

// djToggleButton is the FM page's own switch for the host: a capsule of
// the page's glass, lit when the host speaks.
func (a *app) djToggleButton(c *ui.Context, p palette) ui.Element {
	on := a.settings.DJ
	key := "dj-off"
	if on {
		key = "dj-on"
	}
	b := ui.ButtonBase(c.Key(key)).Height(30).PaddingX(12).Gap(7).Radius(999).
		AlignItems(ui.Center).TextColor(p.text).Label("DJ 讲解")
	fill := p.hover
	if on {
		fill = a.songAccent.Alpha(0.16)
	}
	if b.Hovered() {
		fill = fill.Mix(p.text, 0.08)
	}
	b.Background(b.AnimateColor("bg", fill, ui.Animation{Duration: 160 * time.Millisecond}))
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.95
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() {
		color := p.muted
		if on {
			color = a.songAccent
		}
		ui.Icon(c, iconMic).Size(13, 13).TextColor(color)
		ui.Text(c, "DJ 讲解").FontSize(12).FontWeight(600)
		if on {
			ui.Box(c).Size(5, 5).Radius(999).Background(a.songAccent)
		}
	})
	return b
}

// djLive is what the FM card shows while the host speaks: a microphone,
// a few bars moving with the voice, and the song it introduces.
func (a *app) djLive(c *ui.Context) {
	if !a.djSpeaking() {
		return
	}
	title := a.dj.next.Name
	ui.Row(c).Margin(2, 0, 4, 0).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconMic).Size(13, 13).TextColor(white.Alpha(0.92))
		ui.Text(c, "DJ 讲解中").FontSize(12.5).FontWeight(700).TextColor(white.Alpha(0.94)).SingleLine()
		ui.Box(c).Size(18, 12).Draw(func(dr *ui.Painter, r ui.Rect) {
			now := float64(dr.Now().UnixMilli()) / 1000
			for i := 0; i < 3; i++ {
				phase := now*6 + float64(i)*1.7
				h := 3 + 7*(0.5+0.5*math.Sin(phase))
				x := r.X + float32(i)*6
				dr.Fill(ui.Rect{X: x, Y: r.Y + r.H/2 - float32(h)/2, W: 3, H: float32(h)},
					white.Alpha(0.85-float32(i)*0.15), 1.5)
			}
			dr.AnimationFrameRate(30)
		})
		ui.Text(c, "接下来《"+title+"》").FontSize(12.5).TextColor(white.Alpha(0.62)).SingleLine()
	})
}
