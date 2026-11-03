package app

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
)

// dailyArt is the cover of the day's songs: their first cover, blurred,
// with the date set large over it.
func (a *app) dailyArt(c *ui.Context, now time.Time, size float32) {
	var art *artwork
	if len(a.daily) > 0 {
		art = a.art.get(netease.Sized(a.daily[0].Album.PicURL, 600), true)
	}
	radius := size * 0.07
	box := ui.Box(c).Size(size, size).Radius(radius).Background(ui.Hex("#3a3a40")).
		Shadow(0, 18, 40, -10, a.pal.cardShade)
	in := artIn(box, backdrop(art))
	box.Children(func() {
		artImage(c, backdrop(art), radius, in)
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).
			LinearGradient(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
				{Color: ui.RGBA(0, 0, 0, 0), At: 0.35},
				{Color: ui.RGBA(0, 0, 0, 0.4), At: 1},
			}})
		ui.Column(c).Absolute().Left(size * 0.09).Bottom(size * 0.08).TextColor(white).Children(func() {
			ui.Textf(c, "%d", now.Day()).FontSize(size * 0.44).FontWeight(300).LetterSpacing(-size * 0.02).FontFeatures("tnum").LineHeight(1)
			ui.Textf(c, "%d月 · %s", now.Month(), weekday(now)).FontSize(max(11, size*0.075)).FontWeight(600).TextColor(white.Alpha(0.85))
		})
	})
}

