package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// weeklyPage is the week's listening, a page of its own from the sidebar:
// a poster at its head — the week's best-loved cover held wide and dim,
// the minutes it ran for in figures too big to miss, and the seven days
// drawn as a ring around the song that carried them — then the clock it
// kept, wide as the page, and the songs and singers it heard the most.
//
// weekHeroH is how tall that poster stands: a little taller than the
// daily card, so the two pages open at the same weight.
const weekHeroH = 228

func (a *app) weeklyPage(c *ui.Context) {
	top, bottom := a.insets(c)
	now := c.Now()
	var r weeklyReport
	if a.weekly != nil {
		r = a.weekly.report(now)
	}
	// The colors of the page are the colors of every other page: its own
	// ground, untouched, and the accent the listener chose — not a shade
	// borrowed from the week's cover, which made this one page read as a
	// different room.
	tint := a.pal.accent
	hi := tint.Mix(white, 0.62)
	if !a.pal.dark {
		hi = tint.Mix(ui.RGB(0, 0, 0), 0.28)
	}
	lo := a.pal.muted
	faint := a.pal.faint

	ui.Scroll(c.Key("weekly")).TrackScroll(&a.pageScroll).Grow(1).
		Padding(top+pageGap, pagePad, bottom+56, pagePad).
		ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		if r.Songs == 0 {
			ui.Column(c).Fill().MinHeight(300).Center().Gap(10).Children(func() {
				ui.Icon(c, iconWeekly).Size(38, 38).TextColor(faint)
				ui.Text(c, "这一周还没有听歌").FontSize(15).FontWeight(600).TextColor(lo)
				ui.Text(c, "放一首歌，周报会自己长出来").FontSize(12.5).TextColor(faint)
			})
			return
		}

		// The week's poster, and then its clock, drawn as wide as the
		// page: the two things the week is worth a look for.
		var st weeklyStats
		if a.weekly != nil {
			st = a.weekly.stats(now)
		}
		var news []weeklyTop
		if a.weekly != nil {
			news = a.weekly.newThisWeek(now)
		}
		a.weeklyHero(c, r, st)
		a.weeklyHours(c, r, now, tint, hi, faint)

		// The rest of it in two columns: the songs and singers after the
		// first at the left, the covers and small numbers at the right.
		ui.Row(c).AlignItems(ui.Start).Gap(40).Margin(26, 0, 0, 0).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).MaxWidth(560).Children(func() {
				// The week at a glance: the days it ran without pause, how a
				// day of it sits against the days before, and the fullest day.
				ui.Row(c).Gap(10).MaxWidth(560).Children(func() {
					a.weeklyChip(c, fmt.Sprintf("%d", st.Streak), "连续听歌 · 天")
					a.weeklyChip(c, fmt.Sprintf("%.0f", st.AvgDay), "日均 · 分钟")
					a.weeklyChip(c, fmt.Sprintf("%d", st.BestDayMin), "单日最高 · 分钟")
				})

				// The songs after the first, each with its cover for a face;
				// hovered, a play button stands in for the count, and a
				// click plays the song at once.
				if len(r.Tops) > 1 {
					ui.Text(c, "其他常听").FontSize(12).FontWeight(700).TextColor(faint).Margin(28, 0, 10, 0)
					ui.Column(c).MaxWidth(560).Gap(4).Children(func() {
						for i, t := range r.Tops[1:] {
							rkey := t.Track.Pic
							if rkey == "" {
								rkey = t.Track.Name
							}
							row := ui.Row(c.Key(rkey)).Fill().Height(52).Padding(0, 10).Gap(12).
								AlignItems(ui.Center).Radius(10).Label("周报歌曲" + t.Track.Name)
							hovered := row.Hovered()
							if hovered {
								row.Background(a.pal.hover)
							}
							row.Children(func() {
								// The second and the third carry their medals;
								// the rest wear the house color.
								bg, tx := hi.Alpha(0.16), hi
								if i == 0 {
									bg, tx = ui.Hex("#9fa4ad"), ui.Hex("#ffffff")
								} else if i == 1 {
									bg, tx = ui.Hex("#c08a5a"), ui.Hex("#ffffff")
								}
								ui.Box(c).Size(22, 22).Radius(11).Background(bg).
									AlignItems(ui.Center).Justify(ui.Center).Children(func() {
									ui.Textf(c, "%d", i+2).FontSize(12).FontWeight(800).FontFeatures("tnum").TextColor(tx)
								})
								cover := ui.Box(c.Key("cover")).Size(38, 38).Radius(7).Background(a.pal.hover).Shrink(0)
								bmp := bitmap(a.art.get(netease.Sized(t.Track.Pic, 160), false))
								in := artIn(cover, bmp)
								cover.Children(func() { artImage(c, bmp, 7, in) })
								ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
									ui.Text(c, t.Track.Name).FontSize(13.5).FontWeight(700).SingleLine()
									if t.Track.Artist != "" {
										ui.Text(c, t.Track.Artist).FontSize(11.5).TextColor(lo).SingleLine()
									}
								})
								if hovered {
									ui.Box(c).Size(26, 26).Radius(13).Background(a.pal.accent).
										AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
										Children(func() {
											ui.Icon(c, iconPlay).Size(12, 12).TextColor(white)
										})
								}
								ui.Textf(c, "%d 分钟", t.Minutes).FontSize(12).FontWeight(600).
									FontFeatures("tnum").TextColor(lo)
							})
							if row.Clicked() {
								a.pl.PlayQueue([]netease.Track{t.song()}, 0)
							}
						}
					})
				}

				// The artists their songs ran to; a pill is a door — click
				// it and the search goes looking for the singer.
				if len(r.Artists) > 0 {
					ui.Text(c, "最常听的歌手").FontSize(12).FontWeight(700).TextColor(faint).Margin(24, 0, 10, 0)
					ui.Row(c).MaxWidth(560).Gap(8).Wrap().Children(func() {
						for _, ar := range r.Artists {
							pill := ui.Row(c.Key(ar.Name)).Padding(0, 12, 0, 12).Height(30).Radius(999).Gap(6).
								AlignItems(ui.Center).Background(a.pal.hover).Label("搜索歌手 " + ar.Name)
							hovered := pill.Hovered()
							if hovered {
								pill.Background(hi.Alpha(0.18))
							}
							pill.Children(func() {
								ui.Text(c, ar.Name).FontSize(13).FontWeight(700).SingleLine()
								ui.Textf(c, "%d′", ar.Minutes).FontSize(12).FontWeight(600).
									FontFeatures("tnum").TextColor(lo)
							})
							if pill.Clicked() {
								a.searchFor(route{page: pageSearch, query: ar.Name, kind: netease.SearchArtists})
							}
						}
					})
				}

				// The songs first heard this week — not in the seven days
				// before it — each a small door into the new.
				if len(news) > 0 {
					ui.Text(c, "本周新发现").FontSize(12).FontWeight(700).TextColor(faint).Margin(24, 0, 10, 0)
					ui.Column(c).MaxWidth(560).Gap(4).Children(func() {
						for _, t := range news {
							key := "new-" + t.Track.Pic
							if t.Track.Pic == "" {
								key = "new-" + t.Track.Name
							}
							row := ui.Row(c.Key(key)).Fill().Height(52).Padding(0, 10).Gap(12).
								AlignItems(ui.Center).Radius(10).Label("周报新歌 " + t.Track.Name)
							hovered := row.Hovered()
							if hovered {
								row.Background(a.pal.hover)
							}
							row.Children(func() {
								ui.Box(c).Size(22, 22).Radius(11).Background(a.pal.accent).
									AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
									Children(func() {
										ui.Text(c, "新").FontSize(9).FontWeight(800).TextColor(white)
									})
								cover := ui.Box(c.Key("cover")).Size(38, 38).Radius(7).Background(a.pal.hover).Shrink(0)
								bmp := bitmap(a.art.get(netease.Sized(t.Track.Pic, 160), false))
								in := artIn(cover, bmp)
								cover.Children(func() { artImage(c, bmp, 7, in) })
								ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
									ui.Text(c, t.Track.Name).FontSize(13.5).FontWeight(700).SingleLine()
									if t.Track.Artist != "" {
										ui.Text(c, t.Track.Artist).FontSize(11.5).TextColor(lo).SingleLine()
									}
								})
								if hovered {
									ui.Box(c).Size(26, 26).Radius(13).Background(a.pal.accent).
										AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
										Children(func() {
											ui.Icon(c, iconPlay).Size(12, 12).TextColor(white)
										})
								}
								ui.Textf(c, "%d 分钟", t.Minutes).FontSize(12).FontWeight(600).
									FontFeatures("tnum").TextColor(lo)
							})
							if row.Clicked() {
								a.pl.PlayQueue([]netease.Track{t.song()}, 0)
							}
						}
					})
				}

				// The fortnight behind, drawn as a trend: the week's minutes a
				// day, the line rising and falling across fourteen of them, the
				// fullest day marked, and today a dot at its end.
				ui.Text(c, "近两周").FontSize(12).FontWeight(700).TextColor(faint).Margin(28, 0, 10, 0)
				chart := ui.Box(c.Key("fortnight")).Fill().Height(92).Radius(12).
					Background(a.pal.hover)
				chart.Draw(func(p *ui.Painter, rect ui.Rect) {
					data := st.Fortnight
					var top float64
					for _, v := range data {
						if v > top {
							top = v
						}
					}
					if top <= 0 {
						return
					}
					pad := float32(12)
					x0 := rect.X + pad
					w := rect.W - pad*2
					baseY := rect.Y + rect.H - pad
					h := rect.H - pad*2
					n := len(data)
					xs := func(i int) float32 { return x0 + float32(i)*w/float32(n-1) }
					ys := func(v float64) float32 { return baseY - float32(v/top)*h }
					// The area it swept, and the line along its top.
					area := &ui.Path{}
					area.MoveTo(xs(0), baseY)
					for i := range data {
						area.LineTo(xs(i), ys(data[i]))
					}
					area.LineTo(xs(n-1), baseY)
					area.Close()
					p.FillPath(area, tint.Alpha(0.16))
					line := make([][2]float32, n)
					for i := range data {
						line[i] = [2]float32{xs(i), ys(data[i])}
					}
					p.StrokePolyline(line, 2, tint)
					// The fullest day picked out, and today a dot at the end.
					var pk int
					for i := 1; i < n; i++ {
						if data[i] > data[pk] {
							pk = i
						}
					}
					if data[pk] > 0 {
						p.Fill(ui.Rect{X: xs(pk) - 3.5, Y: ys(data[pk]) - 3.5, W: 7, H: 7}, tint, 3.5)
					}
					p.Fill(ui.Rect{X: xs(n-1) - 3, Y: ys(data[n-1]) - 3, W: 6, H: 6}, white, 3)
				})
				var pk int
				var pkv float64
				for i, v := range st.Fortnight {
					if v > pkv {
						pk, pkv = i, v
					}
				}
				if pkv > 0 {
					d := now.AddDate(0, 0, -13+pk)
					ui.Textf(c, "%d月%d日最高 · %.0f 分钟", int(d.Month()), d.Day(), pkv).
						FontSize(11).FontWeight(600).TextColor(lo).Margin(6, 0, 0, 0)
				}
			})
			// The side rooms of the week: its covers, its clock, its
			// small numbers.
			a.weeklySide(c, r, st, tint, hi, lo, faint, now)
		})
	})
}

