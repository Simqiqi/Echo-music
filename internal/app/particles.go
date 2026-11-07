package app

import (
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
)

// The particle record is the cover of the song playing shattered into
// thousands of specks, standing square in the middle of the pane: a card
// of its own colors floating in the dark, its edges coming apart into it
// so that there is no frame to say where it ends. It leans only after the
// pointer, as a card held up would: left square, it is the pointer that
// gives it depth.

// recordSource paints the particle record: the cover of the song in specks,
// a speck to a cell, each where the cover has its color seen through the
// turn of a small perspective, so that the specks together make the cover
// out as the cover, shot through with the dark between them. The cover
// stands in the middle of the pane, smaller than the pane, and past its
// edge the specks thin out into the dark until none is left: no line, and
// no frame, marks where the cover stops.
const recordSource = `package main

var Tint vec4
var Cen vec2
var Ext float
var Yaw float
var Pitch float
var Push float
var Shape float

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func hash2(p vec2) vec2 {
	return vec2(hash(p), hash(p + vec2(17.3, 9.1)))
}

func Fragment(dst vec4, pos vec2) vec4 {
	// The cover stands where the pane puts it, its middle at Cen and its
	// side Ext: smaller than the pane, so that it floats there rather than
	// fills it, and leaving the foot of the pane for the line being sung.
	// The record comes a little forward at every strike of the drum: it
	// grows toward the eye, its edges running out a breath, and settles
	// back as the strike dies. Only a breath, so that the cover holds.
	S := Ext * (1.0 + Push*0.035)
	cen := Cen
	// A speck every few DIPs, so that the specks make the cover out as the
	// cover itself, and it reads the same however big the pane is.
	cell := 4.0
	G := floor(S / cell)
	cell = S / G

	// The dark it hangs in, with the cover's color glowing faintly at the
	// middle of it.
	q := (pos - cen) / (S * 0.5)
	col := vec3(0.016, 0.015, 0.022)
	col += Tint.rgb * 0.11 * exp(-dot(q, q) * 2.2)

	// The cover hangs askew, turned a little about its own up and its own
	// across, and is seen from near enough that the turn has depth to it.
	// The pane stands square to the eye, so each cell of it is cast back
	// onto the cover's own plane, and takes the color the cover has there:
	// the near side of it larger and brighter than the far.
	cy := cos(Yaw)
	sy := sin(Yaw)
	cp := cos(Pitch)
	sp := sin(Pitch)
	fz := 2.4 - 0.35*Push
	X := q.x
	Y := -q.y
	nx := sy
	ny := -cy * sp
	nz := cy * cp
	t := -(nz * fz) / (nx*X - ny*Y - nz*fz)
	// The point of the cover's plane the eye is looking through this cell
	// at, in the plane's own axes: up the cover is up the pane, so that
	// the cover stands as it stands and does not hang on its head.
	px := t * X
	py := t * Y
	pz := fz * (1.0 - t)
	x1 := px
	y1 := py*cp + pz*sp
	z1 := -py*sp + pz*cp
	u := x1*cy - z1*sy
	v := y1
	near := fz / (fz - pz)

	// How far out this cell stands in the cover's own units: none at its
	// middle, one at its edge, and past that it is past the edge.
	away := max(abs(u), abs(v))
	mask := 1.0 - smoothstep(0.82, 1.26, away)
	uvc := vec2(clamp(u*0.5+0.5, 0.0, 1.0), clamp(0.5-v*0.5, 0.0, 1.0))
	cH := image0Sample(uvc * image0Size()).rgb

	// The speck of this cell, a hair off the middle of it, so that the
	// specks do not line up as a grid does.
	id := floor((pos - cen + vec2(S, S)*0.5) / cell)
	h := hash2(id + vec2(0.37, 0.71))
	hp := cen - vec2(S, S)*0.5 + (id + 0.5 + (h-0.5)*0.22) * cell
	// Past the edge the specks thin out, a speck here, a gap there, until
	// none is left: the cover comes apart into the dark rather than ending
	// at a line, and the dark between it and the pane is the pane's own.
	vis := step(hash(id*1.7 + vec2(3.1, 8.4)), mask)
	dx := pos.x - hp.x
	dy := pos.y - hp.y
	rad := cell * (0.46 + 0.10*h.y) * near
	// The speck itself, as the listener asks: round, square, or a star of
	// four points. Each keeps to the cell, so that the cover reads as the
	// cover whichever they are.
	ax := abs(dx)
	ay := abs(dy)
	mRound := 1.0 - smoothstep(rad*0.30, rad, length(vec2(dx, dy)))
	mSquare := 1.0 - smoothstep(rad*0.58, rad*0.88, max(ax, ay))
	mStar := 1.0 - smoothstep(rad*0.62, rad*0.92, (ax+ay)*0.78 + max(ax, ay)*0.22)
	sq := clamp(Shape, 0.0, 1.0) * (1.0 - clamp(Shape-1.0, 0.0, 1.0))
	st := clamp(Shape-1.0, 0.0, 1.0)
	m := (mRound*(1.0-sq-st) + mSquare*sq + mStar*st) * vis
	col += cH * m * (0.5 + 0.5*mask) * (1.0 + 0.40*(near-1.0)) * (1.0 + 0.22*Push)
	return vec4(col, 1)
}
`

var recordShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(recordSource) })

// How far the pointer leans the record from square: a card held up, turning
// after the hand, never so far that the cover stops reading as a cover. The
// page opens with the pointer asking nothing of it, so the cover stands
// straight there until the pointer moves.
const (
	particleLeanYaw   = 0.34 // 19° either way about its own up
	particleLeanPitch = 0.22 // 13° either way about its own across
)

// pulseState is the song playing as the record feels it: the bass of it,
// smoothed, and the strike of the drum as a leap of that bass, which brings
// the record a breath toward the eye and dies away after.
type pulseState struct {
	lv   barState
	prev float32 // the bass as the last frame held it
	kick float32 // the strike of the drum, up at a leap, dying after
	at   time.Time
}

// step takes the player's spectrum and returns how far the record comes
// forward, from 0 to 1: a little with the bass, and more with the strike of
// the drum, which swallows back into silence as the strike dies.
func (p *pulseState) step(now time.Time, lv [player.Bands]float32, live bool) float32 {
	p.lv.step(now, lv, live)
	dt := float32(min(max(now.Sub(p.at).Seconds(), 0.001), 0.05))
	p.at = now
	var raw float32
	for i := 0; i < 6; i++ {
		raw += lv[i]
	}
	raw /= 6
	if live {
		if on := (raw - p.prev) * 2.2; on > p.kick {
			p.kick = on
		}
	}
	p.prev = raw
	p.kick = min(1, max(0, p.kick-dt*3.2))
	if !live && p.kick < 0.01 {
		return 0
	}
	return min(1, 0.3*p.lv.group(0, 6)+0.7*p.kick)
}

// tiltState is how the record leans after the pointer over the particles
// page: where it leans now, where the pointer asks it to, and when it was
// last drawn.
type tiltState struct {
	x, y   float32 // now, from -1 to 1 either way
	tx, ty float32 // asked for
	at     time.Time
}

// step brings the leaning a frame nearer what the pointer asks, and says
// whether it is still on its way.
func (t *tiltState) step(now time.Time) bool {
	dt := float32(min(max(now.Sub(t.at).Seconds(), 0.001), 0.05))
	t.at = now
	k := 1 - float32(math.Exp(float64(-dt/0.16)))
	t.x += (t.tx - t.x) * k
	t.y += (t.ty - t.y) * k
	return absFloat(t.tx-t.x) > 0.002 || absFloat(t.ty-t.y) > 0.002
}