// dailyCard leads the home page: the day's songs, in the colors of their
// first cover, the day's date set huge and faint behind them. An aurora of
// sound crosses the card — waves that breathe on their own and rise with
// the music while it plays — and the first covers hang over it as discs of
// glass, bobbing on the crests; while the day's songs play the first one's
// sheen turns. The first picks are listed between, under the pointer the
// card leans to it and a sheen sweeps it.
func (a *app) dailyCard(c *ui.Context, now time.Time) {
	songs := a.daily
	if len(songs) == 0 {
		return
	}
	w, _ := c.Size()
	inner := w - sidebarWidth - 2*pagePad
	still := c.Preferences().ReduceMotion
	card := ui.ButtonBase(c.Key("daily")).Height(dailyHeight).Radius(dailyRadius).FocusRing(false).Label("每日推荐").TextColor(white)
	hovered := card.Hovered()
	ours, playing := a.dailyPlaying()

	card.Background(a.dailyRainbow(c.Now()).deep)
	lift := card.AnimateTo("lift", b2f(hovered), ui.Snappy)
	card.Shadow(0, 14+8*lift, 34+12*lift, -14, a.pal.cardShade)
	// A sheen sweeps across once as the pointer comes in.
	sweep := float32(0)
	if hovered && !still {
		sweep = card.AnimateTo("sweep", 1, ui.Animation{Duration: 1100 * time.Millisecond, Ease: ui.EaseInOut})
	} else {
		card.Jump("sweep", 0)
	}

	// The card leans a few degrees toward the pointer, seen in
	// perspective, as a card held to the light: it asks for frames while
	// the pointer moves over it, and eases back to flat as it leaves.
	// The discs of glass lean with it, nearer ones farther, so the sky
	// they hang in has depth.
	var px, py float32
	var over bool
	if !still {
		px, py, over = card.PointerPosition()
	}
	var rx, ry float32
	if over {
		ry = -((px/inner)*2 - 1) * 4
		rx = ((py/dailyHeight)*2 - 1) * 3
	}
	rx = card.AnimateTo("rx", rx, ui.Snappy)
	ry = card.AnimateTo("ry", ry, ui.Snappy)
	card.Transform(ui.Perspective(1200).Then(ui.RotationX(rx)).Then(ui.RotationY(ry)))
	gx := float32(0)
	gy := float32(0)
	if over {
		gx = (px/inner - 0.5) * 16
		gy = (py/dailyHeight - 0.5) * 8
	}
	gx = card.AnimateTo("gx", gx, ui.Snappy)
	gy = card.AnimateTo("gy", gy, ui.Snappy)

	// The playing disc's sheen turns while the songs play, and drifts a
	// little under the pointer.
	turn := 0.0
	switch {
	case still:
	case playing:
		turn = 1
	case hovered:
		turn = 0.12
	}
	showPicks, showDay := inner >= 1040, inner >= 900
	picksW := float32(0)
	if showPicks {
		picksW = picksWidth + 28
	}
	// The words and the picks keep clear of the discs of glass, which
	// hang at the card's right.
	textW := max(232, inner-2*dailyPadX-28-picksW-galaxyWidth)

	card.Children(func() {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
			lv, live := a.pl.Levels()
			a.dailyWave.step(p.Now(), lv, live)
			if a.dailySpin.step(p.Now(), turn) || live || (hovered && !still) {
				p.AnimationFrameRate(dailyRate)
			}
			// The rainbow runs across the card with the clock, so the page
			// asks for frames while it shows.
			if !still {
				p.AnimationFrameRate(dailyHueRate)
			}
			rb := a.dailyRainbow(p.Now())
			drawDailyBackdrop(p, r, rb, sweep)
			p.Clip(r, dailyRadius, func() {
				drawAurora(p, r, &a.dailyWave, p.Now(), rb.rich, rb.glow)
				// Deeper behind the words, over the wave, so they keep
				// their ground while the aurora shines at the right.
				p.FillGradient(r, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
					{Color: rb.deep.Alpha(0.86), At: 0}, {Color: rb.deep.Alpha(0.3), At: 0.34}, {Color: rb.deep.Alpha(0), At: 0.56},
				}}, 0)
				a.drawGalaxy(p, r, now, songs, rb.deep, rb.rich, rb.glow, float32(a.dailySpin.angle), still, playing && live, gx, gy)
			})
			// The day, set huge and faint after the words, cut off by the
			// foot, where the covers leave it room.
			if showDay {
				day := ui.Span{Text: fmt.Sprint(now.Day()), Size: 240, Weight: 800, LetterSpacing: -12, Features: "tnum", Color: white.Alpha(0.055)}
				p.Clip(r, dailyRadius, func() { p.RichText(r.X+dailyPadX+300, r.Y+r.H*0.22, 0, day) })
			}
		})
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(0, dailyPadX).Gap(28).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).MaxWidth(textW).Gap(6).Children(func() {
				ui.Textf(c, "%d月%d日 %s · %s", now.Month(), now.Day(), weekday(now), a.dailyNote()).FontSize(12).FontWeight(600).LetterSpacing(0.3).TextColor(white.Alpha(0.66)).MaxLines(1)
				ui.Text(c, "每日推荐").FontSize(32).FontWeight(800).LetterSpacing(-0.8).LineHeight(1.15)
				ui.Text(c, dailyArtists(songs)).FontSize(13).TextColor(white.Alpha(0.8)).MaxLines(1)
				ui.Row(c).Gap(10).Margin(12, 0, 0, 0).Children(func() {
					a.dailyPlayButton(c.Key("play"), songs, ours, playing)
					if a.profile == nil && pillButton(c.Key("login"), iconUser, "登录", white.Alpha(0.16), white).Clicked() {
						a.openLogin()
					}
				})
			})
			if showPicks {
				a.dailyPicks(c, songs, ours)
			}
			// Room at the right for the discs of glass.
			ui.Box(c).Width(galaxyWidth).Shrink(0)
		})
	})
	if card.Clicked() {
		a.open(route{page: pageDaily})
	}
}

// The daily card's measures; the wave and its discs are laid out by hand.
const (
	dailyHeight = 212
	dailyRadius = 24
	dailyPadX   = 32
	picksWidth  = 236 // the column of the day's first songs
	galaxyWidth = 296 // the room the discs of glass hang in, at the right
)

// dailyRate is how many frames a second the daily card draws what moves
// on it, its wave breathing and the bars on its play button: every frame
// of a 120 Hz display, it redrew the whole card 120 times a second, for
// motion that looks as smooth at half of that.
const dailyRate = 60

// dailyPlaying reports whether the day's songs are what plays, and whether
// they play now rather than paused.
func (a *app) dailyPlaying() (ours, playing bool) {
	q, cur := a.pl.Queue(), a.pl.Current()
	if cur == nil || len(q) != len(a.daily) || len(q) == 0 || q[0].ID != a.daily[0].ID || q[len(q)-1].ID != a.daily[len(q)-1].ID {
		return false, false
	}
	return true, a.pl.Playing()
}

// dailyRainbow is the daily card's colors: a rainbow that begins at a hue
// of its own — another each time the home page opens — and runs across the
// card with the clock, its colors drifting like a slow river.
type dailyRainbow struct {
	hue  float64  // where its band of hues begins, in degrees
	deep ui.Color // the dark ground the words stand on
	rich ui.Color // the color the wave of sound is drawn in
	glow ui.Color // the light the discs of glass hang in
}