// weekSweep keeps the page's own little clocks: when the light began to
// cross the poster, and when the ring and the grid were born, so that
// each can sweep in once and then sit still.
type weekSweep struct {
	hover bool
	at    time.Time
	born  time.Time // the page's first frame, for the ring's sweep-in
	clock time.Time // the clock card's first frame, for its cascade
}

// weeklyHero is the week's poster: the cover of the song that carried it,
// held wide and faded into the dark at its left, the minutes it ran for
// in figures too big to miss, how that stands against the week before —
// and, at its right, the seven days drawn as a ring around the record.
func (a *app) weeklyHero(c *ui.Context, r weeklyReport, st weeklyStats) {
	if a.weekSky.born.IsZero() {
		a.weekSky.born = c.Now()
	}
	pic := ""
	top := 0
	var tracks []netease.Track
	if len(r.Tops) > 0 {
		pic, top = r.Tops[0].Track.Pic, r.Tops[0].Minutes
		for _, t := range r.Tops {
			tracks = append(tracks, t.song())
		}
	}
	// The same dark the FM's card wears, so that the two photographs
	// sit on the same ground.
	ground := ui.Hex("#1b1b20")
	// The cover, blurred wide: a photograph of the week rather than a
	// square of it, so that it can be held across the page.
	bmp := bitmap(a.art.get(netease.Sized(pic, 500), true))
	card := ui.Box(c.Key("week-hero")).Fill().Height(weekHeroH).Radius(24).
		Background(ground).TextColor(white).Shadow(0, 18, 44, -16, a.pal.cardShade)
	if card.Hovered() {
		if !a.weekSky.hover {
			a.weekSky.hover = true
			a.weekSky.at = c.Now()
		}
	} else {
		a.weekSky.hover = false
	}
	card.Children(func() {
		if bmp != nil {
			ui.Image(c, bmp).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover).Radius(24)
		}
		// The photograph fades into the card's own dark at its left,
		// where the figures stand, and keeps its weather at the right.
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(24).
			LinearGradient(ui.LinearGradient{Angle: 90, Oklab: true, Stops: []ui.GradientStop{
				{Color: ground.Alpha(0.96), At: 0},
				{Color: ground.Alpha(0.9), At: 0.22},
				{Color: ground.Alpha(0.72), At: 0.44},
				{Color: ground.Alpha(0.44), At: 0.64},
				{Color: ground.Alpha(0.16), At: 0.84},
				{Color: ground.Alpha(0), At: 1},
			}})
		// Its foot, so the figures stand off it.
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(24).
			LinearGradient(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
				{Color: ui.RGBA(0, 0, 0, 0), At: 0},
				{Color: ui.RGBA(0, 0, 0, 0.06), At: 0.5},
				{Color: ui.RGBA(0, 0, 0, 0.36), At: 1},
			}})
		// A clock of its own while the light crosses, and no longer.
		if !a.weekSky.at.IsZero() && c.Now().Sub(a.weekSky.at) <= time.Second {
			ui.Box(c).Size(1, 1).Loop("weeksweep", 900*time.Millisecond, ui.Linear)
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).
			Draw(func(p *ui.Painter, rect ui.Rect) {
				p.Clip(rect, 24, func() { lightSweep(p, rect, a.weekSky.at, c.Now()) })
			})
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(26, 34).Gap(30).
			AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Textf(c, "%s – %s · 本周听歌报告", weeklyKey(mondayOf(c.Now()))[5:], weeklyKey(c.Now())[5:]).
					FontSize(12.5).FontWeight(600).FontFeatures("tnum").TextColor(white.Alpha(0.7))
				ui.Row(c).AlignItems(ui.Baseline).Gap(12).Margin(2, 0, 0, 0).Children(func() {
					ui.Textf(c, "%d", r.Minutes).FontSize(64).FontWeight(800).FontFeatures("tnum").
						LetterSpacing(-2).TextColor(white)
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, "分钟").FontSize(14).FontWeight(700).TextColor(white.Alpha(0.9))
						ui.Textf(c, "%d 首歌曲", r.Songs).FontSize(12).TextColor(white.Alpha(0.55))
					})
				})
				if cmp := weeklyCompare(r); cmp != "" {
					ui.Text(c, cmp).FontSize(13).FontWeight(600).TextColor(white.Alpha(0.92))
				}
				if len(r.Tops) > 0 {
					share := 0
					if r.Minutes > 0 {
						share = int(float64(top)/float64(r.Minutes)*100 + 0.5)
					}
					ui.Textf(c, "本周最常听 · %s · 占 %d%%", r.Tops[0].Track.Name, share).
						FontSize(12).FontWeight(600).TextColor(white.Alpha(0.6)).SingleLine()
				}
				if len(tracks) > 0 {
					play := ui.Row(c.Key("week-play")).Margin(10, 0, 0, 0).Height(32).Padding(0, 16, 0, 16).Gap(6).
						AlignItems(ui.Center).Radius(999).Background(a.pal.accent).Label("播放本周精选")
					if play.Hovered() {
						play.Background(a.pal.accent.Mix(white, 0.14))
					}
					play.Children(func() {
						ui.Icon(c, iconPlay).Size(13, 13).TextColor(white)
						ui.Textf(c, "播放本周精选 %d 首", len(tracks)).FontSize(12.5).FontWeight(700).TextColor(white)
					})
					if play.Clicked() {
						a.pl.PlayQueue(tracks, 0)
					}
				}
			})
			if len(r.Tops) > 0 {
				a.weeklyRing(c, st, r.Tops[0].Track.Pic, tracks)
			}
		})
	})
}

