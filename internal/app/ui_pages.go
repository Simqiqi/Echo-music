package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
)

const pagePad = 40

// columns returns how many cards of at least min DIPs fit across the page.
func (a *app) columns(c *ui.Context, min float32) int {
	w, _ := c.Size()
	inner := w - sidebarWidth - 2*pagePad
	return max(2, int((inner+20)/(min+20)))
}

func (a *app) heading(c *ui.Context, title, sub string) {
	ui.Column(c).Gap(4).Margin(0, 0, 18, 0).Children(func() {
		if sub != "" {
			ui.Text(c, sub).FontSize(12).FontWeight(600).TextColor(a.pal.muted).LetterSpacing(0.4)
		}
		ui.Text(c, title).FontSize(30).FontWeight(700).LetterSpacing(-0.6)
	})
}

// section is the name of a stretch of a page: once it has scrolled under
// the capsule over the page, the capsule takes it up.
func (a *app) section(c *ui.Context, title string) ui.Element {
	el := ui.Text(c, title).FontSize(19).FontWeight(700).LetterSpacing(-0.3).Margin(34, 0, 14, 0)
	if b := el.Bounds(); b.W > 0 && b.Y+b.H <= a.barLine(c) {
		a.navTitle = title
	}
	return el
}

func (a *app) homePage(c *ui.Context) {
	loading := len(a.featured) == 0 && len(a.charts) == 0
	if loading && !a.homeLoading {
		a.centered(c, a.homeErr, a.loadHome)
		return
	}
	now := c.Now()
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("home")).TrackScroll(&a.pageScroll).Grow(1).Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		hello := greeting(now)
		if a.profile != nil {
			hello += "，" + a.profile.Nickname
		}
		a.heading(c, hello, fmt.Sprintf("%d月%d日 %s", now.Month(), now.Day(), weekday(now)))
		if loading {
			a.homeSkeleton(c, a.pageShimmer(c))
			return
		}
		a.dailyCard(c, now)

		if heard := a.weekly.recent(now, 6); len(heard) > 0 {
			a.section(c, "最近在听")
			a.recentRow(c, heard)
		}

		if len(a.recPlaylists) > 0 {
			a.section(c, "今日为你推荐")
			a.pickGrid(c)
		}

		a.section(c, "官方榜单")
		official := a.charts[:min(4, len(a.charts))]
		cols := min(4, max(2, a.columns(c, 168)))
		ui.Grid(c).Columns(cols).GapX(20).GapY(28).Children(func() {
			for i := range official {
				a.chartCardCompact(c.Key(official[i].ID), &official[i])
			}
		})

		a.section(c, "推荐歌单")
		ui.Grid(c).Columns(a.columns(c, 168)).GapX(20).GapY(28).Children(func() {
			for i := range a.featured {
				a.playlistCard(c.Key(a.featured[i].ID), &a.featured[i])
			}
		})
	})
}

func weekday(t time.Time) string {
	return [...]string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}[t.Weekday()]
}

func (a *app) chartsPage(c *ui.Context) {
	loading := len(a.charts) == 0
	if loading && !a.homeLoading {
		a.centered(c, a.homeErr, a.loadHome)
		return
	}
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("charts")).TrackScroll(&a.pageScroll).Grow(1).Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		a.heading(c, "排行榜", "每日更新")
		if loading {
			a.chartsSkeleton(c, a.pageShimmer(c))
			return
		}
		// The first chart runs wide as the page's one banner, its cover
		// showing through; the rest sit below as cards washed in their
		// own covers' colors, each with the three songs at its top.
		a.chartHero(c, &a.charts[0])
		if rest := a.charts[1:]; len(rest) > 0 {
			a.section(c, "全部榜单")
			ui.Grid(c).Columns(a.columns(c, 330)).GapX(20).GapY(14).Children(func() {
				for i := range rest {
					a.chartTile(c.Key(rest[i].ID), &rest[i])
				}
			})
		}
	})
}

// playlistCard is a square cover with the playlist's name below: it lifts
// under the pointer, where a play button shows on it.
func (a *app) playlistCard(c *ui.Context, pl *netease.Playlist) {
	if a.coverCard(c, pl.Name, pl.Cover(), pl.PlayCount, pl.UpdateFreq, func() { a.loadPlaylist(pl.ID, true) }) {
		a.open(route{page: pagePlaylist, id: pl.ID})
	}
}