// dailyHueRate is how many frames a second the card redraws to carry the
// rainbow along: its hues move slowly enough that this reads as smooth,
// and slowly enough not to cost a frame rate of its own.
const dailyHueRate = 30

// dailyHueDrift is how fast it runs, in degrees a second: a whole turn of
// hues in half a minute.
const dailyHueDrift = 12

// rollDailyRainbow takes the rainbow at another hue, each time the home
// page opens.
func (a *app) rollDailyRainbow() {
	a.dailyHue, a.dailyHued = rand.Float64()*360, true
}

// dailyRainbow is the card's colors for now: the hue it begins at, carried
// on by the clock, and the shades the words and the wave are drawn in.
func (a *app) dailyRainbow(now time.Time) dailyRainbow {
	if !a.dailyHued {
		a.rollDailyRainbow()
	}
	h := math.Mod(a.dailyHue+float64(now.UnixNano())/1e9*dailyHueDrift, 360)
	return dailyRainbow{
		hue:  h,
		deep: hsvColor(math.Mod(h+200, 360), 0.72, 0.17),
		rich: hsvColor(math.Mod(h+120, 360), 0.74, 0.5),
		glow: hsvColor(math.Mod(h+40, 360), 0.55, 1),
	}
}

// veil is the rainbow itself: a band of hues laid across the card, faint
// enough that the words read over it and bright enough to be a rainbow.
func (d dailyRainbow) veil() []ui.GradientStop {
	const n = 10
	ss := make([]ui.GradientStop, n)
	for i := range ss {
		h := math.Mod(d.hue+330*float64(i)/float64(n-1), 360)
		ss[i] = ui.GradientStop{At: float32(i) / float32(n-1), Color: hsvColor(h, 0.95, 0.68).Alpha(0.52)}
	}
	return ss
}

// drawDailyBackdrop paints the daily card in r: the rainbow across it, dark
// behind the words and running bright toward the right, and, as sweep goes
// from 0 to 1, a sheen crossing it.
func drawDailyBackdrop(p *ui.Painter, r ui.Rect, d dailyRainbow, sweep float32) {
	p.Clip(r, dailyRadius, func() {
		// The ground: deep, its hue turning with the rainbow, so white
		// reads over whatever color runs across it.
		p.FillGradient(r, ui.LinearGradient{Angle: 100, Oklab: true, Stops: []ui.GradientStop{
			{Color: d.deep, At: 0},
			{Color: hsvColor(math.Mod(d.hue+250, 360), 0.72, 0.34), At: 0.55},
			{Color: hsvColor(math.Mod(d.hue+300, 360), 0.68, 0.4), At: 1},
		}}, 0)
		// The rainbow over it.
		p.FillGradient(r, ui.LinearGradient{Angle: 100, Stops: d.veil()}, 0)
		// Light from above, as on a stage.
		p.Shadow(ui.Rect{X: r.X + r.W*0.3, Y: r.Y - 80, W: r.W * 0.4, H: 40}, 20, 0, 0, 120, 0, white.Alpha(0.08))
		if sweep > 0 && sweep < 1 {
			at := -0.25 + 1.5*sweep
			p.FillGradient(r, ui.LinearGradient{Angle: 110, Stops: []ui.GradientStop{
				{Color: white.Alpha(0), At: clamp01(at - 0.12)},
				{Color: white.Alpha(0.13 * (1 - sweep*0.5)), At: clamp01(at)},
				{Color: white.Alpha(0), At: clamp01(at + 0.12)},
			}}, 0)
		}
	})
	// A hairline of light around its edge, as on glass.
	p.Stroke(r, white.Alpha(0.1), dailyRadius, 1)
}

// auroraY is where the wave of sound stands at x: three slow swells it
// breathes with on its own, and the music itself, dealt over the card's
// bands, rising toward its right where the discs hang.
func auroraY(r ui.Rect, x float32, w *barState, t float64) float32 {
	f := clamp01((x - r.X) / r.W)
	y := r.Y + r.H*0.58
	y += 8 * float32(math.Sin(t*0.9+float64(f)*4.2))
	y += 5 * float32(math.Sin(t*1.7+float64(f)*9.1+1.7))
	y += 3 * float32(math.Sin(t*2.3+float64(f)*14.3+4.1))
	band := int(f * f * float32(player.Bands-1))
	v := w.group(max(0, band-2), min(player.Bands, band+3))
	return y - (8+40*f*f)*(0.2+v*1.6)
}