// weeklyRing is the seven days of the week drawn around the record: a
// seventh of the circle a day, Monday at the top, each arc as thick as
// the day was long against the fullest one — the week as a dial, and the
// song it turned on at its middle.
func (a *app) weeklyRing(c *ui.Context, st weeklyStats, pic string, tracks []netease.Track) {
	const size = 168
	var top float64
	for _, v := range st.Days {
		if v > top {
			top = v
		}
	}
	peak := -1
	for d, v := range st.Days {
		if top > 0 && v == top {
			peak = d
		}
	}
	// The ring sweeps in once, from the top round the dial, and then
	// holds still — unless the listener asked for less motion.
	prog := float32(1)
	animating := false
	if !a.weekSky.born.IsZero() && !c.Preferences().ReduceMotion {
		if el := c.Now().Sub(a.weekSky.born).Seconds(); el < 0.9 {
			animating = true
			x := float32(el / 0.9)
			prog = 1 - (1-x)*(1-x)*(1-x)
		}
	}
	ring := ui.Box(c.Key("week-ring")).Size(size, size).Shrink(0)
	ring.Draw(func(p *ui.Painter, rect ui.Rect) {
		cx, cy := rect.X+rect.W/2, rect.Y+rect.H/2
		radius := rect.W/2 - 10
		sweep := 360 * prog
		for d := 0; d < 7; d++ {
			norm := float32(0)
			if top > 0 {
				norm = float32(st.Days[d] / top)
			}
			col, width := white.Alpha(0.1), float32(2)
			if norm > 0 {
				col, width = white.Alpha(0.18+0.55*norm), 2.5+7*norm
			}
			// A sliver of a gap between the days, so that the seams of
			// the week read as seams — and, while the ring is arriving,
			// only as much of each day as the light has swept past.
			a0 := -90 + float64(d)*(360.0/7) + 3
			a1 := -90 + float64(d+1)*(360.0/7) - 3
			end := a1
			if sweep < 360 {
				end = math.Min(a1, -90+float64(sweep))
			}
			if end <= a0 {
				continue
			}
			pts := make([][2]float32, 0, 18)
			for ang := a0; ang <= end+0.1; ang += 3 {
				rad := ang * math.Pi / 180
				pts = append(pts, [2]float32{
					cx + radius*float32(math.Cos(rad)),
					cy + radius*float32(math.Sin(rad)),
				})
			}
			if norm > 0 {
				// A dark pass beneath, so the arc holds its own against
				// the poster's brightest weather, and a soft halo round
				// the day the week leaned on.
				p.StrokePolyline(pts, width+4, ui.RGBA(0, 0, 0, 0.42))
				if d == peak {
					p.StrokePolyline(pts, width+8, white.Alpha(0.22))
				}
			}
			p.StrokePolyline(pts, width, col)
		}
	})
	ring.AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		if animating {
			ui.Box(c).Size(1, 1).Loop("weekring", 900*time.Millisecond, ui.Linear)
		}
		a.weeklyDisc(c, pic, 112)
	})
	if len(tracks) > 0 && ring.Clicked() {
		a.pl.PlayQueue(tracks, 0)
	}
}

