package app

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// The weather radio: the sky outside, as a photograph the page wears at
// its head — with the weather itself moving over it, and the photograph
// drifting slower than the page scrolls — and the music it asks for
// below: the mood of the sky, a playlist to match, and the songs.

// wxPeriods are the beats the strip's sky breathes by: the rain falls
// fast, the snow comes slow, the lightning strikes on its own clock,
// and the stars twinkle each at its own.
var (
	wxRainFall = 1100 * time.Millisecond
	wxSnowFall = 7 * time.Second
	wxFlash    = 7 * time.Second
	wxTwinkle  = 2600 * time.Millisecond
	wxDrift    = 22 * time.Second
)

// wxHeroH is how tall the photograph at the head of the page stands.
const wxHeroH = 352

// wxPhase is t through one turn of period, from 0 to 1.
func wxPhase(now time.Time, period time.Duration, shift float64) float32 {
	t := float64(now.UnixNano()) / float64(period)
	t = t + shift
	return float32(t - math.Floor(t))
}

// weatherPage is the weather radio: the sky as it is, the music it asks
// for, and the playlist that answers.
func (a *app) weatherPage(c *ui.Context) {
	top, bottom := a.insets(c)
	if a.wx == nil {
		a.wx = newWx()
	}
	a.wx.ensure(a.post, nil)
	a.wx.ensureShot(a.post)
	a.wx.ensureTracks(a.post)
	if tr := a.pl.Current(); tr == nil && a.settings.WxAutoPlay && len(a.wx.tracks) > 0 && !a.wx.autoPlayed {
		a.wx.autoPlayed = true
		a.pl.PlayQueue(a.wx.tracks, firstPlayable(a.wx.tracks))
	}
	ui.Scroll(c.Key("weather")).TrackScroll(&a.pageScroll).Grow(1).
		Padding(top+pageGap, pagePad, bottom+56, pagePad).
		ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		switch {
		case a.wx.err != "":
			a.centered(c, a.wx.err, func() { a.wx.refresh(a.post, nil) })
			return
		case a.wx.data == nil:
			a.wxWaiting(c)
			return
		}
		a.wxHero(c)
		a.wxBoard(c)
	})
}

// wxWaiting is the page before the sky has been heard — the first run,
// or a network slow to answer: the shape the page will take, its colors
// moving, and a word that it is coming.
func (a *app) wxWaiting(c *ui.Context) {
	mood := wxMoodOf(wxPartly, false, 18)
	ui.Box(c.Key("wx-pulse")).Size(1, 1).Loop("wxpulse", 2*time.Second, ui.Linear)
	hero := ui.Box(c.Key("wx-wait")).Fill().Height(wxHeroH).Radius(26)
	hero.Draw(func(p *ui.Painter, rect ui.Rect) {
		p.Clip(rect, 26, func() {
			p.FillGradient(rect, ui.LinearGradient{Angle: 200, Oklab: true, Stops: []ui.GradientStop{
				{At: 0, Color: mood.From}, {At: 1, Color: mood.To},
			}}, 0)
			ph := wxPhase(c.Now(), 6*time.Second, 0)
			x := -rect.W*0.4 + ph*rect.W*1.8
			p.FillGradient(ui.Rect{X: rect.X + x, Y: rect.Y, W: rect.W * 0.5, H: rect.H},
				ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
					{At: 0, Color: ui.RGBA(255, 255, 255, 0)},
					{At: 0.5, Color: ui.RGBA(255, 255, 255, 0.12)},
					{At: 1, Color: ui.RGBA(255, 255, 255, 0)},
				}}, 0)
		})
	})
	hero.Children(func() {
		ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).
			AlignItems(ui.Center).Justify(ui.Center).Gap(6).Children(func() {
			ui.Text(c, "正在看天……").FontSize(15).FontWeight(700).TextColor(white.Alpha(0.92))
			ui.Text(c, "第一次打开要多等一会儿，之后就快了").FontSize(11.5).
				FontWeight(600).TextColor(white.Alpha(0.6))
		})
	})
	card := ui.Box(c.Key("wx-wait-sheet")).Fill().Padding(18, 20, 18, 20).Radius(24).
		Background(a.pal.bg).Border(1, a.pal.hairline).Margin(-30, 0, 0, 0)
	card.Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(64, 64).Radius(32).Background(a.pal.hover).Shrink(0)
			ui.Column(c).Grow(1).Gap(8).Children(func() {
				ui.Box(c).Height(12).Radius(6).Background(a.pal.hover).MaxWidth(220)
				ui.Box(c).Height(10).Radius(5).Background(a.pal.hover).MaxWidth(140)
			})
		})
	})
}