func absFloat(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// drawParticles paints the particle record of r: the cover art in specks
// about cen, ext across, turned yaw about its own up and pitch about its
// across, with tint as the color of its dark. Square to the eye at yaw and
// pitch of nought, it leans only as it is asked to; push, from nought to
// one, brings it a breath toward the eye.
func drawParticles(p *ui.Painter, r ui.Rect, art *ui.Bitmap, tint ui.Color, cen ui.Point, ext, yaw, pitch, push float32, shape int) {
	recordShader().Draw(p, r, shader.Options{
		Uniforms: map[string]any{
			"Tint": tint, "Cen": [2]float32{cen.X, cen.Y}, "Ext": ext,
			"Yaw": yaw, "Pitch": pitch, "Push": push, "Shape": float32(shape),
		},
		Images: [4]*ui.Bitmap{art},
	})
}

// sparkSource paints the sparks the treble throws off the wall: little
// lights rising off the posters and dying as they go, the more of them,
// and the brighter, the brighter the treble.
const sparkSource = `package main

var Time float
var Treble float

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func hash2(p vec2) vec2 {
	return vec2(hash(p), hash(p + vec2(17.3, 9.1)))
}

func Fragment(dst vec4, pos vec2) vec4 {
	cell := 30.0
	id := floor(pos / cell)
	h := hash2(id)
	lot := hash(id + vec2(7.7, 3.1))
	// Each spark rises on a cycle of its own; the treble wakes more of
	// them, those whose lot falls under it.
	T := 2.0 + 2.4*lot
	ph := fract(Time/T + h.x)
	on := step(lot, Treble*0.9 + 0.04)
	p := (id + vec2(0.5 + 0.35*sin(Time*2.6 + h.y*6.2), 0.5)) * cell
	p.y += ph * cell * (2.2 + 2.4*h.y)
	d := length(pos - p)
	rad := 1.1 + 1.9*h.y + 1.4*Treble
	m := (1.0 - smoothstep(rad*0.3, rad, d)) * (1.0-ph) * (1.0-ph) * on
	col := mix(vec3(1.0, 0.95, 0.85), vec3(0.75, 0.88, 1.0), h.y)
	return vec4(col*m*(0.55+0.8*Treble), m)
}
`

var sparkShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(sparkSource) })

// drawSparks paints the sparks of the treble over r, t seconds into the
// song, transparent where none fly.
func drawSparks(p *ui.Painter, r ui.Rect, t, treble float32) {
	sparkShader().Draw(p, r, shader.Options{
		Uniforms: map[string]any{"Time": t, "Treble": min(treble, 1)},
	})
}

// particlePane fills Now Playing with the cover of the song playing, come
// apart into specks, the line being sung along its foot, and the song's
// own bar under that.
func (a *app) particlePane(c *ui.Context, tr *netease.Track, art *artwork, accent ui.Color, w, h float32, pointerIn bool) {
	top := topBar(c) + npTabsHeight
	// The band under the record is the line's only while there is a line to
	// come, or a song without words to say so: no word yet, no band, and
	// the record keeps the room.
	lines, loaded := a.pl.Lyrics()
	lyrics := !a.settings.ParticleLyricsOff && (len(lines) > 0 || loaded)
	mid := a.settings.ParticleLyricMid
	a.recAt, a.recLyric = particleLayout(top, w, h, lyrics, mid)
	// The record leans after the pointer as a card held up would, its edges
	// riding the mouse; with the pointer gone from the window it comes back
	// to square.
	st := &a.recTilt
	if !pointerIn {
		st.tx, st.ty = 0, 0
	}
	if st.step(c.Now()) {
		c.After(16 * time.Millisecond)
	}
	tabs, _, bareFoot, line := a.npBareParts()
	if !bareFoot {
		a.wall3Foot(c, pointerIn)
	}
	a.particleRecord(c, tr, art, a.recAt)
	if !line {
		a.particleLyrics(c, accent, w, a.recLyric)
	}
	if !bareFoot {
		a.wall3DBar(c, tr, accent, false)
	}
	reveal(a.npTabsRow(c, w), pointerIn && !tabs).Children(func() { a.paneTabs(c) })
}