// drawAurora paints the wave of sound across r: curtains of light
// standing on its crest, a soft body built of stacked strokes, the crest
// itself, a band of glow behind it, a thin highlight of light along its
// top, and specks of stardust through the sky above it.
func drawAurora(p *ui.Painter, r ui.Rect, w *barState, now time.Time, rich, glow ui.Color) {
	t := float64(now.UnixNano()) / 1e9
	// Curtains: bands of glow hanging over the crest, swaying slowly.
	for x := r.X + 24; x < r.X+r.W-12; x += 42 {
		y := auroraY(r, x, w, t)
		h := 54 + 26*float32(math.Sin(float64(x)*0.021+t*0.35))
		p.FillGradient(ui.Rect{X: x - 14, Y: y - h, W: 28, H: h},
			ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
				{Color: glow.Alpha(0.02), At: 0},
				{Color: glow.Alpha(0.09), At: 0.72},
				{Color: glow.Alpha(0.15), At: 1},
			}}, 0)
	}
	pass := func(off, width float32, col ui.Color) {
		var pts [][2]float32
		for x := r.X - 8; x <= r.X+r.W+8; x += 14 {
			pts = append(pts, [2]float32{x, auroraY(r, x, w, t) + off})
		}
		p.StrokePolyline(pts, width, col)
	}
	pass(14, 34, rich.Alpha(0.08))
	pass(7, 22, rich.Alpha(0.12))
	pass(0, 3, rich.Alpha(0.72))
	pass(13, 1.8, glow.Alpha(0.55))
	pass(-9, 1.3, white.Alpha(0.38))
	// Stardust: a speck here and there in the sky over the wave,
	// brighter over its right, where the aurora is strongest.
	for x := r.X + 12; x < r.X+r.W-12; x += 30 {
		seed := float32(math.Sin(float64(x)*12.9898) * 43758.5453)
		f := seed - float32(math.Floor(float64(seed)))
		if f < 0.4 {
			continue
		}
		y := auroraY(r, x, w, t) - 2 - f*30
		d := 1.6 + f*1.8
		a := 0.08 + 0.26*f*clamp01((x-r.X)/r.W+0.2)
		p.Fill(ui.Rect{X: x - d/2, Y: y - d/2, W: d, H: d}, white.Alpha(a), d/2)
	}
}

// drawGalaxy hangs the day's first covers over the wave as discs of
// glass: each rides its crest, bobbing gently, lit from above and
// glowing in the card's color beneath, the nearer ones leaning further
// with the pointer; the first, while its songs play, carries a sheen
// that slowly turns.
func (a *app) drawGalaxy(p *ui.Painter, r ui.Rect, now time.Time, songs []netease.Track, deep, rich, glow ui.Color, spin float32, still, live bool, gx, gy float32) {
	t := float64(now.UnixNano()) / 1e9
	for i := range min(3, len(songs)) {
		d := [...]float32{98, 74, 56}[i]
		fromRight := [...]float32{80, 178, 264}[i]
		// The nearer a disc, the further it leans with the pointer.
		depth := 1 - 0.3*float32(i)
		x := r.X + r.W - dailyPadX - fromRight + gx*depth
		cy := auroraY(r, x-gx*depth, &a.dailyWave, t) - d*0.18 + gy*depth
		if !still {
			bob := 3 + float32(i)
			if live {
				bob += 3
			}
			cy += bob * float32(math.Sin(t*(1.1+0.23*float64(i))+float64(i)*1.9))
		}
		rect := ui.Rect{X: x - d/2, Y: cy - d/2, W: d, H: d}
		// The glow it throws around it, in three soft rings, and the pool
		// of light it casts on the wave below.
		for k, rr := range [...]float32{0.06, 0.05, 0.04} {
			grow := float32(k+1) * d * 0.12
			p.Fill(ui.Rect{X: rect.X - grow, Y: rect.Y - grow, W: d + 2*grow, H: d + 2*grow}, glow.Alpha(rr), d/2+grow)
		}
		p.Fill(ui.Rect{X: rect.X + d*0.12, Y: rect.Y + d*0.82, W: d * 0.76, H: d * 0.26}, glow.Alpha(0.22), d*0.13)
		p.Clip(rect, d/2, func() {
			if bmp := bitmap(a.art.get(netease.Sized(songs[i].Album.PicURL, 240), false)); bmp != nil {
				p.Image(bmp, rect, ui.Cover)
			} else {
				p.Fill(rect, rich, d/2)
			}
			// Light on the glass, strongest at its upper left, and its
			// own shade at the foot, so it reads as a sphere.
			p.FillGradient(rect, ui.LinearGradient{Angle: 160, Stops: []ui.GradientStop{
				{Color: white.Alpha(0.3), At: 0}, {Color: white.Alpha(0.05), At: 0.38}, {Color: white.Alpha(0), At: 0.56},
			}}, d/2)
			p.FillGradient(rect, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
				{Color: ui.RGBA(0, 0, 0, 0), At: 0.62}, {Color: ui.RGBA(0, 0, 0, 0.28), At: 1},
			}}, d/2)
			if i == 0 && live && !still {
				p.FillConicGradient(rect, ui.ConicGradient{Angle: spin, Stops: []ui.GradientStop{
					{Color: white.Alpha(0), At: 0}, {Color: white.Alpha(0.12), At: 0.1}, {Color: white.Alpha(0), At: 0.22},
					{Color: white.Alpha(0), At: 0.55}, {Color: white.Alpha(0.1), At: 0.65}, {Color: white.Alpha(0), At: 0.78},
					{Color: white.Alpha(0), At: 1},
				}}, d/2)
			}
		})
		// The rim of the glass, and the halo it catches.
		p.Stroke(rect, white.Alpha(0.32), d/2, 1.2)
		p.Stroke(ui.Rect{X: rect.X - 3, Y: rect.Y - 3, W: d + 6, H: d + 6}, glow.Alpha(0.16), d/2+3, 1)
	}
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// spinState turns the daily card's record: its angle, in degrees, and how
// fast it turns, as a part of a turntable's speed, which eases up and
// glides to a stop.
type spinState struct {
	angle, speed float64
	last         time.Time
}

