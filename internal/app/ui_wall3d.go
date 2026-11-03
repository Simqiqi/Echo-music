package app

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
)

// The 3D wall stands the queue's covers on a cylinder turning about the
// listener: the song playing faces front, its neighbours turn away into
// the dark either side, and the wall spins, dragged or swiped, to bring
// another to the front, which a click plays. A poster further off is
// smaller, dimmer and lower, as it would be seen, so that the rows curve
// away with the wall.

// A poster's side at the front, the gap beside it, and so how far apart
// two columns stand there.
const (
	wall3Tile = 168
	wall3Gap  = 22
	// How fast it turns of itself, a column's width every six seconds,
	// for it never stands still: the hand turns it as it holds it, and
	// it goes on from where the hand leaves it.
	wall3AutoSpeed = float32(wall3Tile+wall3Gap) / 6
)

// wall3Press is the pointer as it presses the wall, dragging it round.
type wall3Press struct {
	down      bool
	x0, y0    float32 // where it pressed
	t0        time.Time
	x, y      float32 // where it last was
	moved     bool
	lastX     float32
	lastT     time.Time
	velocityX float32
}

// wall3State is the wall as the pointer spins it.
type wall3State struct {
	off float32 // where the wall shows, in DIP along its turn
	to  float32 // where it goes as it glides to rest
	at  time.Time
	// The pointer over the wall, where it last was.
	px, py float32
	pover  bool
	press  wall3Press
	// The song the wall last brought to the front, 0 until it shows.
	track int64
	// How shown the song playing along the foot is: it comes out of the
	// dark as the pointer enters the pane, and back into it as it goes.
	ui float32
	// The spectrum of the song playing, smoothed, for the bass to shake
	// the wall and the treble to throw sparks off it, and the seconds
	// the sparks have run, only while the song plays.
	lv     barState
	sparkT float32
	// Where each poster showed in the last frame, the nearest last, for
	// the click and the pointer to find one under them.
	hits []wall3Hit
}

// wall3Hit is where a poster of the queue showed.
type wall3Hit struct {
	i int
	r ui.Rect
}

// wall3Geom is the wall as it stands: how its posters are laid out. The
// wall has no ends: its slots go on either way for ever, the queue
// repeats every cols of them, so that turning it comes round to the
// first poster again past the last.
type wall3Geom struct {
	cols, rows int
	pitch      float32 // between columns, and between rows, at the front
	radius     float32 // of the cylinder the posters stand on
	step       float32 // of the turn between two columns, in radians
}

// layWall3 lays n posters out in an area w×h: as many rows as stand in it,
// and columns enough that one turn of the wall holds the queue, which
// then repeats for as far as the wall goes.
func layWall3(n int, w, h float32) wall3Geom {
	pitch := float32(wall3Tile + wall3Gap)
	rows := 3
	for rows > 1 && float32(rows)*pitch > h*0.98 {
		rows--
	}
	// The camera stands inside the cylinder, near enough to its far side
	// that the front poster is a poster's size and the sides have turned
	// well away.
	radius := max(w*0.92, 320)
	step := pitch / radius
	// So that one turn of the wall holds the queue, and that the arc in
	// view shows no poster twice over, the rows standing a period apart.
	vis := int(100*math.Pi/180/step) + 1
	cols := max((n+rows-1)/rows, 2*vis+1)
	return wall3Geom{cols: cols, rows: rows, pitch: pitch, radius: radius, step: step}
}

// yaw returns the turn the wall shows at, off DIP along it.
func (g wall3Geom) yaw(off float32) float32 {
	return off / g.radius
}

// slotOf returns the queue index a slot s shows in row r, the queue
// repeating every cols of the wall's endless slots.
func (g wall3Geom) slotOf(s, r, n int) int {
	return ((r*g.cols+s)%n + n) % n
}

// rest returns where the wall comes to rest: the slot nearest off. The
// wall has no ends, so there is nothing to hold it within.
func (g wall3Geom) rest(off float32) float32 {
	return float32(math.Round(float64(off/g.pitch))) * g.pitch
}

// frontOf returns the offset that brings the queue index i to the front,
// by the turn that takes it there shortest way round. It shows in the
// row it is laid in, at slots a queue-length apart.
func (g wall3Geom) frontOf(i, n int, off float32) float32 {
	// It shows in the row it is laid in, where its slot is i mod cols,
	// and the slots that hold it stand a queue-length apart.
	base := i % g.cols
	s := base + n*int(math.Round(float64((-off/g.pitch-float32(base))/float32(n))))
	return -float32(s) * g.pitch
}