// wxHero is the sky itself, a photograph at the head of the page: the
// weather moving over it, the report laid along its foot, and its ground
// drifting a little behind the page as it scrolls.
func (a *app) wxHero(c *ui.Context) {
	w, mood := a.wx.data, a.wx.mood
	// A frame clock of its own: the sky keeps breathing even where
	// nothing else on the page asks for a frame.
	ui.Box(c.Key("wx-pulse")).Size(1, 1).Loop("wxpulse", 2*time.Second, ui.Linear)
	hero := ui.Box(c.Key("wx-hero")).Fill().Height(wxHeroH).Radius(26).
		Shadow(0, 26, 56, -18, ui.RGBA(0, 0, 0, 0.5))
	// The light passes over the photograph as the pointer comes onto it,
	// and again each time it comes back — not on a clock of its own.
	if hero.Hovered() {
		if !a.wx.hover {
			a.wx.hover = true
			a.wx.sweepAt = c.Now()
		}
	} else {
		a.wx.hover = false
	}
	hero.Draw(func(p *ui.Painter, rect ui.Rect) {
		p.Clip(rect, 26, func() { a.wxScene(p, rect, c.Now()) })
	})
	hero.Children(func() {
		// Where the listener is, in the city's own name.
		ui.Row(c.Key("wx-place")).Absolute().Left(22).Top(20).Height(30).Padding(0, 12).Gap(7).
			Radius(15).AlignItems(ui.Center).Background(ui.RGBA(0, 0, 0, 0.26)).
			Border(1, ui.RGBA(255, 255, 255, 0.16)).Children(func() {
			ui.Icon(c, iconPin).Size(13, 13).TextColor(white.Alpha(0.92))
			ui.Text(c, w.City).FontSize(12.5).FontWeight(700).TextColor(white)
			if w.Region != "" && w.Region != w.City {
				ui.Box(c).Size(3, 3).Radius(1.5).Background(white.Alpha(0.45))
				ui.Text(c, w.Region).FontSize(11.5).FontWeight(600).TextColor(white.Alpha(0.72))
			}
		})
		// The photograph changes, and the report is asked again.
		ui.Row(c.Key("wx-acts")).Absolute().Right(18).Top(18).Gap(8).Children(func() {
			if a.wxGlass(c, "wx-shot", iconPhoto, "换一张风景").Clicked() {
				a.wx.otherShot()
			}
			if a.wxGlass(c, "wx-refresh", iconReload, "刷新天气").Clicked() {
				a.wx.refresh(a.post, nil)
			}
		})
		// The report, along the foot of the photograph, high enough
		// that the sheet laid over it does not cover a word.
		ui.Row(c.Key("wx-report")).Absolute().Left(24).Right(22).Bottom(52).Gap(14).
			AlignItems(ui.End).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Baseline).Children(func() {
					ui.Textf(c, "%.0f", w.Temp).FontSize(74).FontWeight(800).
						LetterSpacing(-4).TextColor(white)
					ui.Text(c, "°").FontSize(30).FontWeight(700).
						TextColor(white.Alpha(0.75)).Margin(0, 0, 8, 0)
					ui.Column(c).Gap(3).Margin(0, 0, 8, 10).Children(func() {
						ui.Text(c, wxDescOf(w)).FontSize(14.5).FontWeight(800).TextColor(white)
						if w.High != 0 || w.Low != 0 {
							ui.Textf(c, "最高 %.0f° · 最低 %.0f°", w.High, w.Low).
								FontSize(11.5).FontWeight(600).TextColor(white.Alpha(0.72))
						}
					})
				})
				ui.Text(c, mood.Emoji+" "+mood.Title).FontSize(15.5).FontWeight(800).
					TextColor(white.Alpha(0.94))
			})
			ui.Column(c).Gap(6).AlignItems(ui.End).Children(func() {
				ui.Row(c).Gap(6).Children(func() {
					a.wxGlassChip(c, fmt.Sprintf("体感 %.0f°", w.Feels))
					a.wxGlassChip(c, fmt.Sprintf("湿度 %d%%", w.Humidity))
					if w.Wind != "" {
						a.wxGlassChip(c, w.Wind)
					}
				})
				if w.Sunrise != "" && w.Sunset != "" {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconSunrise).Size(12, 12).TextColor(white.Alpha(0.6))
						ui.Textf(c, "日出 %s · 日落 %s", w.Sunrise, w.Sunset).FontSize(10.5).
							FontWeight(600).TextColor(white.Alpha(0.62))
					})
				}
				if a.wx.shot.Credit != "" {
					ui.Textf(c, "图 · %s", a.wx.shot.Credit).FontSize(9.5).FontWeight(500).
						TextColor(white.Alpha(0.38)).SingleLine().MaxWidth(330)
				}
			})
		})
	})
}