// pickGrid is the day's picks: the first of them large, as the page's one
// hero, and the rest of the first two rows laid around it.
func (a *app) pickGrid(c *ui.Context) {
	cols := a.columns(c, 168)
	// The hero holds the first two columns of both rows, so the grid
	// takes two fewer cards than it had room for.
	n := min(len(a.recPlaylists), max(1, cols*2-4))
	ui.Grid(c).Columns(cols).GapX(20).GapY(28).Children(func() {
		a.focusCard(c.Key(a.recPlaylists[0].ID), &a.recPlaylists[0]).ColumnSpan(2).RowSpan(2)
		for i := 1; i < n; i++ {
			a.playlistCard(c.Key(a.recPlaylists[i].ID), &a.recPlaylists[i])
		}
	})
}

// focusCard is the first of the day's picks, large: it wears a landscape,
// another each time the page opens — clear at its left, dissolving into
// the card's dark by its right — with its name and its numbers over the
// foot, a mark saying what it is at the head. The button on it plays it;
// the rest opens it.
func (a *app) focusCard(c *ui.Context, pl *netease.Playlist) ui.Element {
	card := ui.ButtonBase(c).Fill().Radius(16).FocusRing(false).Label(pl.Name).TextColor(white).
		Background(ui.Hex("#20242b"))
	hovered := card.Hovered()
	pose := ui.Pose{}
	if hovered {
		pose.Scale = 1.012
	}
	card.AnimatePose(pose, ui.Snappy)
	// The landscape, another each time the page opens; it shows as soon
	// as it has loaded, which asks for a frame of its own.
	bmp := a.sky.bmp(a)
	base := ui.Hex("#20242b")
	if a.sky.url == "" {
		a.sky.pick(a, a.post)
		bmp = a.sky.bmp(a)
	}
	// The light passes over it as the pointer comes onto it, once each
	// time it does; the sky keeps the times, and only one card shows.
	if hovered {
		if !a.sky.hover {
			a.sky.hover = true
			a.sky.sweepAt = c.Now()
		}
	} else {
		a.sky.hover = false
	}
	card.Children(func() {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Clip(r, 16, func() {
				if bmp != nil {
					p.Image(bmp, r, ui.Cover)
				}
				// The landscape stands clear at the left and dissolves,
				// by the right, into the card's own dark: the way the FM's
				// card fades, turned about — the words keep their ground
				// and the photograph keeps its weather.
				p.FillGradient(r, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
					{At: 0, Color: base.Alpha(0)},
					{At: 0.18, Color: base.Alpha(0.08)},
					{At: 0.38, Color: base.Alpha(0.2)},
					{At: 0.56, Color: base.Alpha(0.42)},
					{At: 0.74, Color: base.Alpha(0.7)},
					{At: 0.9, Color: base.Alpha(0.92)},
					{At: 1, Color: base.Alpha(1)},
				}}, 0)
				// Its foot, so the name stands off the photograph.
				p.FillGradient(r, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
					{At: 0, Color: ui.RGBA(0, 0, 0, 0)},
					{At: 0.45, Color: ui.RGBA(0, 0, 0, 0.1)},
					{At: 1, Color: ui.RGBA(0, 0, 0, 0.78)},
				}}, 0)
				lightSweep(p, r, a.sky.sweepAt, c.Now())
			})
		})
		// A clock of its own while the light crosses, and no longer.
		if !a.sky.sweepAt.IsZero() && c.Now().Sub(a.sky.sweepAt) <= time.Second {
			ui.Box(c).Size(1, 1).Loop("skysweep", 900*time.Millisecond, ui.Linear)
		}
		ui.Row(c).Absolute().Top(12).Left(12).Padding(4, 9).Radius(999).Gap(5).AlignItems(ui.Center).
			Background(ui.RGBA(0, 0, 0, 0.32)).Border(1, ui.RGBA(255, 255, 255, 0.14)).Children(func() {
			ui.Icon(c, iconDaily).Size(11, 11).TextColor(white.Alpha(0.9))
			ui.Text(c, "今日首选").FontSize(10.5).FontWeight(700).TextColor(white.Alpha(0.94))
		})
		ui.Column(c).Absolute().Left(16).Right(16).Bottom(14).Gap(5).Children(func() {
			ui.Text(c, pl.Name).FontSize(20).FontWeight(800).LetterSpacing(-0.4).MaxLines(2).LineHeight(1.22)
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				if pl.PlayCount > 0 {
					ui.Icon(c, iconHeadset).Size(11, 11).TextColor(white.Alpha(0.7))
					ui.Text(c, plays(pl.PlayCount)).FontSize(11.5).FontWeight(600).TextColor(white.Alpha(0.78))
				}
				if pl.TrackCount > 0 {
					ui.Box(c).Size(3, 3).Radius(1.5).Background(white.Alpha(0.4))
					ui.Textf(c, "%d 首", pl.TrackCount).FontSize(11.5).FontWeight(600).TextColor(white.Alpha(0.78))
				}
			})
		})
		a.coverPlayButton(c, hovered, func() { a.loadPlaylist(pl.ID, true) })
	})
	if card.Clicked() {
		a.open(route{page: pagePlaylist, id: pl.ID})
	}
	return card
}