// at returns where a column stands, th round the wall from the front, in
// an area w wide: its middle across, and how near it is, 1 at the front.
func (g wall3Geom) at(th, w float32) (x, near float32) {
	sin, cos := float32(math.Sin(float64(th))), float32(math.Cos(float64(th)))
	den := 2 - cos
	return w/2 + g.radius*sin/den, 1 / den
}

// wall3Col is a column of posters as the wall stands it.
type wall3Col struct {
	c     int
	x     float32
	near  float32
	round float32 // how far it has turned from the front
}

// wall3DPane fills Now Playing with the wall, and the song playing along
// its foot.
func (a *app) wall3DPane(c *ui.Context, tr *netease.Track, accent ui.Color, w, h float32, pointerIn bool) {
	top := topBar(c) + npTabsHeight
	const foot = 150
	a.wall3D(c, top, w, max(h-top-foot, 200), accent, pointerIn)
	a.wall3DBar(c, tr, accent, true)
	reveal(npTabsRow(c, w), pointerIn).Children(func() { a.paneTabs(c) })
}

// wall3D stands the queue's covers on the wall, in an area from top down,
// w×h.
func (a *app) wall3D(c *ui.Context, top, w, h float32, accent ui.Color, pointerIn bool) {
	pl, st := a.pl, &a.wall3
	queue := pl.Queue()
	area := ui.Box(c.Key("wall3d")).Absolute().Top(top).Left(0).Right(0).Height(h).Clip()
	if len(queue) == 0 {
		return
	}
	g := layWall3(len(queue), w, h)
	playing := -1
	if order, at := pl.PlayOrder(); at >= 0 && at < len(order) {
		playing = order[at]
	}
	// Shown, the wall brings the song playing to the front; after, it
	// turns to the next as another is played.
	if tr := pl.Current(); tr != nil && tr.ID != st.track && playing >= 0 {
		st.to = g.frontOf(playing, len(queue), st.off)
		st.track = tr.ID
	}

	area.HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			st.press = wall3Press{down: true, x0: ev.X, y0: ev.Y, t0: ev.Time, x: ev.X, y: ev.Y, lastX: ev.X, lastT: ev.Time}
			st.to, st.px, st.py, st.pover = st.off, ev.X, ev.Y, true
			return true
		case ui.InputPointerMove:
			st.px, st.py, st.pover = ev.X, ev.Y, true
			if !st.press.down {
				return true // the poster under the pointer lights as it goes
			}
			if dx := ev.X - st.press.x0; math.Abs(float64(dx)) > 3 {
				st.press.moved = true
			}
			if dt := ev.Time.Sub(st.press.lastT).Seconds(); dt > 0 {
				// How fast the pointer was going, for the throw.
				v := (ev.X - st.press.lastX) / float32(dt)
				st.press.velocityX = 0.6*st.press.velocityX + 0.4*v
				st.press.lastX, st.press.lastT = ev.X, ev.Time
			}
			st.off += ev.X - st.press.x
			st.press.x, st.press.y = ev.X, ev.Y
			st.to = st.off
			return true
		case ui.InputPointerUp:
			if !st.press.down {
				return false
			}
			press := st.press
			st.press.down = false
			if !press.moved && ev.Time.Sub(press.t0) < 800*time.Millisecond {
				// A click: the nearest poster under where it let go plays.
				for k := len(st.hits) - 1; k >= 0; k-- {
					h := st.hits[k]
					if ev.X >= h.r.X && ev.X < h.r.X+h.r.W && ev.Y >= h.r.Y && ev.Y < h.r.Y+h.r.H {
						if h.i != playing && queue[h.i].Playable() {
							pl.Jump(h.i)
						}
						break
					}
				}
				return true
			}
			// Thrown, the wall glides on to the column the throw carries
			// it to; its own turn comes due again after the glide.
			st.to = g.rest(st.off + press.velocityX*0.22)
			return true
		case ui.InputScroll:
			// Two fingers turn it, a column at a time, round and round.
			st.to = g.rest(st.to + ev.DX)
			return true
		}
		return false
	})

	area.Children(func() {
		now := c.Now()
		// The song playing along the foot comes out of the dark as the
		// pointer is in the pane, and back into it as it goes, so that
		// the wall stands alone in it.
		a.wall3Foot(c, pointerIn)
		// The song's spectrum, smoothed: the bass of it drives the wall's
		// turn and shakes the wall, the treble throws sparks off it.
		lv, live := pl.Levels()
		a.wall3.lv.step(now, lv, live)
		var bass, treble float32
		for i := 0; i < 6; i++ {
			bass += a.wall3.lv.h[i]
		}
		bass /= 6
		for i := 16; i < player.Bands; i++ {
			treble += a.wall3.lv.h[i]
		}
		treble /= float32(player.Bands - 16)
		if live {
			st.sparkT += float32(min(max(now.Sub(st.at).Seconds(), 0.001), 0.05))
		}
		// The hand turns it where it will; else the wall turns of
		// itself, slowly, and never stands still: on and on it goes
		// round the endless wall, however the pointer hovers over it,
		// and it goes on from where the hand leaves it.
		if st.off != st.to {
			// To where the hand threw it, or the song playing, it
			// glides, the nearer the quicker.
			dt := float32(min(max(now.Sub(st.at).Seconds(), 0.001), 0.05))
			st.off += (st.to - st.off) * (1 - float32(math.Exp(float64(-dt/0.09))))
			if math.Abs(float64(st.to-st.off)) < 0.3 {
				st.off = st.to
			}
		} else if !st.press.down {
			// Its own turning, slow and steady, and the quicker for the
			// bass driving it.
			dt := float32(min(max(now.Sub(st.at).Seconds(), 0.001), 0.05))
			st.off += wall3AutoSpeed * (1 + bass*1.6) * dt
			st.to = st.off
		}
		// The wall stands turning while this pane shows, so a frame
		// follows this one.
		c.After(16 * time.Millisecond)
		st.at = now
		yaw := g.yaw(st.off)

		// The slots about the front, the ones that have turned furthest
		// away first, so that the near ones are painted over them. The
		// wall goes round for ever, the queue repeating along it.
		front := int(math.Round(float64(-st.off / g.pitch)))
		vis := int(100*math.Pi/180/g.step) + 1
		cols := make([]wall3Col, 0, 2*vis+1)
		for d := -vis; d <= vis; d++ {
			s := front + d
			// The turn of the slot s: the front one stands square to the
			// listener, its neighbours a step aside, round and round.
			th := yaw + float32(s)*g.step
			// Past a third of the way round, a column has turned its back.
			if math.Abs(float64(th)) > 100*math.Pi/180 {
				continue
			}
			x, near := g.at(th, w)
			if math.Abs(float64(x-w/2)) > float64(w)*0.7 {
				continue
			}
			cols = append(cols, wall3Col{c: front + d, x: x, near: near, round: th})
		}
		slices.SortFunc(cols, func(p, q wall3Col) int {
			return cmp.Compare(math.Abs(float64(q.round)), math.Abs(float64(p.round)))
		})

		hover := -1
		st.hits = st.hits[:0]
		centre := (float32(g.rows) - 1) / 2
		for _, col := range cols {
			// The bass pounds: the posters swell a little, and the wall
			// lifts with the drum and settles as it dies.
			side := wall3Tile * col.near * (1 + bass*0.06)
			half := side / 2
			// Away from the front, the wall fades into the dark, and out
			// at the sides of the area it goes before it is cut.
			dim := min(1, 0.3+0.7*col.near)
			if d := float32(math.Abs(float64(col.x-w/2))) / (w / 2); d > 0.74 {
				dim *= 1 - smoothstep(0.74, 1.02, d)
			}
			for r := range g.rows {
				i := g.slotOf(col.c, r, len(queue))
				y := h/2 + (float32(r)-centre)*g.pitch*col.near - bass*bass*7*col.near
				if dim < 0.02 || side < 12 || y < -side || y > h+side {
					continue
				}
				st.hits = append(st.hits, wall3Hit{i, ui.Rect{X: col.x - half, Y: y - half, W: side, H: side}})
				if st.pover && !st.press.down && st.px >= col.x-half && st.px < col.x+half &&
					st.py >= y-half && st.py < y+half {
					hover = i
					area.Cursor(ui.CursorPointer)
				}
				a.wall3Poster(c, &queue[i], col.x-half, y-half, side, dim,
					i == playing, i == hover, col.near, accent, col.c, r, bass)
			}
		}
		// The sparks the treble throws off the wall, over the posters,
		// through where the pointer passes, when the treble is there to
		// throw them.
		if live && treble > 0.005 {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().
				Draw(func(p *ui.Painter, r ui.Rect) {
					drawSparks(p, r, st.sparkT, treble)
					p.AnimationFrameRate(60)
				})
		}
		// The wall keeps a few frames while it rests, for the poster that
		// has the pointer over it.
		if hover >= 0 {
			c.After(120 * time.Millisecond)
		}
	})
}