// wxScene paints the photograph and everything the sky does over it.
func (a *app) wxScene(p *ui.Painter, rect ui.Rect, now time.Time) {
	mood := a.wx.mood
	shot := a.wx.shot
	var bmp *ui.Bitmap
	if shot.URL != "" {
		art := a.art.get(shot.URL, false)
		bmp = bitmap(art)
		switch {
		case bmp != nil:
			a.wx.shown = shot.URL
		case a.wx.shown != "" && a.wx.shown != shot.URL:
			// The next photograph is still coming: the one that showed
			// stands in for it, so the strip never goes bare.
			bmp = bitmap(a.art.get(a.wx.shown, false))
		}
		// A photograph too bright to read a report over is no ground
		// for one: the page turns it down and takes another.
		if art != nil && !art.failed && art.lum > 0.6 && a.wx.shotTry < 4 {
			a.wx.shotTry++
			a.wx.wash(shot.URL)
			return
		}
	}
	// The photograph drifts behind the page: it moves up slower than
	// the page scrolls, and breathes a little larger as it goes.
	scroll := a.pageScroll.Y
	if scroll < 0 {
		scroll = 0
	}
	shift := scroll * 0.34
	if shift > 110 {
		shift = 110
	}
	zoom := 1.07 + 0.04*wxPhase(now, 90*time.Second, 0)
	gw, gh := rect.W*(zoom-1)/2, rect.H*(zoom-1)/2
	dst := ui.Rect{X: rect.X - gw, Y: rect.Y - gh + shift, W: rect.W + gw*2, H: rect.H + gh*2}
	if bmp != nil {
		p.Image(bmp, dst, ui.Cover)
	} else {
		// Before the photograph comes, and where it cannot: the mood's
		// own colors, moving slowly as a sky does.
		p.FillGradient(rect, ui.LinearGradient{Angle: 200, Oklab: true, Stops: []ui.GradientStop{
			{At: 0, Color: mood.From}, {At: 1, Color: mood.To},
		}}, 0)
		a.wxAurora(p, rect, now)
	}
	a.wxGrade(p, rect)
	a.wxWeather(p, rect, now)
	a.wxSweep(p, rect, now)
}

// wxSweep is light passing over the photograph as the pointer touches
// it, and again each time it comes back.
func (a *app) wxSweep(p *ui.Painter, rect ui.Rect, now time.Time) {
	lightSweep(p, rect, a.wx.sweepAt, now)
}

// wxAurora is the fallback sky: soft fields of the mood's colors,
// drifting over one another.
func (a *app) wxAurora(p *ui.Painter, rect ui.Rect, now time.Time) {
	mood := a.wx.mood
	for i := 0; i < 7; i++ {
		ph := wxPhase(now, 40*time.Second+time.Duration(i)*7*time.Second, float64(i)*0.29)
		cx := rect.X + rect.W*(0.1+0.8*float32(math.Sin(float64(ph)*2*math.Pi))*0.5+0.4)
		cy := rect.Y + rect.H*(0.15+0.7*ph)
		r := 120 + float32(i)*46
		c := mood.From
		if i%2 == 1 {
			c = mood.To
		}
		p.Fill(ui.Rect{X: cx - r, Y: cy - r, W: r * 2, H: r * 2}, c.Alpha(0.09), r)
	}
}

// wxGrade gives the photograph the mood's light, and the shadows a
// report reads over: a tint, a foot, and the corners drawn in.
func (a *app) wxGrade(p *ui.Painter, rect ui.Rect) {
	mood := a.wx.mood
	p.FillGradient(rect, ui.LinearGradient{Angle: 160, Oklab: true, Stops: []ui.GradientStop{
		{At: 0, Color: mood.From.Alpha(0.34)},
		{At: 0.55, Color: mood.To.Alpha(0.1)},
		{At: 1, Color: mood.To.Alpha(0.3)},
	}}, 0)
	// The foot, so the report stands off the photograph.
	p.FillGradient(rect, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{At: 0.32, Color: ui.RGBA(0, 0, 0, 0)},
		{At: 0.66, Color: ui.RGBA(0, 0, 0, 0.28)},
		{At: 1, Color: ui.RGBA(0, 0, 0, 0.86)},
	}}, 0)
	// The head, so the place and the buttons stand off it too.
	p.FillGradient(rect, ui.LinearGradient{Angle: 0, Stops: []ui.GradientStop{
		{At: 0.55, Color: ui.RGBA(0, 0, 0, 0)},
		{At: 1, Color: ui.RGBA(0, 0, 0, 0.42)},
	}}, 0)
	// The corners, drawn in as a lens draws them.
	corner := func(angle float32) {
		p.FillGradient(rect, ui.LinearGradient{Angle: angle, Stops: []ui.GradientStop{
			{At: 0, Color: ui.RGBA(0, 0, 0, 0.3)},
			{At: 0.22, Color: ui.RGBA(0, 0, 0, 0)},
		}}, 0)
	}
	corner(90)
	corner(270)
}