// particleFoot is what the bar at the foot of the pane takes, and what the
// line being sung over it takes when it is shown.
const (
	particleFoot  = 118
	particleLyric = 84
)

// particleLayout is where the record stands in a pane of w×h whose content
// starts at top, and the band the line being sung keeps. The record takes
// the middle of the room the pane leaves it, but no more than the pane's
// own height allows. The line stands under the record, in a band of its
// own, or across the middle of the pane over the record, which keeps its
// room either way and is seen through the line.
func particleLayout(top, w, h float32, lyrics, mid bool) (rec ui.Rect, line ui.Rect) {
	var band float32
	if lyrics {
		band = particleLyric
	}
	room := h - top - particleFoot
	if lyrics && !mid {
		room -= band
	}
	area := max(room, 180)
	ext := min(min(w*0.86, area), h*0.56)
	// The line stands under the record, in the band the record leaves it,
	// or across the middle of the room the pane has, over the record; a
	// pane too low for that keeps the line as high as its room allows.
	lineY := top + area
	if mid {
		lineY = max(top, top+room/2-band/2)
	}
	return ui.Rect{X: w/2 - ext/2, Y: top + (area-ext)/2, W: ext, H: ext},
		ui.Rect{X: 0, Y: lineY, W: w, H: band}
}

// particleRecord fills the pane with the cover of the song playing, in
// specks, hung askew where at puts it: a record of the song's own colors
// floating in a dark of them, with no frame between the two.
func (a *app) particleRecord(c *ui.Context, tr *netease.Track, art *artwork, at ui.Rect) {
	area := ui.Box(c.Key("particles")).Absolute().Top(0).Left(0).Right(0).Bottom(0)
	pl := a.pl
	area.Children(func() {
		tint := ui.RGB(110, 110, 140)
		if art != nil {
			tint = art.tint
		}
		bmp := bitmap(a.art.get(netease.Sized(tr.Album.PicURL, 512), false))
		box := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0)
		if bmp == nil {
			box.Background(ui.RGB(10, 10, 16))
		}
		cen := ui.Point{X: at.X + at.W/2, Y: at.Y + at.H/2}
		box.Draw(func(p *ui.Painter, r ui.Rect) {
			if bmp == nil {
				// The record stands still once it is there; while its
				// cover loads, the pane keeps drawing to take it.
				p.AnimationFrameRate(30)
				return
			}
			lv, live := pl.Levels()
			push := a.pulse.step(p.Now(), lv, live)
			drawParticles(p, r, bmp, tint, cen, at.W,
				particleLeanYaw*a.recTilt.x, particleLeanPitch*a.recTilt.y, push, a.settings.ParticleShape)
			// The record beats while the song plays: it asks for frames of
			// its own until the song stops and its last strike dies away.
			if live || push > 0.002 {
				p.AnimationFrameRate(60)
			}
		})
	})
}

// particleLyrics shows the line being sung along the foot of the record,
// set as the stage sets its lines: the words springing into their places
// as they are sung and drifting after, the line coming as the last goes.
// A song without words says so instead.
func (a *app) particleLyrics(c *ui.Context, accent ui.Color, w float32, at ui.Rect) {
	if at.H <= 0 {
		return
	}
	pl := a.pl
	lines, loaded := pl.Lyrics()
	band := ui.Box(c.Key("particle-lyrics")).Absolute().Left(0).Right(0).Top(at.Y).Height(at.H).PassThrough()
	band.Children(func() {
		if len(lines) == 0 {
			if !loaded {
				return
			}
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
				ui.Text(c, "纯音乐，请欣赏").FontSize(18).FontWeight(700).TextColor(white.Alpha(0.4))
			})
			return
		}
		pos := pl.Position()
		cur := lyrics.Index(lines, pos)
		if pl.Playing() {
			stageNextFrame(c, lines, cur, pos, 80*time.Millisecond)
		}
		if cur < 0 {
			return
		}
		f := min(32, max(20, w*0.024))
		// Over the record the words ask a shadow to read against; it
		// hangs on each word, so that no edge of it is ever seen.
		a.particleLine(c, &lines[cur], pos, f, w, at.H, accent, a.settings.ParticleLyricMid)
	})
}