// wall3Poster shows the cover of tr as a poster s wide at x,y, as near as
// near says, the song playing ringed and the one under the pointer named.
func (a *app) wall3Poster(c *ui.Context, tr *netease.Track, x, y, s, dim float32, playing, hovered bool, near float32, accent ui.Color, slot, row int, bass float32) {
	// Keyed by where it stands, not what it shows: on a wall that goes
	// round for ever, the same poster may stand in two places at once.
	post := ui.Box(c.Key(int64(slot)*16+int64(row))).Absolute().Left(x).Top(y).Size(s, s).Radius(10).Opacity(dim).
		Label(tr.Name).ContextMenu(a.songItems(tr, 0))
	if playing {
		post.Shadow(0, 16*near, 34*near, -6*near, ui.RGBA(0, 0, 0, 0.62))
	} else {
		post.Shadow(0, 8*near, 20*near, -6*near, ui.RGBA(0, 0, 0, 0.45))
	}
	if !tr.Playable() {
		post.Opacity(dim * 0.4)
	}
	radius := 10 * near
	post.Children(func() {
		a.cover(c, tr.Album.PicURL, 240, s, radius)
		if hovered {
			// Its name and artists over a shade along its foot, on the
			// cover, so that they turn with it.
			ui.Column(c.Key("caption")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).Padding(8*near).
				Gap(1).Justify(ui.End).PassThrough().TextColor(white).
				Gradient(ui.RGBA(0, 0, 0, 0.78), ui.Transparent, 0).
				Children(func() {
					ui.Text(c, tr.Name).FontSize(max(11, 12*near)).FontWeight(700).SingleLine()
					if names := tr.ArtistNames(); names != "" {
						ui.Text(c, names).FontSize(max(9.5, 10.5*near)).TextColor(white.Alpha(0.7)).SingleLine()
					}
				})
		}
		if playing {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).PassThrough().
				Border(2+bass*2.5, accent.Mix(white, 0.55))
		}
	})
}

