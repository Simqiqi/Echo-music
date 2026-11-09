package app

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
)

// The three visual panes are painted on the GPU, as the particle record
// and the wallpaper's flow are: a shader over the whole view, so the light
// and the color move as one field rather than as so many drawn shapes.
// They share this file's frame and stage; each keeps its own shader in a
// file of its own, so one that does not earn its place can simply go.

// glslCommon is the noise the three shaders share. It is the same value
// noise and four octaves the wallpaper's flow uses, already known to the
// renderer, prefixed to each shader's own uniforms and Fragment.
const glslCommon = `package main

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func hash2(p vec2) vec2 {
	return vec2(hash(p), hash(p + vec2(17.3, 9.1)))
}

func noise(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * (3.0 - 2.0*f)
	a := hash(i)
	b := hash(i + vec2(1.0, 0.0))
	c := hash(i + vec2(0.0, 1.0))
	d := hash(i + vec2(1.0, 1.0))
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}

func fbm(p vec2) float {
	v := 0.0
	amp := 0.5
	for i := 0; i < 4; i++ {
		v += amp * noise(p)
		p = p*2.07 + vec2(1.7, 9.2)
		amp *= 0.5
	}
	return v
}
`

// visualFrame is one frame's worth of what the shaders need, read in the
// build and held still while the painter runs: the clock, the smoothed
// spectrum, the pointer's disturbance, and where the song's colors sit.
type visualFrame struct {
	t, bass, treble, pulse, swirl float32
	ext, progress                 float32
	cen, pointer                  [2]float32
	tint, accent                  ui.Color
	cover                         *ui.Bitmap
}

// visualState keeps the clock running between builds. It slows rather
// than stops on a pause, so the light still breathes when the music
// rests, and remembers how far the pointer has stirred the field.
type visualState struct {
	at                  time.Time
	t                   float64
	speed, bass, treble float32
	pulse, prev, swirl  float32
	px, py              float32
	pointerSeen         bool
}

func (s *visualState) step(now time.Time, lv [player.Bands]float32, live, playing, pointerIn bool, x, y float32) {
	dt := float32(min(max(now.Sub(s.at).Seconds(), 0.001), 0.05))
	if s.at.IsZero() {
		dt = 1.0 / 60
	}
	s.at = now
	target := float32(0.28)
	if playing {
		target = 1
	}
	s.speed += (target - s.speed) * (1 - float32(math.Exp(float64(-dt/0.35))))
	s.t += float64(dt * s.speed)

	var bass, treble float32
	low := min(6, player.Bands)
	for i := 0; i < low; i++ {
		bass += lv[i]
	}
	bass /= float32(max(1, low))
	for i := low; i < player.Bands; i++ {
		treble += lv[i]
	}
	treble /= float32(max(1, player.Bands-low))
	k := 1 - float32(math.Exp(float64(-dt/0.09)))
	s.bass += (bass - s.bass) * k
	s.treble += (treble - s.treble) * k
	if live && bass-s.prev > 0.06 {
		s.pulse = 1
	}
	s.prev = bass
	s.pulse = max(0, s.pulse-dt*2.4)

	if pointerIn && !s.pointerSeen {
		s.px, s.py, s.pointerSeen = x, y, true
	}
	if pointerIn {
		d := float32(math.Hypot(float64(x-s.px), float64(y-s.py)))
		s.swirl = min(1, s.swirl+d*0.006)
	} else {
		s.pointerSeen = false
	}
	floor := float32(0)
	if pointerIn {
		floor = 0.22
	}
	s.swirl = max(floor, s.swirl-dt*0.8)
	s.px, s.py = x, y
}

// visualFrame builds the frame the shaders paint, from the clock and the
// song's cover and colors: what moves with the music, and where the field
// sits. Called from the draw, so the show keeps moving when the pointer is
// still and no rebuild runs.
func (a *app) visualFrame(w, h float32, tint, accent ui.Color, cover *ui.Bitmap) visualFrame {
	f := visualFrame{
		t: float32(a.visual.t), bass: a.visual.bass, treble: a.visual.treble,
		pulse: a.visual.pulse, swirl: a.visual.swirl,
		ext:     min(w, h) * 0.46,
		cen:     [2]float32{w / 2, h * 0.48},
		pointer: [2]float32{a.npPokeX, a.npPokeY},
		tint:    tint, accent: accent, cover: cover,
	}
	if d := a.pl.Duration(); d > 0 {
		f.progress = min(1, float32(a.pl.Position())/float32(d))
	}
	return f
}

// visualPane is the stage the three shaders share: a full-view field, the
// words of the song held over it, and the row of tabs at the top. paint
// draws the field itself; the rest is laid out once, in one place.
func (a *app) visualPane(c *ui.Context, tr *netease.Track, art *artwork, accent ui.Color,
	w, h float32, pointerIn bool, paint func(*ui.Painter, ui.Rect, visualFrame)) {

	tint := ui.RGB(120, 120, 140)
	if art != nil && !art.failed {
		tint = art.tint
	}
	var cover *ui.Bitmap
	if tr.Album.PicURL != "" {
		cover = bitmap(a.art.get(netease.Sized(tr.Album.PicURL, 512), false))
	}
	// The clock steps in the draw, not the build: a Painter's
	// AnimationFrameRate only repaints — it does not rebuild the view — so
	// stepping the clock here is what keeps the field alive the moment the
	// pointer rests, when nothing else asks for a build. The repaint alone
	// then carries the motion, where stepping it in the build left the field
	// frozen between two moves.
	field := ui.Box(c.Key("visual-field")).Absolute().Top(0).Left(0).Right(0).Bottom(0)
	field.Draw(func(p *ui.Painter, r ui.Rect) {
		lv, live := a.pl.Levels()
		a.visual.step(p.Now(), lv, live, a.pl.Playing(), pointerIn, a.npPokeX, a.npPokeY)
		paint(p, r, a.visualFrame(w, h, tint, accent, cover))
		// The field is never quite still: even at rest its colors drift.
		p.AnimationFrameRate(45)
	})

	tabs, _, _, _ := a.npBareParts()
	reveal(a.npTabsRow(c, w), pointerIn && !tabs).Children(func() { a.paneTabs(c) })
	a.visualWords(c, tr, w, h)
}

// visualWords holds the song's name and the line being sung over the
// field, low and quiet: the light is the show, the words are its caption.
func (a *app) visualWords(c *ui.Context, tr *netease.Track, w, h float32) {
	names := make([]string, 0, len(tr.Artists))
	for _, ar := range tr.Artists {
		names = append(names, ar.Name)
	}
	line := ""
	if lines, _ := a.pl.Lyrics(); len(lines) > 0 {
		if at := lyrics.Index(lines, a.pl.Position()); at >= 0 && at < len(lines) {
			line = strings.TrimSpace(lines[at].Text)
		}
	}
	ui.Column(c.Key("visual-words")).Absolute().Left(0).Right(0).Bottom(38).
		Height(96).Gap(7).Center().PassThrough().Children(func() {
		if line != "" {
			ui.Text(c, line).FontSize(17).FontWeight(600).TextColor(white.Alpha(0.82)).
				SingleLine().MaxLines(1).Margin(0, 30, 0, 30)
		}
		ui.Text(c, tr.Name).FontSize(13).FontWeight(700).TextColor(white.Alpha(0.55)).SingleLine()
		if len(names) > 0 {
			ui.Text(c, strings.Join(names, " / ")).FontSize(11).TextColor(white.Alpha(0.36)).SingleLine()
		}
	})
}