// weeklyHours is the week as the clock saw it, drawn as wide as the page:
// a cell for every hour of every day, Monday at the top and midnight at
// the left, the brightest the hour the week leaned on; the seven days
// folded into one bar an hour underneath, so that a quiet week still
// keeps its shape; and the week's own sentence about it at the foot.
func (a *app) weeklyHours(c *ui.Context, r weeklyReport, now time.Time, tint, hi, faint ui.Color) {
	var grid [7][24]float64
	if a.weekly != nil {
		grid = a.weekly.weekClock(now)
	}
	var top float64
	peakDay, peakHour := 0, 0
	for d := range grid {
		for h, v := range grid[d] {
			if v > top {
				top, peakDay, peakHour = v, d, h
			}
		}
	}
	short := []string{"一", "二", "三", "四", "五", "六", "日"}
	long := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}

	// The cells light up left to right, once, in the order a day is read.
	if a.weekSky.clock.IsZero() {
		a.weekSky.clock = c.Now()
	}
	animating := false
	elapsed := 0.0
	if !c.Preferences().ReduceMotion {
		elapsed = c.Now().Sub(a.weekSky.clock).Seconds()
		if elapsed < 1.35 {
			animating = true
		}
	}
	// cellP is how far hour h has arrived, from 0 to 1.
	cellP := func(h int) float32 {
		if !animating {
			return 1
		}
		t := (elapsed - float64(h)*0.032) / 0.4
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		return 1 - (1-float32(t))*(1-float32(t))
	}

	card := ui.Box(c.Key("week-hours")).Fill().Margin(20, 0, 0, 0).Padding(18, 20).Radius(18).
		Background(a.pal.hover).Border(1, a.pal.hairline)
	card.Children(func() {
		if animating {
			ui.Box(c).Size(1, 1).Loop("weekclock", 1400*time.Millisecond, ui.Linear)
		}
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "一周的时钟").FontSize(12).FontWeight(700).TextColor(faint)
			ui.Box(c).Grow(1)
			if top > 0 {
				ui.Textf(c, "%s %d 点最投入 · %d 分钟", long[peakDay], peakHour,
					int(grid[peakDay][peakHour]/60+0.5)).FontSize(11.5).FontWeight(600).
					FontFeatures("tnum").TextColor(faint)
			}
		})
		ui.Column(c).Gap(3).Margin(12, 0, 0, 0).Children(func() {
			for d := 0; d < 7; d++ {
				ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(20).Shrink(0).Children(func() {
						ui.Text(c, short[d]).FontSize(10.5).FontWeight(600).TextColor(faint)
					})
					ui.Row(c).Grow(1).Gap(2).Children(func() {
						for h := 0; h < 24; h++ {
							v := grid[d][h]
							rel := float32(0)
							if top > 0 {
								rel = float32(v / top)
							}
							p := cellP(h)
							cell := ui.Box(c.Key(h)).Grow(1).Height(16).Radius(4)
							switch {
							case top <= 0 || v <= 0:
								// Quieter than the hairline itself, so that
								// a week barely begun reads as quiet, not
								// as broken.
								cell.Background(a.pal.hairline.Alpha(0.55 * p))
							case d == peakDay && h == peakHour:
								cell.Background(hi.Alpha(p))
							default:
								cell.Background(tint.Alpha((0.16 + 0.66*rel) * p))
							}
							cell.Label(fmt.Sprintf("%s %d 点 · %d 分钟", long[d], h, int(v/60+0.5)))
						}
					})
				})
			}
		})
		ui.Row(c).Gap(3).Margin(4, 0, 0, 0).Children(func() {
			ui.Box(c).Width(20).Shrink(0)
			ui.Row(c).Grow(1).Justify(ui.SpaceBetween).Children(func() {
				for _, h := range []string{"0", "3", "6", "9", "12", "15", "18", "21", "24"} {
					ui.Text(c, h).FontSize(10).FontWeight(600).FontFeatures("tnum").TextColor(faint)
				}
			})
		})
		// The rhythm of the day: all seven days folded into a bar an
		// hour, standing on the same grid, so that the shape of when
		// the music ran shows even through a thin week.
		hourTotals := [24]float64{}
		hourTop := 0.0
		for h := 0; h < 24; h++ {
			for d := 0; d < 7; d++ {
				hourTotals[h] += grid[d][h]
			}
			if hourTotals[h] > hourTop {
				hourTop = hourTotals[h]
			}
		}
		if hourTop > 0 {
			ui.Row(c).Gap(3).Margin(7, 0, 0, 0).AlignItems(ui.End).Children(func() {
				ui.Box(c).Width(20).Shrink(0)
				ui.Row(c).Grow(1).Gap(2).AlignItems(ui.End).Children(func() {
					for h := 0; h < 24; h++ {
						rel := float32(hourTotals[h] / hourTop)
						p := cellP(h)
						bar := ui.Box(c.Key(h)).Grow(1).Height(max(rel*18, 3)).Radius(2)
						if h == peakHour {
							bar.Background(hi.Alpha(p))
						} else {
							bar.Background(tint.Alpha((0.2 + 0.5*rel) * p))
						}
						bar.Label(fmt.Sprintf("%d 点全天 · %d 分钟", h, int(hourTotals[h]/60+0.5)))
					}
				})
			})
		}
		if r.Story != "" {
			ui.Text(c, r.Story).FontSize(13).FontWeight(700).TextColor(hi).Margin(12, 0, 0, 0)
		}
	})
}