// step turns the record on to now, its speed easing to to, and reports
// whether it still turns.
func (s *spinState) step(now time.Time, to float64) bool {
	dt := now.Sub(s.last).Seconds()
	if s.last.IsZero() || dt > 0.2 {
		dt = 1.0 / 60
	}
	s.last = now
	s.speed += (to - s.speed) * min(1, 2.2*dt)
	if math.Abs(to-s.speed) < 0.002 {
		s.speed = to
	}
	// 33⅓ turns a minute.
	s.angle = math.Mod(s.angle+s.speed*dt*200, 360)
	return s.speed > 0
}

// dailyArtists names the first few singers of songs, and how many songs.
func dailyArtists(songs []netease.Track) string {
	var names []string
	seen := map[string]bool{}
	for i := range songs {
		if len(songs[i].Artists) == 0 {
			continue
		}
		n := songs[i].Artists[0].Name
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		names = append(names, n)
		if len(names) == 3 {
			break
		}
	}
	if len(names) == 0 {
		return fmt.Sprintf("今日 %d 首", len(songs))
	}
	return fmt.Sprintf("%s 等 %d 首", strings.Join(names, "、"), len(songs))
}

// dailyPlayButton plays the day's songs; while they play it says so, its
// bars dancing, and pauses them; paused, it goes on.
func (a *app) dailyPlayButton(c *ui.Context, songs []netease.Track, ours, playing bool) {
	ink := ui.Hex("#1d1d1f")
	b := ui.ButtonBase(c).Height(38).PaddingX(20).Gap(8).Radius(999).AlignItems(ui.Center).TextColor(ink).
		Shadow(0, 6, 16, -6, ui.RGBA(0, 0, 0, 0.35))
	fill := white
	if b.Hovered() {
		fill = ui.Hex("#f2f2f4")
	}
	b.Background(b.AnimateColor("bg", fill, ui.Animation{Duration: 120 * time.Millisecond}))
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.95
	}
	b.AnimatePose(pose, ui.Snappy)
	label := "播放"
	// As wide hovered as not, its icon and its four characters: narrower,
	// it would leave the pointer, and flicker between the two.
	switch {
	case playing && b.Hovered():
		label = "暂停播放"
	case playing:
		label = "正在播放"
	case ours:
		label = "继续播放"
	}
	b.Label(label)
	b.Children(func() {
		if playing && !b.Hovered() {
			accent := a.pal.accent
			ui.Box(c).Size(14, 13).Draw(func(p *ui.Painter, r ui.Rect) {
				lv, live := a.pl.Levels()
				a.dailyBars.step(p.Now(), lv, live)
				drawBars(p, r, &a.dailyBars, 4, accent, 2.5)
				if live || a.dailyBars.group(0, player.Bands) > 0.01 {
					p.AnimationFrameRate(dailyRate)
				}
			})
		} else {
			icon := iconPlay
			if playing {
				icon = iconPause
			}
			ui.Icon(c, icon).Size(14, 14)
		}
		ui.Text(c, label).FontSize(14).FontWeight(650).SingleLine()
	})
	if b.Clicked() {
		if ours {
			a.pl.TogglePlay()
		} else {
			a.pl.PlayQueue(songs, firstPlayable(songs))
		}
	}
}