// wxWeather draws the sky's own motion over the photograph: its rain,
// its snow, its fog, its wind, its lightning, and its light.
func (a *app) wxWeather(p *ui.Painter, rect ui.Rect, now time.Time) {
	w := a.wx.data
	switch w.Kind {
	case wxRain:
		a.wxRain(p, rect, now, 76)
	case wxThunder:
		a.wxRain(p, rect, now, 96)
		a.wxLightning(p, rect, now)
	case wxSnow:
		a.wxSnow(p, rect, now, 64)
	case wxFog:
		a.wxFog(p, rect, now)
	case wxWind:
		a.wxWind(p, rect, now)
	case wxSun, wxPartly:
		if w.Night {
			a.wxStars(p, rect, now)
		} else {
			a.wxSunlight(p, rect, now)
		}
	case wxCloud:
		if w.Night {
			a.wxStars(p, rect, now)
		} else {
			a.wxDriftClouds(p, rect, now)
		}
	}
}

// wxRain is rain in three depths: fine and slow far off, long and quick
// close by, all of it leaning the way the wind goes.
func (a *app) wxRain(p *ui.Painter, rect ui.Rect, now time.Time, n int) {
	at := func(x, y, w, h float32) ui.Rect {
		return ui.Rect{X: rect.X + x, Y: rect.Y + y, W: w, H: h}
	}
	// A wet haze near the ground, where the rain gathers.
	p.FillGradient(rect, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{At: 0.55, Color: ui.RGBA(255, 255, 255, 0)},
		{At: 1, Color: ui.RGBA(210, 226, 240, 0.09)},
	}}, 0)
	for i := 0; i < n; i++ {
		depth := i % 3
		seed := float32(math.Mod(float64(i)*47.13, 100)) / 100
		period := time.Duration(float64(wxRainFall) * (1.75 - 0.42*float64(depth)))
		ph := wxPhase(now, period, float64(i)*0.137)
		length := 10 + float32(depth)*9 + float32(i%4)*2
		width := 0.8 + float32(depth)*0.45
		alpha := 0.1 + float32(depth)*0.075
		x := seed*rect.W + 26*float32(math.Sin(float64(wxPhase(now, wxDrift, float64(i)*0.4))*2*math.Pi))
		y := ph*(rect.H+140) - 70
		p.Line(rect.X+x, rect.Y+y, rect.X+x-2.4-float32(depth)*1.6, rect.Y+y+length, width, white.Alpha(alpha))
	}
	// A drop or two close to the lens, out of focus.
	for i := 0; i < 6; i++ {
		ph := wxPhase(now, 5*time.Second+time.Duration(i)*900*time.Millisecond, float64(i)*0.21)
		x := float32(math.Mod(float64(i)*38.7, 100)) / 100 * rect.W
		y := ph*(rect.H+60) - 30
		p.Fill(at(x, y, 5, 7), white.Alpha(0.06), 2.5)
	}
}

// wxSnow is the snow, three depths of it, swaying as it comes down.
func (a *app) wxSnow(p *ui.Painter, rect ui.Rect, now time.Time, n int) {
	at := func(x, y, w, h float32) ui.Rect {
		return ui.Rect{X: rect.X + x, Y: rect.Y + y, W: w, H: h}
	}
	for i := 0; i < n; i++ {
		depth := i % 3
		seed := float32(math.Mod(float64(i)*37.91, 100)) / 100
		period := time.Duration(float64(wxSnowFall) * (1.9 - 0.5*float64(depth)))
		ph := wxPhase(now, period, float64(i)*0.13)
		y := ph*(rect.H+60) - 30
		sway := (10 + float32(depth)*9) * float32(math.Sin(float64(ph)*2*math.Pi+float64(i)))
		r := 1.1 + float32(depth)*0.9 + float32(i%3)*0.4
		x := seed*rect.W + sway
		p.Fill(at(x-r*2, y-r*2, r*4, r*4), white.Alpha(0.07+0.05*float32(depth)), r*2)
		p.Fill(at(x-r, y-r, r*2, r*2), white.Alpha(0.5+0.16*float32(depth)), r)
	}
}