// recentRow is what the app heard lately, laid compactly: a cover, the
// song's name, and who sings it — clicking any of them goes on with it.
func (a *app) recentRow(c *ui.Context, tracks []netease.Track) {
	p := a.pal
	ui.Grid(c).Columns(a.columns(c, 216)).GapX(16).GapY(4).Children(func() {
		for i := range tracks {
			tr, at := tracks[i], i
			card := ui.ButtonBase(c.Key(tr.ID)).Padding(8).Radius(12).Gap(11).AlignItems(ui.Center).
				FocusRing(false).Label(tr.Name)
			hovered := card.Hovered()
			fill := ui.Transparent
			if hovered {
				fill = p.hover
			}
			card.Background(card.AnimateColor("bg", fill, ui.Animation{Duration: 120 * time.Millisecond}))
			card.Children(func() {
				ui.Box(c).Size(44, 44).Radius(9).Shrink(0).Children(func() {
					a.coverFill(c, tr.Album.PicURL, 200, 9)
					if hovered {
						ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(9).
							Background(ui.RGBA(0, 0, 0, 0.36)).Center().Children(func() {
							ui.Icon(c, iconPlay).Size(14, 14).TextColor(white).Margin(0, 0, 0, 1)
						})
					}
				})
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, tr.Name).FontSize(13).FontWeight(600).SingleLine()
					ui.Text(c, tr.ArtistNames()).FontSize(11.5).TextColor(p.muted).SingleLine()
				})
			})
			if card.Clicked() {
				a.pl.PlayQueue(tracks, at)
			}
		}
	})
}

// chartCardCompact is a chart as the small card a row of them takes: its
// cover, its name, and how often it refreshes. The full card with its top
// three songs stays for the charts page, where a card has room.
func (a *app) chartCardCompact(c *ui.Context, pl *netease.Playlist) {
	if a.coverCard(c, pl.Name, pl.Cover(), 0, pl.UpdateFreq, func() { a.loadPlaylist(pl.ID, true) }) {
		a.open(route{page: pagePlaylist, id: pl.ID})
	}
}