// dailyPicks lists the day's first songs, numbered, the one playing with
// its bars; each plays the day's songs from it.
func (a *app) dailyPicks(c *ui.Context, songs []netease.Track, ours bool) {
	cur := a.pl.Current()
	ui.Column(c.Key("picks")).Width(picksWidth).Shrink(0).Gap(2).Children(func() {
		for i := range min(3, len(songs)) {
			tr := &songs[i]
			row := ui.ButtonBase(c.Key(tr.ID)).Row().Gap(12).Padding(7, 12).Radius(12).AlignItems(ui.Center).FocusRing(false).Label(tr.Name).
				ContextMenu(a.songItems(tr, 0))
			bg := white.Alpha(0)
			if row.Hovered() {
				bg = white.Alpha(0.1)
			}
			row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
			current := ours && cur != nil && cur.ID == tr.ID
			row.Children(func() {
				ui.Box(c).Width(18).Center().Children(func() {
					if current {
						ui.Icon(c, iconPlay).Size(11, 11).TextColor(white)
					} else {
						ui.Textf(c, "%02d", i+1).FontSize(12).FontWeight(700).FontFeatures("tnum").TextColor(white.Alpha(0.45))
					}
				})
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, tr.Name).FontSize(13).FontWeight(600).MaxLines(1)
					a.artistLinks(c, tr, white.Alpha(0.6), white).FontSize(11.5).MaxLines(1)
				})
			})
			if row.Clicked() {
				a.pl.PlayQueue(songs, i)
			}
		}
	})
}

// dailyNote says, shortly, how the day's songs are picked.
func (a *app) dailyNote() string {
	if a.profile == nil {
		return "登录后为你专属生成"
	}
	return "每天 6:00 更新"
}

// dailyBlurb says whose taste the day's songs follow.
func (a *app) dailyBlurb() string {
	if a.profile == nil {
		return "大家都在听的好歌，登录后为你专属生成"
	}
	return "根据你的音乐口味生成，每天 6:00 更新"
}

func (a *app) dailyPage(c *ui.Context) {
	loading := len(a.daily) == 0
	if loading && !a.homeLoading {
		a.centered(c, "暂时没有推荐", a.loadHome)
		return
	}
	now := c.Now()
	songs := a.daily
	s := a.pageShimmer(c)
	header := func(c *ui.Context) {
		p := a.pal
		ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
			a.dailyArt(c, now, 212)
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "每日推荐").FontSize(12).FontWeight(600).TextColor(p.accent)
				ui.Text(c, "每日歌曲推荐").FontSize(30).FontWeight(800).LetterSpacing(-0.6)
				if loading {
					s.text(c, 13, 1.3).Width(240)
					ui.Row(c).Gap(10).Margin(8, 0, 0, 0).Children(func() {
						s.bone(c, 18).Size(112, 36)
						s.bone(c, 18).Size(112, 36)
					})
					return
				}
				ui.Text(c, fmt.Sprintf("%d 首 · %s", len(songs), a.dailyBlurb())).FontSize(13).TextColor(p.muted)
				ui.Row(c).Gap(10).Margin(8, 0, 0, 0).Children(func() {
					if pillButton(c.Key("play"), iconPlay, "播放全部", p.accent, p.onAccent).Clicked() {
						if a.pl.Shuffle() {
							a.pl.ToggleShuffle()
						}
						a.pl.PlayQueue(songs, firstPlayable(songs))
					}
					if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
						if !a.pl.Shuffle() {
							a.pl.ToggleShuffle()
						}
						a.pl.PlayQueue(songs, firstPlayable(songs))
					}
				})
			})
		})
	}
	if loading {
		a.trackSkeleton(c, s, true, header)
		return
	}
	a.trackRows(c, songs, true, header)
}