// wall3DBar is what plays along the foot of the wall: its name, the line
// of its lyrics sung, where it is, and the transport.
// wall3Foot eases the song along the foot of a pane out of the dark as the
// pointer comes into it, and back into it as it goes, so that the pane
// stands alone while no one is about.
func (a *app) wall3Foot(c *ui.Context, pointerIn bool) {
	st := &a.wall3
	want := float32(0)
	if pointerIn {
		want = 1
	}
	if d := want - st.ui; math.Abs(float64(d)) > 0.004 {
		dt := float32(min(max(c.Now().Sub(st.at).Seconds(), 0.001), 0.05))
		st.ui += d * (1 - float32(math.Exp(float64(-dt/0.28))))
		c.After(16 * time.Millisecond)
	} else {
		st.ui = want
	}
}

func (a *app) wall3DBar(c *ui.Context, tr *netease.Track, accent ui.Color, lyric bool) {
	pl := a.pl
	bar := ui.Column(c.Key("wall3d-bar")).Absolute().Left(0).Right(0).Bottom(0).
		Padding(0, 48, 24, 48).Gap(10).Opacity(a.wall3.ui)
	if a.wall3.ui < 0.05 {
		bar = bar.PassThrough() // hidden, it keeps out of the way
	}
	bar.Children(func() {
		ui.Column(c).Gap(3).AlignItems(ui.Center).Children(func() {
			ui.Text(c, tr.Name).FontSize(17).FontWeight(700).LetterSpacing(-0.2).SingleLine()
			if names := tr.ArtistNames(); names != "" {
				ui.Text(c, names).FontSize(12.5).TextColor(white.Alpha(0.55)).SingleLine()
			}
			lines, _ := pl.Lyrics()
			if len(lines) == 0 || !lyric {
				return
			}
			pos := pl.Position()
			cur := lyrics.Index(lines, pos)
			if pl.Playing() && cur+1 < len(lines) {
				c.After(lines[cur+1].At - pos + 20*time.Millisecond)
			}
			sung := ""
			if cur >= 0 {
				sung = lines[cur].Text
			}
			ui.Text(c, sung).FontSize(12.5).TextColor(white.Alpha(0.4)).SingleLine().MaxLines(1)
		})
		a.scrubber(c.Key("wall3d-scrub"), &a.npScrub, white.Alpha(0.18), white.Alpha(0.9), 14, accent.Mix(white, 0.35))
		ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				ui.Text(c, clock(pl.Position())).FontSize(11.5).FontFeatures("tnum").TextColor(white.Alpha(0.5))
			})
			a.transport(c, white, white.Alpha(0.45), white, 22)
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Justify(ui.End).Children(func() {
				ui.Text(c, "-"+clock(pl.Duration()-pl.Position())).FontSize(11.5).FontFeatures("tnum").TextColor(white.Alpha(0.5))
			})
		})
	})
}