// coverCard is a playlist's card, or a podcast's: its cover, with how many
// times it was played on it, over its name and a line under that. The
// button on the cover calls play; it reports whether the rest was clicked.
func (a *app) coverCard(c *ui.Context, name, cover string, playCount float64, sub string, play func()) bool {
	p := a.pal
	// From the top: a grid stretches the cards of a row as tall as the
	// tallest, whose name or line under it takes two lines.
	card := ui.ButtonBase(c).Column().Gap(10).AlignItems(ui.Stretch).Justify(ui.Start).FocusRing(false).Label(name)
	hovered := card.Hovered()
	card.Children(func() {
		art := ui.Box(c).AspectRatio(1).Radius(10)
		lift := ui.Pose{}
		if hovered {
			lift = ui.Pose{Y: -3}
			art.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			art.Shadow(0, 2, 8, -2, p.cardShade)
		}
		art.AnimatePose(lift, ui.Snappy)
		art.Children(func() {
			a.coverFill(c, cover, 400, 10)
			if playCount > 0 {
				ui.Row(c).Absolute().Top(8).Right(8).Padding(2, 7).Radius(999).Gap(3).AlignItems(ui.Center).
					Background(ui.RGBA(0, 0, 0, 0.35)).TextColor(ui.RGB(255, 255, 255)).Children(func() {
					ui.Icon(c, iconHeadset).Size(10, 10)
					ui.Text(c, plays(playCount)).FontSize(10.5).FontWeight(600)
				})
			}
			a.coverPlayButton(c, hovered, play)
		})
		ui.Text(c, name).FontSize(13).FontWeight(500).MaxLines(2).LineHeight(1.35)
		if sub != "" {
			ui.Text(c, sub).FontSize(12).TextColor(p.muted).SingleLine().Margin(-6, 0, 0, 0)
		}
	})
	return card.Clicked()
}

// coverArt draws a cover; having none, it draws a quiet color taken from
// the name, rather than a gray box.
func (a *app) coverArt(c *ui.Context, cover, name string, px int, radius float32) {
	if cover != "" {
		a.coverFill(c, cover, px, radius)
		return
	}
	a.coverTint(c, name, radius)
}

// playlistCover draws what a playlist looks like: its own cover, or, with
// none, the covers of the songs it holds in four, as a record of them.
func (a *app) playlistCover(c *ui.Context, pl *netease.Playlist, px int, radius float32) {
	if cover := pl.Cover(); cover != "" {
		a.coverFill(c, cover, px, radius)
		return
	}
	var covers []string
	for _, tr := range pl.Tracks {
		if tr.Album.PicURL == "" {
			continue
		}
		covers = append(covers, tr.Album.PicURL)
		if len(covers) == 4 {
			break
		}
	}
	if len(covers) < 4 {
		a.coverTint(c, pl.Name, radius)
		return
	}
	// Four covers, each rounded a little, with a seam between them: a
	// mosaic left round at its corners, which no clip has to cut.
	inner := max(0, radius-5)
	ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Gap(2).Children(func() {
		for r := 0; r < 2; r++ {
			ui.Row(c).Grow(1).Basis(0).Gap(2).Children(func() {
				for i := r * 2; i < r*2+2; i++ {
					ui.Box(c).Grow(1).Basis(0).Radius(inner).Children(func() {
						a.coverFill(c, covers[i], px, inner)
					})
				}
			})
		}
	})
}

// coverTint stands in for a cover an item has not: a quiet color of its
// own name, a note on it, so that a playlist without a picture still looks
// like something rather than a gap.
func (a *app) coverTint(c *ui.Context, name string, radius float32) {
	h := nameHue(name)
	from, to := hueTint(h, 0.30, 0.74), hueTint(h+24, 0.36, 0.56)
	if a.pal.dark {
		from, to = hueTint(h, 0.34, 0.44), hueTint(h+24, 0.40, 0.27)
	}
	box := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).Center()
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillGradient(r, ui.LinearGradient{Angle: 145, Oklab: true, Stops: []ui.GradientStop{
			{Color: from, At: 0}, {Color: to, At: 1},
		}}, radius)
	})
	box.Children(func() {
		n := max(13, radius*2.1)
		ui.Icon(c, iconNote).Size(n, n).TextColor(ui.RGBA(255, 255, 255, 0.5))
	})
}

// nameHue is where a name falls on the wheel, always the same place: two
// playlists of a name share a color, and neighbors rarely do.
func nameHue(name string) float32 {
	h := uint32(2166136261)
	for _, r := range name {
		h ^= uint32(r)
		h *= 16777619
	}
	return float32(h % 360)
}

// hueTint is the color at hue h, muted to saturation s and lightness l.
func hueTint(h, s, l float32) ui.Color {
	seg := math.Mod(math.Mod(float64(h), 360)+360, 360) / 60
	c := (1 - absf(2*l-1)) * s
	x := c * (1 - absf(float32(math.Mod(seg, 2))-1))
	switch int(seg) {
	case 0:
		return toneRGB(c, x, 0, l, c)
	case 1:
		return toneRGB(x, c, 0, l, c)
	case 2:
		return toneRGB(0, c, x, l, c)
	case 3:
		return toneRGB(0, x, c, l, c)
	case 4:
		return toneRGB(x, 0, c, l, c)
	default:
		return toneRGB(c, 0, x, l, c)
	}
}