// particleLine sets the words of a line across the middle of the band under
// the record: the words of the stage's classic visualizer, sprung into
// their places and glowing as they are sung, but kept together and to the
// middle, where the band is narrow and a line of the stage would scatter.
func (a *app) particleLine(c *ui.Context, l *lyrics.Line, pos time.Duration, f, w, h float32, accent ui.Color, shadow bool) {
	shapes := stageShapes(c, l, f)
	ui.Row(c.Key(l.At)).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(0, w*0.06).
		Wrap().Justify(ui.Center).AlignContent(ui.Center).AlignItems(ui.Center).
		PassThrough().Children(func() {
		var shade float32
		if shadow {
			shade = 1
		}
		for i, word := range l.Words {
			a.stageWord(c.Key(i), word, shapes[i], pos, f, accent, shade)
		}
	})
}

// particleLyricPlace is the choice of where the line being sung stands on
// the particles page: under the record, or across the middle of the pane
// over it. It shows only while the line is shown, for a choice of where
// nothing stands asks nothing.
func (a *app) particleLyricPlace(c *ui.Context, accent ui.Color) {
	if a.settings.ParticleLyricsOff {
		return
	}
	mid := a.settings.ParticleLyricMid
	tr := ui.Row(c.Key("particle-lyrics-place")).Height(npTabsHeight).Padding(0, 9, 0, 12).Gap(7).
		Radius(999).AlignItems(ui.Center).Background(white.Alpha(0.08)).Label("歌词位置")
	tr.Children(func() {
		ui.Text(c, "位置").FontSize(13).FontWeight(600).TextColor(white.Alpha(0.75)).SingleLine()
		ui.Row(c).Gap(2).Padding(2).Radius(999).Background(white.Alpha(0.06)).Children(func() {
			a.particlePlaceChip(c, "下方", !mid, false, accent)
			a.particlePlaceChip(c, "中间", mid, true, accent)
		})
	})
}

// particlePlaceChip is one of the two places the line may stand, lit with
// the song's color when it is the one chosen.
func (a *app) particlePlaceChip(c *ui.Context, name string, on, mid bool, accent ui.Color) {
	chip := ui.Row(c.Key(name)).Height(npTabsHeight-12).Padding(0, 8, 0, 8).Radius(999).
		AlignItems(ui.Center).Label("歌词" + name)
	if on {
		chip.Background(accent.Alpha(0.92))
	} else if chip.Hovered() {
		chip.Background(white.Alpha(0.14))
	}
	fg := white.Alpha(0.7)
	if on {
		fg = white
	}
	ui.Text(c, name).FontSize(12).FontWeight(600).TextColor(fg).SingleLine()
	if chip.Clicked() && !on {
		a.settings.ParticleLyricMid = mid
		a.settings.save()
	}
}

// particleShapePicker is the choice of what the specks of the particles
// page look like: round, square, or a star of four points.
func (a *app) particleShapePicker(c *ui.Context, accent ui.Color) {
	names := []string{"圆点", "方块", "星形"}
	tr := ui.Row(c.Key("particle-shape")).Height(npTabsHeight).Padding(0, 9, 0, 12).Gap(7).
		Radius(999).AlignItems(ui.Center).Background(white.Alpha(0.08)).Label("粒子形态")
	tr.Children(func() {
		ui.Text(c, "形态").FontSize(13).FontWeight(600).TextColor(white.Alpha(0.75)).SingleLine()
		ui.Row(c).Gap(2).Padding(2).Radius(999).Background(white.Alpha(0.06)).Children(func() {
			for i, name := range names {
				on := a.settings.ParticleShape == i
				chip := ui.Row(c.Key(name)).Height(npTabsHeight-12).Padding(0, 8).Radius(999).
					AlignItems(ui.Center).Label("粒子" + name)
				if on {
					chip.Background(accent.Alpha(0.92))
				} else if chip.Hovered() {
					chip.Background(white.Alpha(0.14))
				}
				fg := white.Alpha(0.7)
				if on {
					fg = white
				}
				ui.Text(c, name).FontSize(12).FontWeight(600).TextColor(fg).SingleLine()
				if chip.Clicked() && !on {
					a.settings.ParticleShape = i
					a.settings.save()
				}
			}
		})
	})
}