// weeklySide is the week's right-hand column: the covers it ran to as a
// small wall, and the numbers the totals alone don't say.
func (a *app) weeklySide(c *ui.Context, r weeklyReport, st weeklyStats, tint, hi, lo, faint ui.Color, now time.Time) {
	ui.Column(c).Width(278).Shrink(0).Gap(24).Children(func() {
		// The wall: the covers of the week, fullest first, nine at the
		// most, each a small square of the week it carried.
		wt := a.weekly.weekTracks(now)
		if len(wt) > 0 {
			ui.Text(c, "封面墙").FontSize(12).FontWeight(700).TextColor(faint)
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for i, t := range wt {
					if i == 9 {
						break
					}
					// Keyed by its cover, not by its place: wherever
					// the song sits in the order, its square is its
					// own, and its cover does not dance.
					key := t.Track.Pic
					if key == "" {
						key = t.Track.Name
					}
					sq := ui.Box(c.Key(key)).Size(86, 86).Radius(10).Background(a.pal.hover).Shrink(0)
					sq.Label(t.Track.Name + " · " + t.Track.Artist)
					bmp := bitmap(a.art.get(netease.Sized(t.Track.Pic, 160), false))
					in := artIn(sq, bmp)
					sq.Children(func() { artImage(c, bmp, 10, in) })
				}
			})
		}

		// The small numbers: the songs and singers of the week, the
		// hours it woke and slept, the day it leaned on.
		first, last := -1, -1
		for h, v := range st.Hours24 {
			if v <= 0 {
				continue
			}
			if first < 0 {
				first = h
			}
			last = h
		}
		singers := map[string]bool{}
		for _, t := range wt {
			for _, name := range strings.Split(t.Track.Artist, "/") {
				if name = strings.TrimSpace(name); name != "" {
					singers[name] = true
				}
			}
		}
		wd := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
		facts := [][2]string{
			{"听到歌手", fmt.Sprintf("%d 位", len(singers))},
		}
		if first >= 0 {
			facts = append(facts,
				[2]string{"最早一次", fmt.Sprintf("%d 点", first)},
				[2]string{"最晚一次", fmt.Sprintf("%d 点", last)},
				[2]string{"最投入的一天", fmt.Sprintf("%s · %d 分钟", wd[st.BestDayWd], st.BestDayMin)},
			)
		}
		ui.Column(c).Gap(10).Padding(14, 16, 14, 16).Radius(14).Background(a.pal.hover).Children(func() {
			for _, f := range facts {
				ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					ui.Text(c, f[0]).FontSize(12).TextColor(faint)
					ui.Text(c, f[1]).FontSize(12.5).FontWeight(700).FontFeatures("tnum").TextColor(a.pal.text)
				})
			}
		})
	})
}