func toneRGB(r, g, b, l, c float32) ui.Color {
	m := l - c/2
	to := func(v float32) uint8 { return uint8(min(1, max(0, v+m))*255 + 0.5) }
	return ui.RGB(to(r), to(g), to(b))
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// coverFill fills its parent with a cover, its corners rounded by radius.
// The cover rounds itself rather than its parent clipping it: a clip moved
// by a transform, as a card lifting under the pointer, is drawn on the CPU.
func (a *app) coverFill(c *ui.Context, url string, px int, radius float32) {
	bmp := bitmap(a.art.get(netease.Sized(url, px), false))
	box := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius)
	in := artIn(box, bmp)
	if in < 1 {
		box.Background(a.pal.hover)
	}
	box.Children(func() { artImage(c, bmp, radius, in) })
}

// coverPlayButton plays a playlist or a podcast from its card, without
// opening it.
func (a *app) coverPlayButton(c *ui.Context, shown bool, play func()) {
	b := ui.ButtonBase(c.Key("play")).Absolute().Bottom(10).Right(10).Size(38, 38).Radius(999).Center().
		Background(a.pal.accent).TextColor(ui.RGB(255, 255, 255)).Label("播放").FocusRing(false).
		Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.3))
	pose := ui.Pose{Fade: 1, Scale: 0.8, Y: 6}
	if shown || b.Hovered() {
		pose = ui.Pose{Scale: 1}
		if b.Hovered() {
			pose.Scale = 1.08
		}
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() { ui.Icon(c, iconPlay).Size(15, 15).Margin(0, 0, 0, 2) })
	if b.Clicked() {
		play()
	}
}

// chartHero leads the charts page: the day's first chart run wide across
// it, its cover showing through from the right and the three songs at its
// top beside the name. The button under it plays the chart; the rest of
// the banner opens it.
func (a *app) chartHero(c *ui.Context, pl *netease.Playlist) {
	const radius, height = 20, 224
	base := ui.Hex("#17181c")
	card := ui.ButtonBase(c).Height(height).Radius(radius).Clip().FocusRing(false).Label(pl.Name).Background(base)
	hovered := card.Hovered()
	pose := ui.Pose{}
	if hovered {
		pose.Scale = 1.004
	}
	card.AnimatePose(pose, ui.Snappy)
	bmp := bitmap(a.art.get(netease.Sized(pl.Cover(), 900), false))
	w, _ := c.Size()
	wide := w-sidebarWidth > 700
	card.Children(func() {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
			if bmp != nil {
				p.Image(bmp, r, ui.Cover)
			}
			// The words keep their ground at the left; the cover comes
			// clear by the right, with a last dark breath along the foot.
			p.FillGradient(r, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
				{At: 0, Color: base.Alpha(0.96)},
				{At: 0.34, Color: base.Alpha(0.9)},
				{At: 0.64, Color: base.Alpha(0.52)},
				{At: 1, Color: base.Alpha(0.2)},
			}}, radius)
			p.FillGradient(r, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
				{At: 0, Color: ui.RGBA(0, 0, 0, 0)},
				{At: 1, Color: ui.RGBA(0, 0, 0, 0.42)},
			}}, radius)
		})
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(22).Gap(26).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(6).Children(func() {
				ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconCharts).Size(12, 12).TextColor(white.Alpha(0.7))
					label := pl.UpdateFreq
					if label == "" {
						label = "每日更新"
					}
					ui.Text(c, label).FontSize(11).FontWeight(700).LetterSpacing(0.8).TextColor(white.Alpha(0.7))
				})
				ui.Text(c, pl.Name).FontSize(26).FontWeight(800).LetterSpacing(-0.5).TextColor(white).SingleLine()
				ui.Column(c).Gap(3).Margin(4, 0, 2, 0).Children(func() {
					for i, line := range pl.Preview {
						ui.Row(c).Gap(9).AlignItems(ui.Center).Children(func() {
							ui.Textf(c, "%d", i+1).FontSize(11).FontWeight(700).Width(12).FontFeatures("tnum").TextColor(white.Alpha(0.66))
							ui.Text(c, line.First).FontSize(12.5).FontWeight(600).TextColor(white.Alpha(0.95)).SingleLine().Shrink(1).MinWidth(0)
							ui.Text(c, line.Second).FontSize(12).TextColor(white.Alpha(0.58)).SingleLine().Shrink(1).MinWidth(0)
						})
					}
				})
				ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(6, 0, 0, 0).Children(func() {
					if pillButton(c.Key("play"), iconPlay, "播放", white, base).Clicked() {
						a.loadPlaylist(pl.ID, true)
					}
					var meta []string
					if pl.TrackCount > 0 {
						meta = append(meta, fmt.Sprintf("%d 首", pl.TrackCount))
					}
					if pl.PlayCount > 0 {
						meta = append(meta, plays(pl.PlayCount)+" 次播放")
					}
					if len(meta) > 0 {
						ui.Text(c, strings.Join(meta, " · ")).FontSize(11.5).FontWeight(600).TextColor(white.Alpha(0.62))
					}
				})
			})
			if wide {
				ui.Box(c).Size(148, 148).Shrink(0).Radius(14).Shadow(0, 18, 44, -10, ui.RGBA(0, 0, 0, 0.5)).Children(func() {
					a.coverArt(c, pl.Cover(), pl.Name, 500, 14)
				})
			}
		})
	})
	if card.Clicked() {
		a.open(route{page: pagePlaylist, id: pl.ID})
	}
}

