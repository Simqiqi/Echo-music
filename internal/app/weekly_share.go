package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// reportPoster is a week's or a month's listening, laid out for a card:
// the totals, the shape of its days, what it heard most, and the cover of
// the song that led it carried across the ground.
type reportPoster struct {
	name  string
	dates string
	days  int
	r     weeklyReport
	byDay []float64
	cover *ui.Bitmap
	pic   string
	ink   ui.Color
}

// shareReport copies a poster of the last days of listening, a week or a
// month, to paste into a chat or keep as a picture.
func (a *app) shareReport(days int) {
	if a.weekly == nil {
		a.toast = "还没有听歌记录"
		a.redraw()
		return
	}
	now := time.Now()
	r := a.weekly.periodReport(now, days)
	if r.Songs == 0 {
		a.toast = "这段时间还没有听歌"
		a.redraw()
		return
	}
	p := a.reportPosterOf(now, days, r)
	if p.cover != nil || p.pic == "" {
		a.writeReportCard(p)
		return
	}
	// The cover is not cached yet: fetch it, then draw on the main
	// thread, where the renderer lives.
	url := netease.Sized(p.pic, 600)
	go func() {
		b, err := netease.Fetch(url)
		mygo.RunOnMain(func() {
			if err == nil {
				if bmp, derr := ui.DecodeBitmap(b); derr == nil {
					p.cover = bmp
				}
			}
			a.writeReportCard(p)
		})
	}()
}

func (a *app) reportPosterOf(now time.Time, days int, r weeklyReport) reportPoster {
	name := "这一周"
	if days > 7 {
		name = "最近 30 天"
	}
	p := reportPoster{
		name:  name,
		dates: now.AddDate(0, 0, -(days-1)).Format("01.02") + " – " + now.Format("01.02"),
		days:  days,
		r:     r,
		byDay: a.weekly.periodDays(now, days),
		ink:   a.pal.accent,
	}
	if len(r.Tops) > 0 {
		p.pic = r.Tops[0].Track.Pic
	}
	if p.pic != "" {
		if art := a.art.get(netease.Sized(p.pic, 600), false); art != nil && !art.failed {
			p.cover, p.ink = art.bmp, art.tint
		}
	}
	return p
}

func (a *app) writeReportCard(p reportPoster) {
	done := "周报海报已复制，粘贴即可分享"
	if p.days > 7 {
		done = "月报海报已复制，粘贴即可分享"
	}
	a.writeCard(func(c *ui.Context) { a.reportCard(c, p) }, done)
}

// reportCard draws the poster: the cover dimmed behind it, the minutes in
// figures too big to miss, the days as bars, and what the span heard most.
func (a *app) reportCard(c *ui.Context, p reportPoster) {
	ink := p.ink
	ui.Column(c).Size(cardW, cardH).Clip().Children(func() {
		cardBackdrop(c, p.cover, ink)
		// The backdrop is a stack of absolute layers; the words must be
		// one too, or they are painted beneath it.
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).
			Padding(40, 46, 36, 46).Gap(0).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(ink.Mix(white, 0.3))
				ui.Text(c, "Echo云音乐 · 听歌回顾").FontSize(11.5).TextColor(white.Alpha(0.5))
			})
			ui.Text(c, p.name).FontSize(36).FontWeight(800).TextColor(white).Margin(18, 0, 0, 0)
			ui.Text(c, p.dates).FontSize(12.5).TextColor(white.Alpha(0.5)).Margin(3, 0, 0, 0)

			ui.Text(c, fmt.Sprintf("%d", p.r.Minutes)).FontSize(54).FontWeight(800).
				TextColor(white).Margin(18, 0, 0, 0)
			ui.Text(c, fmt.Sprintf("分钟 · 共 %d 首歌", p.r.Songs)).FontSize(13).
				TextColor(white.Alpha(0.7))
			if p.r.Story != "" {
				ui.Text(c, p.r.Story).FontSize(12.5).MaxLines(1).
					TextColor(white.Alpha(0.55)).Margin(6, 0, 0, 0)
			}

			// What the span's days were like, in words: this absolute
			// column gives drawn boxes no width, but text always shows.
			peak, active := 0.0, 0
			for _, m := range p.byDay {
				peak = max(peak, m)
				if m > 0 {
					active++
				}
			}
			ui.Text(c, fmt.Sprintf("%d 天有音乐 · 最满的一天 %d 分钟", active, int(peak+0.5))).
				FontSize(13).TextColor(white.Alpha(0.8)).Margin(16, 0, 0, 0)

			if len(p.r.Tops) > 0 {
				ui.Text(c, "最常听的歌").FontSize(12).FontWeight(700).SingleLine().
					TextColor(white.Alpha(0.8)).Margin(18, 0, 0, 0)
				for i, t := range p.r.Tops {
					if i == 3 {
						break
					}
					ui.Row(c).Gap(10).AlignItems(ui.Center).Margin(8, 0, 0, 0).Children(func() {
						ui.Textf(c, "%d", i+1).FontSize(12).TextColor(ink.Mix(white, 0.4)).Width(16)
						ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() {
							ui.Text(c, t.Track.Name).FontSize(14).FontWeight(600).SingleLine().TextColor(white)
							ui.Text(c, t.Track.Artist).FontSize(11.5).SingleLine().TextColor(white.Alpha(0.5))
						})
						ui.Text(c, fmt.Sprintf("%d′", t.Minutes)).FontSize(12).TextColor(white.Alpha(0.7))
					})
				}
			}
			if len(p.r.Artists) > 0 {
				ui.Text(c, "陪你最久的歌手").FontSize(12).FontWeight(700).SingleLine().
					TextColor(white.Alpha(0.8)).Margin(16, 0, 0, 0)
				ui.Row(c).Gap(8).Margin(8, 0, 0, 0).Children(func() {
					for _, ar := range p.r.Artists {
						ui.Box(c).Radius(999).Background(white.Alpha(0.12)).Padding(6, 12).Children(func() {
							ui.Text(c, ar.Name).FontSize(12).TextColor(white)
						})
					}
				})
			}
			ui.Box(c).Grow(1)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(ink.Mix(white, 0.3))
				ui.Text(c, "Echo云音乐").FontSize(11.5).TextColor(white.Alpha(0.45))
			})
		})
	})
}