// mondayOf is the Monday at the head of now's week.
func mondayOf(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(int(now.Weekday())+6)%7)
}

// weeklyCompare stands this week against the last, as the listener says
// it. Nothing to stand against, and it says the week is the first.
func weeklyCompare(r weeklyReport) string {
	switch {
	case r.LastWeekMinutes <= 0:
		return "这是你被记录的第一周"
	case r.Minutes > r.LastWeekMinutes:
		return fmt.Sprintf("比上周多听了 %d 分钟", r.Minutes-r.LastWeekMinutes)
	case r.Minutes < r.LastWeekMinutes:
		return fmt.Sprintf("比上周少听了 %d 分钟", r.LastWeekMinutes-r.Minutes)
	default:
		return "和上周持平"
	}
}

// weeklyDisc is the song of the week on a record: the cover for a label,
// turning slowly, whatever plays — the week's own, spinning whether or
// not the room is silent.
func (a *app) weeklyDisc(c *ui.Context, pic string, d float32) {
	platter := ui.Box(c.Key("weekly-disc")).Size(d, d).Radius(d/2).Background(ui.Hex("#101013")).
		Shadow(0, 24, 48, -10, a.pal.cardShade)
	platter.Children(func() {
		rim := d - 3
		ui.Box(c.Key("rim")).Size(rim, rim).Radius(rim/2).Border(1, white.Alpha(0.1)).
			Absolute().Top(1.5).Left(1.5)
		disc := ui.Box(c.Key("record")).Absolute().Top(0).Left(0).Size(d, d)
		if !c.Preferences().ReduceMotion {
			disc.TransformOrigin(0.5, 0.5).Rotate(disc.Loop("wspin", vinylRev, ui.Linear) * 360)
		}
		disc.Children(func() {
			for i, k := range []float32{0.475, 0.45, 0.425, 0.40, 0.375, 0.35, 0.325, 0.30, 0.275} {
				gd := d * k * 2
				ui.Box(c.Key(i)).Size(gd, gd).Radius(gd/2).Border(1, white.Alpha(0.05)).
					Absolute().Top((d - gd) / 2).Left((d - gd) / 2)
			}
			ld := d * 0.52
			label := ui.Box(c.Key("label")).Size(ld, ld).Radius(ld / 2).Background(white.Alpha(0.08)).
				Absolute().Top((d - ld) / 2).Left((d - ld) / 2)
			bmp := bitmap(a.art.get(netease.Sized(pic, 300), false))
			if bmp == nil {
				label.Background(ui.RGB(70, 70, 78))
			}
			in := artIn(label, bmp)
			label.Children(func() { artImage(c, bmp, ld/2, in) })
			hd := max(d*0.032, 5)
			ui.Box(c.Key("spindle")).Size(hd, hd).Radius(hd / 2).Background(ui.Hex("#0b0b0d")).
				Absolute().Top((d - hd) / 2).Left((d - hd) / 2)
		})
	})
}