// chartTile is one of the other charts: its cover on the left, its name,
// and the three songs at its top, on a wash of the cover's own color. The
// pointer lifts it, and a button on the cover plays it.
func (a *app) chartTile(c *ui.Context, pl *netease.Playlist) {
	p := a.pal
	card := ui.ButtonBase(c).Padding(13).Radius(16).Gap(13).AlignItems(ui.Center).FocusRing(false).Label(pl.Name)
	hovered := card.Hovered()
	tint := chartWash(a.avatarTint(pl.Cover(), pl.Name))
	fill := tint.Alpha(0.1)
	if p.dark {
		fill = tint.Alpha(0.16)
	}
	if hovered {
		fill = tint.Alpha(0.18)
		if p.dark {
			fill = tint.Alpha(0.26)
		}
	}
	card.Background(card.AnimateColor("bg", fill, ui.Animation{Duration: 140 * time.Millisecond})).Border(1, p.hairline)
	pose := ui.Pose{}
	if hovered {
		pose.Y = -2
		card.Shadow(0, 10, 26, -12, p.cardShade)
	}
	card.AnimatePose(pose, ui.Snappy)
	card.Children(func() {
		ui.Box(c).Size(86, 86).Shrink(0).Radius(12).Shadow(0, 3, 10, -3, p.cardShade).Children(func() {
			a.coverArt(c, pl.Cover(), pl.Name, 300, 12)
			a.coverPlayButton(c, hovered, func() { a.loadPlaylist(pl.ID, true) })
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, pl.Name).FontSize(14.5).FontWeight(700).SingleLine()
			meta := pl.UpdateFreq
			if pl.TrackCount > 0 {
				if meta != "" {
					meta += " · "
				}
				meta += fmt.Sprintf("%d 首", pl.TrackCount)
			}
			if meta != "" {
				ui.Text(c, meta).FontSize(11).TextColor(p.faint).SingleLine()
			}
			ui.Column(c).Gap(3).Margin(5, 0, 0, 0).Children(func() {
				for i, line := range pl.Preview {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Textf(c, "%d", i+1).FontSize(11).FontWeight(700).Width(11).FontFeatures("tnum").TextColor(p.accent.Alpha(0.85))
						ui.Text(c, line.First).FontSize(12).FontWeight(500).SingleLine().Shrink(1).MinWidth(0)
						ui.Text(c, line.Second).FontSize(11).TextColor(p.muted).SingleLine().Shrink(1).MinWidth(0)
					})
				}
			})
		})
	})
	if card.Clicked() {
		a.open(route{page: pagePlaylist, id: pl.ID})
	}
}