// wxFog is the fog: broad bands of it, gliding over one another, and a
// veil of it over all.
func (a *app) wxFog(p *ui.Painter, rect ui.Rect, now time.Time) {
	at := func(x, y, w, h float32) ui.Rect {
		return ui.Rect{X: rect.X + x, Y: rect.Y + y, W: w, H: h}
	}
	for i := 0; i < 5; i++ {
		ph := wxPhase(now, wxDrift*time.Duration(2+i), float64(i)*0.31)
		x := -320 + ph*(rect.W+640)
		y := 26 + float32(i)*54 + 10*float32(math.Sin(float64(ph)*2*math.Pi))
		w := 300 + float32(i%3)*120
		p.Fill(at(x, y, w, 26+float32(i%2)*16), white.Alpha(0.055+0.02*float32(i%3)), 18)
	}
	p.FillGradient(rect, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{At: 0, Color: white.Alpha(0.1)},
		{At: 0.5, Color: white.Alpha(0.04)},
		{At: 1, Color: white.Alpha(0.12)},
	}}, 0)
}

// wxWind is the wind, streaks of it hurrying across, and a leaf or two
// going with it.
func (a *app) wxWind(p *ui.Painter, rect ui.Rect, now time.Time) {
	mood := a.wx.mood
	for i := 0; i < 12; i++ {
		seed := float32(math.Mod(float64(i)*53.3, 100)) / 100
		period := 2*time.Second + time.Duration(i%3)*700*time.Millisecond
		ph := wxPhase(now, period, float64(i)*0.23)
		x := -180 + ph*(rect.W+360)
		y := 26 + seed*rect.H*0.8
		p.Line(rect.X+x, rect.Y+y, rect.X+x+70+float32(i%4)*34, rect.Y+y-3+float32(i%2)*6,
			1.4+float32(i%3)*0.5, white.Alpha(0.14))
	}
	for i := 0; i < 4; i++ {
		ph := wxPhase(now, 6*time.Second+time.Duration(i)*1500*time.Millisecond, float64(i)*0.4)
		x := -60 + ph*(rect.W+120)
		y := 60 + float32(i)*58 + 18*float32(math.Sin(float64(ph)*4*math.Pi))
		p.Fill(ui.Rect{X: rect.X + x, Y: rect.Y + y, W: 9, H: 4}, mood.To.Alpha(0.55), 2)
	}
}

// wxStars is the night's own light: stars, each twinkling at its own
// beat, and a soft glow where the moon is.
func (a *app) wxStars(p *ui.Painter, rect ui.Rect, now time.Time) {
	at := func(x, y, w, h float32) ui.Rect {
		return ui.Rect{X: rect.X + x, Y: rect.Y + y, W: w, H: h}
	}
	for i := 0; i < 46; i++ {
		sx := float32(math.Mod(float64(i)*73.71, 100)) / 100
		sy := float32(math.Mod(float64(i)*41.37, 62)) / 100
		ph := wxPhase(now, wxTwinkle, float64(i)*0.37)
		tw := 0.2 + 0.62*float32(math.Abs(math.Sin(float64(ph)*2*math.Pi)))
		r := 0.9 + float32(i%4)*0.35
		p.Fill(at(sx*rect.W, sy*rect.H*0.7, r*2, r*2), white.Alpha(tw), r)
	}
	// The moon's glow, low and to the right.
	mx, my := rect.W-150, float32(74)
	for i := 6; i >= 1; i-- {
		r := float32(i) * 22
		p.Fill(at(mx-r, my-r, r*2, r*2), white.Alpha(0.018), r)
	}
}

// wxSunlight is the light of a clear day: a warm glow high and to the
// right, and the dust that floats in it.
func (a *app) wxSunlight(p *ui.Painter, rect ui.Rect, now time.Time) {
	at := func(x, y, w, h float32) ui.Rect {
		return ui.Rect{X: rect.X + x, Y: rect.Y + y, W: w, H: h}
	}
	breathe := 0.85 + 0.15*float32(math.Sin(float64(wxPhase(now, 7*time.Second, 0))*2*math.Pi))
	sx, sy := rect.W-130, float32(30)
	for i := 8; i >= 1; i-- {
		r := float32(i) * 34
		p.Fill(at(sx-r, sy-r, r*2, r*2), ui.Hex("#ffd9a0").Alpha(0.022*breathe), r)
	}
	// The dust, drifting up through the light.
	for i := 0; i < 24; i++ {
		seed := float32(math.Mod(float64(i)*61.7, 100)) / 100
		ph := wxPhase(now, 16*time.Second+time.Duration(i)*700*time.Millisecond, float64(i)*0.19)
		x := seed*rect.W + 16*float32(math.Sin(float64(ph)*2*math.Pi))
		y := rect.H - ph*rect.H
		r := 0.7 + float32(i%3)*0.4
		p.Fill(at(x, y, r*2, r*2), ui.Hex("#ffe7bd").Alpha(0.3), r)
	}
}