// weeklyStory names the part of the day the week ran at, from its hours:
// the hour that carried the most, as the listener says it — the small
// hours, the evening, the afternoon.
func weeklyStory(perHour map[int]float64) string {
	var best int
	var top float64
	for h, v := range perHour {
		if v > top {
			best, top = h, v
		}
	}
	if top <= 0 {
		return ""
	}
	switch {
	case best >= 5 && best < 11:
		return "这一周，你在早晨听得最多"
	case best >= 11 && best < 14:
		return "这一周，你在午间听得最多"
	case best >= 14 && best < 18:
		return "这一周，你在下午听得最多"
	case best >= 18 && best < 23:
		return "这一周，你在夜晚听得最多"
	default:
		return "这一周，你在深夜听得最多"
	}
}

// weeklyStats gathers the shapes of the week that the totals alone don't
// tell: the hour of the day it ran at, the fortnight behind it, the days
// it has run without pause, and how a day of it sits against the average.
type weeklyStats struct {
	Days       [7]float64  // minutes a day, Monday first
	Hours24    [24]float64 // seconds per hour of the day, this week
	Fortnight  [14]float64 // minutes per day, the fortnight ending today
	Streak     int         // the days it has run without pause, today in
	BestDayMin int         // the fullest day of the week, in minutes
	BestDayWd  int         // which weekday that was, Monday nought
	AvgDay     float64     // the days that have run, shared evenly
}