// chartWash softens a cover's color for the card behind a chart: a cover
// that is nearly black would give a wash nobody sees, so it is lifted.
func chartWash(c ui.Color) ui.Color {
	l := 0.299*float32(c.R)/255 + 0.587*float32(c.G)/255 + 0.114*float32(c.B)/255
	if l < 0.22 {
		return c.Mix(white, 0.55)
	}
	return c
}

func (a *app) playlistPage(c *ui.Context) {
	id := a.route.id
	pl := a.playlists[id]
	if pl == nil {
		if !a.loadingPl[id] {
			a.centered(c, a.plErr[id], func() { a.loadPlaylist(id, false) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, true, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	a.trackRows(c, pl.Tracks, true, func(c *ui.Context) { a.playlistHeader(c, pl) })
}

func (a *app) playlistHeader(c *ui.Context, pl *netease.Playlist) {
	p := a.pal
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
		ui.Box(c).Size(212, 212).Radius(14).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			a.playlistCover(c, pl, 600, 14)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			kind := "歌单"
			if pl.UpdateFreq != "" {
				kind = "榜单 · " + pl.UpdateFreq
			}
			ui.Text(c, kind).FontSize(12).FontWeight(600).TextColor(p.accent)
			ui.Text(c, pl.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
			var meta []string
			if pl.Creator.Nickname != "" {
				meta = append(meta, pl.Creator.Nickname)
			}
			meta = append(meta, fmt.Sprintf("%d 首", len(pl.Tracks)))
			if pl.PlayCount > 0 {
				meta = append(meta, plays(pl.PlayCount)+" 次播放")
			}
			ui.Text(c, strings.Join(meta, " · ")).FontSize(13).TextColor(p.muted)
			if d := strings.TrimSpace(pl.Description); d != "" {
				ui.Text(c, strings.ReplaceAll(d, "\n", " ")).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45).Margin(2, 0, 0, 0)
			}
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() {
					if a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.pl.PlayQueue(pl.Tracks, firstPlayable(pl.Tracks))
				}
				if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
					if !a.pl.Shuffle() {
						a.pl.ToggleShuffle()
					}
					a.pl.PlayQueue(pl.Tracks, firstPlayable(pl.Tracks))
				}
				if mine := a.editable(pl.ID); mine != nil {
					more := iconButton(c.Key("more"), iconMore, 18, p.muted, "更多")
					more.Menu(func(m *ui.Menu) {
						if m.Item("重命名…").Chosen() {
							a.startRename(mine)
						}
						m.Separator()
						if m.Item("删除歌单…").Chosen() {
							a.confirmDelete(mine)
						}
					})
				}
				switch {
				case pl.SpecialType != 5:
				case a.pl.Radio() == heartRadio:
					if pillButton(c.Key("heart"), iconX, "退出心动模式", p.hover.Over(p.bg), p.text).Clicked() {
						a.exitRadio()
					}
				default:
					label := "心动模式"
					if a.radioStarting == heartRadio {
						label = "正在开启…"
					}
					if pillButton(c.Key("heart"), iconHeartFill, label, p.hover.Over(p.bg), p.text).Clicked() {
						a.startHeartMode()
					}
				}
			})
		})
	})
}