// wxDriftClouds is a grey day: the light flat, the clouds crossing
// slowly over the photograph.
func (a *app) wxDriftClouds(p *ui.Painter, rect ui.Rect, now time.Time) {
	for i := 0; i < 3; i++ {
		ph := wxPhase(now, 70*time.Second, float64(i)*0.34)
		x := -380 + ph*(rect.W+760)
		y := 10 + float32(i)*42
		w, h := 340+float32(i%2)*120, float32(70)
		for k := 3; k >= 1; k-- {
			p.Fill(ui.Rect{X: rect.X + x + float32(k)*10, Y: rect.Y + y - float32(k)*8,
				W: w - float32(k)*30, H: h}, white.Alpha(0.05), h/2)
		}
	}
}

// wxLightning is the storm's flash, and the bolt it comes from.
func (a *app) wxLightning(p *ui.Painter, rect ui.Rect, now time.Time) {
	ph := wxPhase(now, wxFlash, 0)
	flash := float32(0)
	for _, beat := range []float32{0.5, 0.57} {
		d := ph - beat
		if d < 0 {
			d = -d
		}
		flash += float32(math.Max(0, 1-float64(d)/0.02))
	}
	if flash <= 0 {
		return
	}
	p.Fill(rect, white.Alpha(0.42*flash), 0)
	// The bolt itself, down from the top right of the sky.
	x := rect.X + rect.W*0.66
	y := rect.Y
	points := [][2]float32{{x, y}}
	steps := []struct{ dx, dy float32 }{{14, 34}, {-10, 30}, {12, 26}, {-16, 30}, {8, 22}}
	for _, s := range steps {
		x, y = x+s.dx, y+s.dy
		points = append(points, [2]float32{x, y})
	}
	p.StrokePolyline(points, 2.2, white.Alpha(0.5+0.4*flash))
}

// wxChip is one small fact of the weather, in a glass pill of its own.
func (a *app) wxGlassChip(c *ui.Context, s string) {
	ui.Box(c).Padding(0, 9).Height(24).Radius(12).Background(ui.RGBA(0, 0, 0, 0.3)).
		Border(1, ui.RGBA(255, 255, 255, 0.14)).
		AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Text(c, s).FontSize(10.5).FontWeight(600).TextColor(white.Alpha(0.9))
	})
}

// wxGlass is a round button that floats over the photograph.
func (a *app) wxGlass(c *ui.Context, key any, icon *ui.SVG, label string) ui.Element {
	b := ui.Box(c.Key(key)).Size(32, 32).Radius(16).Label(label).
		AlignItems(ui.Center).Justify(ui.Center).
		Background(ui.RGBA(0, 0, 0, 0.26)).Border(1, ui.RGBA(255, 255, 255, 0.16))
	if b.Hovered() {
		b.Background(ui.RGBA(0, 0, 0, 0.4))
		b.Border(1, ui.RGBA(255, 255, 255, 0.32))
	}
	if b.Pressed() {
		b.Background(ui.RGBA(255, 255, 255, 0.22))
	}
	b.Children(func() { ui.Icon(c, icon).Size(15, 15).TextColor(white) })
	return b
}

// wxBoard is what the sky asked for: the playlist that answers it, the
// songs of it, and the buttons to play them or ask again. It sits over
// the foot of the photograph, as a sheet laid on it.
func (a *app) wxBoard(c *ui.Context) {
	playing := a.wx.playingHere(a.pl)
	card := ui.Box(c.Key("wx-board")).Fill().Padding(18, 18, 14, 18).Radius(24).
		Background(a.pal.bg).Border(1, a.pal.hairline).
		Shadow(0, 22, 46, -20, a.pal.cardShade).Margin(-30, 0, 0, 0)
	card.Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			a.wxPlay(c, playing, 64)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, "天气电台 · 为此刻的天色选歌").FontSize(11).FontWeight(700).
					TextColor(a.pal.faint).SingleLine()
				a.wxBoardTitle(c)
			})
			a.wxReroll(c)
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(12, 0, 12, 0).Children(func() {
			if kw := a.wx.seekKeyword; kw != "" {
				ui.Row(c).Padding(0, 10).Height(24).Radius(12).Shrink(0).
					Background(a.pal.accent.Alpha(0.1)).Border(1, a.pal.accent.Alpha(0.18)).
					AlignItems(ui.Center).Gap(5).Children(func() {
					ui.Box(c).Size(4, 4).Radius(2).Background(a.pal.accent)
					ui.Text(c, kw).FontSize(11).FontWeight(700).TextColor(a.pal.accent)
				})
			}
			ui.Row(c).Grow(1).Justify(ui.End).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "打开时自动播放").FontSize(11.5).FontWeight(600).TextColor(a.pal.muted)
				if ui.Switch(c.Key("wx-auto"), &a.settings.WxAutoPlay).Label("打开时自动播放").Changed() {
					a.changed()
				}
			})
		})
		if msg := a.wx.trouble(); msg != "" {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Margin(0, 0, 12, 0).Children(func() {
				ui.Text(c, msg).FontSize(12.5).TextColor(a.pal.muted).Grow(1)
				if ui.Button(c, "重试").Clicked() {
					a.wx.retry(a.post)
				}
			})
			return
		}
		if len(a.wx.tracks) == 0 {
			ui.Text(c, "正在按天气找歌……").FontSize(13).TextColor(a.pal.faint)
			return
		}
		ui.Box(c).Fill().Height(1).Background(a.pal.hairline).Margin(0, 0, 8, 0)
		a.wxSongList(c)
	})
}