// stats gathers those shapes from the record.
func (w *weeklyState) stats(now time.Time) weeklyStats {
	var s weeklyStats
	if w == nil {
		return s
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	off := (int(now.Weekday()) + 6) % 7
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	var weekTop float64
	var weekSecs float64
	for i := 0; i < 7; i++ {
		day := weeklyKey(monday.AddDate(0, 0, i))
		var secs float64
		for _, v := range w.data.Days[day] {
			secs += v
		}
		weekSecs += secs
		s.Days[i] = secs / 60
		if secs > weekTop {
			weekTop = secs
			s.BestDayMin = int(secs/60 + 0.5)
			s.BestDayWd = i
		}
		for h, v := range w.data.Hours[day] {
			s.Hours24[h] += v
		}
	}
	s.AvgDay = weekSecs / 60 / float64(off+1)
	for i := 0; i < 14; i++ {
		day := weeklyKey(now.AddDate(0, 0, -13+i))
		var secs float64
		for _, v := range w.data.Days[day] {
			secs += v
		}
		s.Fortnight[i] = secs / 60
	}
	// The streak runs back from today — or from yesterday, where today
	// has not yet begun — over the days the record heard something.
	start := now
	if s.Fortnight[13] <= 0 {
		start = now.AddDate(0, 0, -1)
	}
	for d := start; s.Streak < 14; d = d.AddDate(0, 0, -1) {
		var secs float64
		for _, v := range w.data.Days[weeklyKey(d)] {
			secs += v
		}
		if secs <= 0 {
			break
		}
		s.Streak++
	}
	return s
}

// newThisWeek are the songs first heard in the week now opens: in the days
// from Monday to now, but not in the seven before — the week's own finds,
// fullest first, four at the most.
func (w *weeklyState) newThisWeek(now time.Time) []weeklyTop {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	off := (int(now.Weekday()) + 6) % 7
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	last := monday.AddDate(0, 0, -7)
	this := map[int64]bool{}
	before := map[int64]bool{}
	for i := 0; i < 7; i++ {
		for id := range w.data.Days[weeklyKey(monday.AddDate(0, 0, i))] {
			this[id] = true
		}
		for id := range w.data.Days[weeklyKey(last.AddDate(0, 0, i))] {
			before[id] = true
		}
	}
	perSong := map[int64]float64{}
	for id := range this {
		if before[id] {
			continue
		}
		for i := 0; i < 7; i++ {
			perSong[id] += w.data.Days[weeklyKey(monday.AddDate(0, 0, i))][id]
		}
	}
	ids := make([]int64, 0, len(perSong))
	for id := range perSong {
		if w.data.Tracks[id].Name != "" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if perSong[ids[i]] != perSong[ids[j]] {
			return perSong[ids[i]] > perSong[ids[j]]
		}
		return ids[i] < ids[j]
	})
	if len(ids) > 4 {
		ids = ids[:4]
	}
	tops := make([]weeklyTop, 0, len(ids))
	for _, id := range ids {
		tops = append(tops, weeklyTop{ID: id, Track: w.data.Tracks[id], Minutes: int(perSong[id]/60 + 0.5)})
	}
	return tops
}

// weeklyChip is one small card of the week at a glance: a figure big
// enough to read across the room, and what the figure counts.
func (a *app) weeklyChip(c *ui.Context, big, label string) {
	ui.Column(c).Grow(1).Gap(2).Padding(12, 14, 12, 14).Radius(12).
		Background(a.pal.hover).Children(func() {
		ui.Textf(c, "%s", big).FontSize(22).FontWeight(800).FontFeatures("tnum").
			TextColor(a.pal.text)
		ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(a.pal.faint)
	})
}

// weekClock is the week as the clock saw it: a cell a day and hour,
// Monday first, the seconds each cell carried.
func (w *weeklyState) weekClock(now time.Time) [7][24]float64 {
	var g [7][24]float64
	if w == nil {
		return g
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	off := (int(now.Weekday()) + 6) % 7
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	for i := 0; i < 7; i++ {
		day := weeklyKey(monday.AddDate(0, 0, i))
		for h, v := range w.data.Hours[day] {
			g[i][h] = v
		}
	}
	return g
}

// weekTracks is the week's songs, fullest first, a dozen at most.
func (w *weeklyState) weekTracks(now time.Time) []weeklyTop {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	off := (int(now.Weekday()) + 6) % 7
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	perSong := map[int64]float64{}
	for i := 0; i < 7; i++ {
		for id, secs := range w.data.Days[weeklyKey(monday.AddDate(0, 0, i))] {
			perSong[id] += secs
		}
	}
	ids := make([]int64, 0, len(perSong))
	for id := range perSong {
		if w.data.Tracks[id].Name != "" {
			ids = append(ids, id)
		}
	}
	// Fullest first, and where two ran the same, the smaller id first:
	// the order must not wander, or the covers dance.
	sort.Slice(ids, func(i, j int) bool {
		if perSong[ids[i]] != perSong[ids[j]] {
			return perSong[ids[i]] > perSong[ids[j]]
		}
		return ids[i] < ids[j]
	})
	if len(ids) > 12 {
		ids = ids[:12]
	}
	tops := make([]weeklyTop, 0, len(ids))
	for _, id := range ids {
		tops = append(tops, weeklyTop{Track: w.data.Tracks[id], Minutes: int(perSong[id]/60 + 0.5)})
	}
	return tops
}