// trackRows lists tracks under a header, which scrolls with them; a click
// plays the list from the track clicked.
func (a *app) trackRows(c *ui.Context, tracks []netease.Track, covers bool, header func(c *ui.Context), footer ...func(c *ui.Context)) {
	p := a.pal
	cur := a.pl.Current()
	// A song's menu can take it out of the playlist it is listed in.
	var pid int64
	if a.route.page == pagePlaylist {
		pid = a.route.id
	}
	w, _ := c.Size()
	// An album's page has no column of the album its songs are on.
	wide := w-sidebarWidth > 820 && a.route.page != pageAlbum
	top, bottom := a.insets(c)
	// Rows out of view are not built, their marks with them: a list
	// scrolled away from its start or end has content under the bars.
	a.underTop, a.underBottom = true, true
	rows := len(tracks) + 1 + len(footer)
	ui.List(c.Key(a.route), &a.trackList, rows, func(i int) {
		if i == 0 {
			ui.Column(c).Children(func() {
				a.startMark(c)
				header(c)
			})
			return
		}
		if i == rows-1 {
			defer a.endMark(c)
		}
		if i > len(tracks) {
			footer[i-len(tracks)-1](c)
			return
		}
		i--
		tr := &tracks[i]
		playing := cur != nil && cur.ID == tr.ID
		row := ui.ButtonBase(c.Key(tr.ID)).Height(54).PaddingX(12).Gap(14).Radius(10).AlignItems(ui.Center).FocusRing(false).Label(tr.Name).
			ContextMenu(a.songItems(tr, pid))
		hovered := row.Hovered()
		bg := ui.Transparent
		switch {
		case playing:
			bg = p.accent.Alpha(0.09)
		case hovered:
			bg = p.hover
		}
		row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 100 * time.Millisecond}))
		if !tr.Playable() {
			row.Opacity(0.42)
		}
		row.Children(func() {
			ui.Box(c).Width(26).Center().Children(func() {
				switch {
				case playing:
					a.nowPlayingBars(c)
				case hovered && tr.Playable():
					ui.Icon(c, iconPlay).Size(13, 13).TextColor(p.text)
				default:
					ui.Textf(c, "%d", i+1).FontSize(13).FontFeatures("tnum").TextColor(p.faint)
				}
			})
			if covers {
				a.cover(c, tr.Album.PicURL, 96, 38, 6)
			}
			ui.Column(c).Grow(3).Basis(0).MinWidth(0).Gap(3).Children(func() {
				name := p.text
				if playing {
					name = p.accent
				}
				spans := []ui.Span{{Text: tr.Name, Weight: 500, Color: name}}
				if len(tr.Alias) > 0 {
					spans = append(spans, ui.Span{Text: "（" + tr.Alias[0] + "）", Color: p.faint})
				}
				ui.RichText(c, spans...).FontSize(14).SingleLine()
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if !tr.Playable() {
						ui.Text(c, "VIP").FontSize(9).FontWeight(800).TextColor(p.accent).Padding(0, 4).Radius(3).Border(1, p.accent.Alpha(0.6))
					}
					a.artistLinks(c, tr, p.muted, p.text).FontSize(12.5).SingleLine().Shrink(1).MinWidth(0)
					if tr.Reason != "" {
						ui.Text(c, tr.Reason).FontSize(10.5).FontWeight(600).TextColor(p.accent).Padding(1, 6).Radius(999).Background(p.accent.Alpha(0.1)).SingleLine()
					}
				})
			})
			if wide {
				album := ui.RichText(c).Grow(2).Basis(0).MinWidth(0).FontSize(12.5).TextColor(p.muted).SingleLine()
				album.Children(func() {
					if tr.Album.ID == 0 {
						ui.Text(c, tr.Album.Name)
					} else {
						a.link(c.Key("album"), tr.Album.Name, p.muted, p.text, route{page: pageAlbum, id: tr.Album.ID})
					}
				})
			}
			// A liked song's heart shows; the others' as the pointer is on them.
			ui.Box(c).Size(32, 32).Center().Children(func() {
				if a.liked[tr.ID] || hovered {
					a.likeButton(c.Key("like"), tr, 15, p.muted)
				}
			})
			ui.Text(c, clock(tr.Length())).FontSize(12.5).FontFeatures("tnum").TextColor(p.faint).Width(44).TextAlign(ui.End)
		})
		if row.Clicked() && tr.Playable() {
			a.pl.PlayQueue(tracks, i)
		}
	}).Grow(1).Padding(top+pageGap, pagePad-12, bottom+32, pagePad-12).ScrollbarInsets(top, 0, bottom, 0)
}

// nowPlayingBars dance in the row of the track playing.
func (a *app) nowPlayingBars(c *ui.Context) {
	color := a.pal.accent
	ui.Box(c).Size(16, 14).Draw(func(p *ui.Painter, r ui.Rect) {
		lv, live := a.pl.Levels()
		a.rowBars.step(p.Now(), lv, live)
		drawBars(p, r, &a.rowBars, 4, color, 2.5)
		if live || a.rowBars.group(0, player.Bands) > 0.01 {
			p.AnimationFrame()
		}
	})
}