// wxBoardTitle is the name of the playlist the weather picked, and the
// line under it: how much of it there is.
func (a *app) wxBoardTitle(c *ui.Context) {
	pl := a.wx.picked
	if pl == nil {
		ui.Text(c, "正在按天气找歌单……").FontSize(17).FontWeight(800).
			TextColor(a.pal.text).SingleLine()
		return
	}
	row := ui.Row(c).Gap(8).AlignItems(ui.Center).Radius(8).Label("歌单" + pl.Name)
	if row.Hovered() {
		row.Opacity(0.72)
	}
	if row.Clicked() {
		a.open(route{page: pagePlaylist, id: pl.ID})
	}
	row.Children(func() {
		ui.Text(c, pl.Name).FontSize(17).FontWeight(800).TextColor(a.pal.text).SingleLine().Shrink(1)
		ui.Textf(c, "%d 首 · %.0f 万播放", pl.TrackCount, pl.PlayCount/10000).
			FontSize(11.5).FontWeight(600).TextColor(a.pal.muted).Shrink(0)
	})
}

// wxPlay is the button that plays the weather, and pauses it: a disc of
// the mood's own light, glowing as it sounds.
func (a *app) wxPlay(c *ui.Context, playing bool, size float32) ui.Element {
	mood := a.wx.mood
	b := ui.Box(c.Key("wx-play")).Size(size, size).Radius(size / 2).Label("播放/暂停").
		AlignItems(ui.Center).Justify(ui.Center)
	b.LinearGradient(ui.LinearGradient{Angle: 145, Oklab: true, Stops: []ui.GradientStop{
		{At: 0, Color: mood.From.Mix(white, 0.08)},
		{At: 1, Color: mood.To.Mix(ui.Hex("#000000"), 0.22)},
	}})
	glow := float32(0.38)
	if playing {
		glow = 0.55 + 0.2*float32(math.Sin(float64(wxPhase(c.Now(), 3*time.Second, 0))*2*math.Pi))
	}
	b.Shadow(0, 8, 24, 0, mood.To.Alpha(glow))
	if b.Hovered() {
		b.Shadow(0, 12, 32, 2, mood.To.Alpha(glow+0.15))
	}
	if b.Pressed() {
		b.Scale(0.97, 0.97)
	}
	b.Border(1, ui.RGBA(255, 255, 255, 0.28))
	b.Children(func() {
		icon := iconPlay
		if playing {
			icon = iconPause
		}
		ui.Icon(c, icon).Size(size*0.36, size*0.36).TextColor(white).Margin(0, 0, 0, 0)
	})
	if b.Clicked() && len(a.wx.tracks) > 0 {
		if playing || (a.pl.Current() != nil && a.pl.Playing()) {
			a.pl.TogglePlay()
		} else {
			a.pl.PlayQueue(a.wx.tracks, firstPlayable(a.wx.tracks))
		}
	}
	return b
}

// wxReroll asks the weather for another playlist: a pill that lights up
// in the mood's color under the pointer.
func (a *app) wxReroll(c *ui.Context) ui.Element {
	b := ui.Row(c.Key("wx-reroll")).Height(34).Padding(0, 14).Gap(7).Radius(17).Shrink(0).
		AlignItems(ui.Center).Background(a.pal.hover).Border(1, a.pal.hairline).
		Label("换一批")
	if b.Hovered() {
		b.Background(a.pal.accent.Alpha(0.12))
		b.Border(1, a.pal.accent.Alpha(0.32))
	}
	if b.Pressed() {
		b.Background(a.pal.accent.Alpha(0.2))
	}
	b.Children(func() {
		ui.Icon(c, iconShuffle).Size(13, 13).TextColor(a.pal.accent)
		ui.Text(c, "换一批").FontSize(12.5).FontWeight(700).TextColor(a.pal.text)
	})
	if b.Clicked() {
		a.wx.reroll(a.post)
	}
	return b
}