// particleLyricSwitch is the switch in the corner of the particles page
// that keeps the line being sung there, or takes it away.
func (a *app) particleLyricSwitch(c *ui.Context, accent ui.Color) {
	on := !a.settings.ParticleLyricsOff
	tr := ui.Row(c.Key("particle-lyrics-switch")).Height(npTabsHeight).Padding(0, 10, 0, 12).Gap(9).
		Radius(999).AlignItems(ui.Center).Label("粒子页歌词")
	fill := white.Alpha(0.08)
	if tr.Hovered() {
		fill = white.Alpha(0.18)
	}
	tr.Background(tr.AnimateColor("bg", fill, ui.Animation{Duration: 120 * time.Millisecond}))
	track, dot, left := white.Alpha(0.16), white.Alpha(0.6), float32(3)
	if on {
		track, dot, left = accent.Alpha(0.9), white, 13
	}
	tr.Children(func() {
		ui.Text(c, "歌词").FontSize(13).FontWeight(600).TextColor(white.Alpha(0.75)).SingleLine()
		ui.Box(c.Key("track")).Size(26, 15).Radius(8).Background(track).Children(func() {
			ui.Box(c).Absolute().Left(left).Top(2.5).Size(10, 10).Radius(5).Background(dot)
		})
	})
	if tr.Clicked() {
		a.settings.ParticleLyricsOff = on
		a.settings.save()
	}
}

// particleOptionsMenu folds the particles page's corner controls into
// one small button: what the specks look like, whether the line being
// sung shows, and where it stands. The record keeps the width the three
// pills used to take.
func (a *app) particleOptionsMenu(c *ui.Context, accent ui.Color) {
	b := ui.ButtonBase(c.Key("particle-options")).Height(npTabsHeight).PaddingX(12).
		Radius(999).Gap(5).AlignItems(ui.Center).Label("粒子页选项").Tooltip("粒子页选项")
	bg, fg := white.Alpha(0.08), white.Alpha(0.8)
	if b.Hovered() {
		bg, fg = white.Alpha(0.18), white
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	b.Children(func() {
		ui.Icon(c, iconMore).Size(15, 15).TextColor(fg)
		ui.Text(c, "选项").FontSize(12).FontWeight(600).TextColor(fg).SingleLine()
		ui.Icon(c, iconDown).Size(11, 11).TextColor(accent)
	})
	b.Menu(func(m *ui.Menu) {
		for i, name := range []string{"圆点", "方块", "星形"} {
			if m.Item(name).Checked(a.settings.ParticleShape == i).Chosen() && a.settings.ParticleShape != i {
				a.settings.ParticleShape = i
				a.settings.save()
			}
		}
		m.Separator()
		if m.Item("显示歌词").Checked(!a.settings.ParticleLyricsOff).Chosen() {
			a.settings.ParticleLyricsOff = !a.settings.ParticleLyricsOff
			a.settings.save()
		}
		if !a.settings.ParticleLyricsOff {
			if m.Item("歌词在下方").Checked(!a.settings.ParticleLyricMid).Chosen() {
				a.settings.ParticleLyricMid = false
				a.settings.save()
			}
			if m.Item("歌词在中间").Checked(a.settings.ParticleLyricMid).Chosen() {
				a.settings.ParticleLyricMid = true
				a.settings.save()
			}
		}
	})
}