// wxSongList is the songs of the playlist the weather picked, fullest
// click plays from there.
func (a *app) wxSongList(c *ui.Context) {
	cur := a.pl.Current()
	ui.Column(c).Gap(1).Children(func() {
		for i, t := range a.wx.tracks {
			i, t := i, t
			now := cur != nil && cur.ID == t.ID
			var row ui.Element
			if now {
				row = ui.Row(c.Key(fmt.Sprintf("t%d", t.ID))).Fill().Height(54).Padding(0, 10).Gap(12).
					AlignItems(ui.Center).Radius(12).Background(a.pal.accent.Alpha(0.07)).
					Label("天气歌曲" + t.Name)
			} else {
				row = ui.Row(c.Key(fmt.Sprintf("t%d", t.ID))).Fill().Height(54).Padding(0, 10).Gap(12).
					AlignItems(ui.Center).Radius(12).Label("天气歌曲" + t.Name)
			}
			if !now && row.Hovered() {
				row.Background(a.pal.hover)
			}
			if row.Clicked() && t.Playable() {
				a.pl.PlayQueue(a.wx.tracks, i)
			}
			hovered := row.Hovered()
			row.Children(func() {
				// The number, or what the row does under the pointer.
				ui.Box(c).Size(22, 22).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					switch {
					case now:
						a.wxBars(c, 14)
					case hovered:
						ui.Icon(c, iconPlay).Size(11, 11).TextColor(a.pal.text)
					default:
						ui.Textf(c, "%d", i+1).FontSize(12).FontWeight(600).
							FontFeatures("tnum").TextColor(a.pal.faint)
					}
				})
				cover := ui.Box(c.Key("cover")).Size(40, 40).Radius(9).Background(a.pal.hover).Shrink(0)
				bmp := bitmap(a.art.get(netease.Sized(t.Album.PicURL, 160), false))
				in := artIn(cover, bmp)
				cover.Children(func() { artImage(c, bmp, 9, in) })
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
					name := ui.Text(c, t.Name).FontSize(13.5).FontWeight(700).SingleLine()
					if now {
						name.TextColor(a.pal.accent)
					}
					if t.ArtistNames() != "" {
						ui.Text(c, t.ArtistNames()).FontSize(11.5).TextColor(a.pal.muted).SingleLine()
					}
				})
				if t.Album.Name != "" {
					ui.Text(c, t.Album.Name).FontSize(11.5).TextColor(a.pal.faint).
						SingleLine().MaxWidth(180).Shrink(0)
				}
				ui.Text(c, clock(t.Length())).FontSize(12).FontWeight(600).
					FontFeatures("tnum").TextColor(a.pal.faint)
			})
		}
	})
}

// wxBars draws the three bars that say a song is sounding.
func (a *app) wxBars(c *ui.Context, h float32) {
	box := ui.Box(c.Key("bars")).Size(14, h)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		for i := 0; i < 3; i++ {
			ph := wxPhase(c.Now(), 900*time.Millisecond+time.Duration(i)*260*time.Millisecond, float64(i)*0.31)
			bh := 3 + (r.H-3)*(0.35+0.65*float32(math.Abs(math.Sin(float64(ph)*2*math.Pi))))
			x := r.X + float32(i)*(r.W/4.2)
			p.Fill(ui.Rect{X: x, Y: r.Y + r.H - bh, W: 2.4, H: bh}, a.pal.accent, 1.2)
		}
	})
}

// playingHere says whether what plays is one of the weather's own
// songs, for the board's button to know pause from resume.
func (s *wxState) playingHere(pl *player.Player) bool {
	cur := pl.Current()
	if cur == nil {
		return false
	}
	for _, t := range s.tracks {
		if t.ID == cur.ID {
			return true
		}
	}
	return false
}

// trouble is what the weather's page has to say when its playlist or
// its songs did not come.
func (s *wxState) trouble() string {
	if s.plErr != "" {
		return s.plErr
	}
	if s.picked != nil && s.tracksErr != "" {
		return s.tracksErr
	}
	return ""
}

// retry asks again for what did not come.
func (s *wxState) retry(post func(func())) {
	if s.plErr != "" {
		s.plErr = ""
		s.ensurePlaylists(post)
		return
	}
	s.tracksErr, s.tracks = "", nil
	s.ensureTracks(post)
}

// reroll throws the mood's dice again: another keyword, another
// playlist, another page of songs.
func (s *wxState) reroll(post func(func())) {
	s.playlists, s.plErr, s.picked, s.tracks, s.tracksErr = nil, "", nil, nil, ""
	if s.mood.Keywords != nil {
		// A different keyword this time: the same one would give the
		// same list back.
		s.seekKeyword = ""
	}
	s.ensurePlaylists(post)
}
