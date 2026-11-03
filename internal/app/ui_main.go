package app

import (
	"fmt"
	"math"
	"runtime"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// sidebarWidth is how wide the sidebar's column is, the gap that holds
// its pane of glass and the room around it.
const sidebarWidth = 232

// sidebarInset is the room the sidebar's pane of glass keeps from the
// window's edges, floating it over the page, and sidebarRadius its
// corners, as macOS 26's floating sidebars round them.
const (
	sidebarInset  = 8
	sidebarRadius = 22
)

func (a *app) view(c *ui.Context) {
	t := *c.Theme()
	a.pal = paletteFor(&t, a.settings.Accent)
	p := a.pal
	a.songAccent = a.coverAccent(&p)
	t.Accent, t.AccentText = p.accent, p.onAccent
	t.Selection = p.accent.Alpha(0.3)
	c.SetTheme(&t)

	a.shortcuts(c)

	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	root := ui.Box(c).Fill()
	if !c.Focused(a.searchBox) {
		// Space plays and pauses wherever the focus is, as in Music. On
		// the container around everything, it comes before a button the
		// pointer focused, which would take Space as a click; on the
		// window, it would come after.
		root.OnShortcut(0, ui.KeySpace, a.pl.TogglePlay)
		// And where nothing has the focus, as once Now Playing covers the
		// button that had it.
		c.OnShortcut(0, ui.KeySpace, a.pl.TogglePlay)
	}
	npOpen, npSettled := a.npState(c)
	root.Children(func() {
		page := ui.Row(c).Fill().AlignItems(ui.Stretch)
		page.Children(func() {
			if npOpen && npSettled {
				// Covered by Now Playing: not built, which would cost each
				// of its frames as much again as the stage itself. Where
				// it is scrolled is the app's (pageScroll, navScroll,
				// trackList), so it shows again as it was.
				return
			}
			// Liquid Glass: the page's ground fills the whole window, and
			// the sidebar floats on it as a pane of glass, a step above
			// the page, so the color of what plays has something to bend
			// and frost.
			bg := a.homeTinted(&p)
			liquid := a.settings.glassOn()
			if liquid {
				a.ground(page, bg)
			}
			side := ui.Column(c).Width(sidebarWidth)
			switch {
			case liquid:
				side.Padding(sidebarInset, sidebarInset, sidebarInset, sidebarInset)
			case !c.Vibrancy():
				side.Background(p.sidebar)
			}
			side.Children(func() {
				pill := ui.Column(c).Fill()
				if liquid {
					pill.Radius(sidebarRadius).Material(glass.Glass{Clarity: a.glassClarity(0.5)})
				}
				pill.Children(func() { a.sidebar(c) })
			})
			// Keyed, so that a popover of the sidebar opening or closing
			// keeps what the page and its bars hold, their edges' fades.
			main := ui.Box(c.Key("main")).Grow(1)
			if !liquid {
				main.BorderWidth(0, 0, 0, 1).BorderColor(p.hairline)
				a.ground(main, bg)
			}
			main.Children(func() {
				a.content(c)
				// The bars float over the page, which scrolls under them:
				// positioned, they paint over it.
				a.pageTop(c)
				a.playerBar(c)
			})
		})
		a.nowPlayingLayer(c, npOpen, npSettled)
	})
	// The settings, when open, sit above the page but below a dialog
	// they may ask for.
	a.settingsSheet(c)
	a.loginDialog(c)
	a.playlistNameDialog(c)
	a.deletePlaylistDialog(c)
	if a.toast != "" {
		c.Toast(a.toast)
		a.toast = ""
	}
	a.syncWindows()
}

func (a *app) shortcuts(c *ui.Context) {
	// The side buttons of a mouse, the keys of keyboards that have them
	// and swipes between pages, and the keys of Music.
	c.OnShortcut(0, ui.KeyBack, a.back)
	c.OnShortcut(0, ui.KeyForward, a.goForward)
	c.OnShortcut(ui.Cmd, ui.KeyBracketLeft, a.back)
	c.OnShortcut(ui.Cmd, ui.KeyBracketRight, a.goForward)
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		a.nowPlaying = false
		a.searchBox.Focus(c)
	})
	if a.nowPlaying {
		c.OnShortcut(0, ui.KeyEscape, func() { a.nowPlaying = false })
		// ⌘ and a number choose Now Playing's tab of that place.
		if a.pl.Current() != nil {
			for i, t := range a.npTabs() {
				c.OnShortcut(ui.Cmd, ui.Key1+ui.Key(i), func() { a.choosePane(t.pane) })
			}
		}
	}
}

// go opens a page, remembering the one before for the back button, and
// closes Now Playing over it, though it is the page open.
func (a *app) open(r route) {
	a.nowPlaying = false
	if r == a.route {
		return
	}
	a.history, a.forward = append(a.history, a.route), nil
	a.show(r)
}

// show shows the page of r.
func (a *app) show(r route) {
	a.route = r
	if r.page == pageSearch {
		a.query, a.searched = r.query, r
	}
	a.load(r)
}

// load loads what the page of r shows, unless it has.
func (a *app) load(r route) {
	switch r.page {
	case pageHome:
		a.freshenHome()
		// The day's first pick wears another landscape each time it opens,
		// and the daily card another rainbow.
		a.sky.pick(a, a.post)
		a.rollDailyRainbow()
	case pagePlaylist:
		a.loadPlaylist(r.id, false)
	case pageSearch:
		a.loadSearch(r, false)
	case pageArtist:
		a.loadArtist(r.id)
	case pageAlbum:
		a.loadAlbum(r.id)
	case pageUser:
		a.loadUser(r.id)
	case pagePodcast:
		a.loadPodcast(r.id)
	case pagePodcasts:
		a.loadPodcastTab()
	case pageWeather:
		// Another sky each time the page opens, from the photographs
		// already in hand; the weather itself is kept between openings.
		if a.wx == nil {
			a.wx = newWx()
			a.wx.art = a.art
		}
		a.wx.rollShot(a.post)
		a.wx.ensure(a.post, nil)
	case pageFM:
		// The FM's card wears another landscape each time it opens.
		a.sky.pick(a, a.post)
	}
}

// back goes to the page before, as the back button of a mouse does, or
// closes Now Playing, which covers the pages.
func (a *app) back() {
	if a.nowPlaying {
		a.nowPlaying = false
		return
	}
	if len(a.history) == 0 {
		return
	}
	a.forward = append(a.forward, a.route)
	r := a.history[len(a.history)-1]
	a.history = a.history[:len(a.history)-1]
	a.show(r)
}

// goForward goes to the page back left, as the forward button of a mouse
// does.
func (a *app) goForward() {
	if len(a.forward) == 0 {
		return
	}
	a.history = append(a.history, a.route)
	r := a.forward[len(a.forward)-1]
	a.forward = a.forward[:len(a.forward)-1]
	a.nowPlaying = false
	a.show(r)
}

func (a *app) sidebar(c *ui.Context) {
	// Under the window buttons where they sit over the sidebar, as the
	// traffic lights do; elsewhere, as close to the top as to the sides.
	top := float32(14)
	if c.TitleBar().Left > 0 {
		top = topBar(c) + 4
	}
	if a.settings.glassOn() {
		// The pane floats, inset from the top: what it leaves is the room
		// its own inset gives back.
		top -= sidebarInset
	}
	// Where nothing in it takes the press, the sidebar moves the window, as
	// the title bar it reaches up into would.
	ui.Column(c).Fill().Padding(top, 14, 14, 14).Gap(4).DragWindow().Children(func() {
		a.searchField(c)

		// #31 滚动渐隐：顶/底毛玻璃，随滚动柔化边缘。
		// 渐变作为覆盖在滚动区之上的绝对定位层，确保在内容之上合成，
		// 不被滚动子项遮挡（在 Scroll 自身 Draw 上绘制会被其子项盖住）。
		navWrap := ui.Box(c.Key("nav-wrap")).Grow(1).Margin(0, -6)
		navWrap.Children(func() {
			sc := ui.Scroll(c.Key("nav")).TrackScroll(&a.navScroll).Fill().Padding(0, 6).Gap(4)
			sc.Children(func() {
				a.navItem(c, iconHome, "为你推荐", route{page: pageHome}, 0)
				a.navItem(c, iconDaily, "每日推荐", route{page: pageDaily}, 1)
				a.navItem(c, iconWeekly, "听歌周报", route{page: pageWeekly}, 2)
				a.navItem(c, iconRadio, "私人 FM", route{page: pageFM}, 3)
				a.navItem(c, iconWeather, "天气电台", route{page: pageWeather}, 4)
				a.navItem(c, iconCharts, "排行榜", route{page: pageCharts}, 5)
				a.navItem(c, iconPodcast, "播客", route{page: pagePodcasts}, 6)
				if a.searched.query != "" {
					a.navItem(c, iconSearch, "搜索结果", a.searched, 7)
				}
				a.myPlaylists(c, "我的歌单", a.mine, true, &a.mineCollapsed)
				a.myPlaylists(c, "收藏的歌单", a.saved, false, &a.savedCollapsed)
			})
			// 顶部柔化：随向上滚动量淡入，把内容溶进背景。
			// PassThrough：仅作装饰合成层，不拦截指针，否则会盖住顶部「为你推荐」的点击。
			ui.Box(c.Key("nav-top-fade")).Absolute().Top(0).Left(0).Right(0).Height(26).PassThrough().
				Draw(func(dr *ui.Painter, r ui.Rect) {
					top := float32(0)
					if a.navScroll.Y > 0 {
						top = a.navScroll.Y / 24
						if top > 1 {
							top = 1
						}
					}
					if top > 0.01 {
						dr.FillGradient(r, ui.LinearGradient{From: a.pal.bg.Alpha(top), To: a.pal.bg.Alpha(0), Angle: 90}, 0)
					}
				})
			// 底部柔化：随接近底部淡入。PassThrough 同上，避免遮挡底部项点击。
			ui.Box(c.Key("nav-bot-fade")).Absolute().Bottom(0).Left(0).Right(0).Height(26).PassThrough().
				Draw(func(dr *ui.Painter, r ui.Rect) {
					bot := float32(0)
					if a.navScroll.MaxY > 0 && a.navScroll.Y < a.navScroll.MaxY {
						bot = (a.navScroll.MaxY - a.navScroll.Y) / 24
						if bot > 1 {
							bot = 1
						}
					}
					if bot > 0.01 {
						dr.FillGradient(r, ui.LinearGradient{From: a.pal.bg.Alpha(0), To: a.pal.bg.Alpha(bot), Angle: 90}, 0)
					}
				})
		})

		// The words of the moment, then what plays, as a card at the foot
		// of the sidebar, over who listens.
		a.lyricPeek(c)
		if tr := a.pl.Current(); tr != nil {
			a.sidebarNowPlaying(c, tr)
		}
		a.accountRow(c)
	})
}

func (a *app) navItem(c *ui.Context, icon *ui.SVG, label string, r route, index int) {
	p := a.pal
	selected := a.route.page == r.page && !a.nowPlaying
	b := ui.ButtonBase(c.Key(label)).Height(34).PaddingX(10).Gap(10).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false)
	playing := r.page == pageFM && selected && a.pl.Playing()
	reduce := c.Preferences().ReduceMotion
	// #26 错峰入场：淡入并轻滑，按 index 递延，像菜单依次展开。
	if !reduce {
		b.Transition(ui.ElementTransition{
			Enter: &ui.Motion{Opacity: 0, X: -12},
			Animation: &ui.Animation{
				Duration: 380 * time.Millisecond,
				Delay:    time.Duration(index) * 45 * time.Millisecond,
				Ease:     ui.EaseOut,
			},
		})
	}
	bg := ui.Transparent
	switch {
	case selected:
		bg = a.songAccent.Alpha(0.10)
	case b.Hovered():
		bg = p.hover
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
	if selected {
		// The pill carries a thin ink line at its edge.
		b.Border(1, ui.RGB(28, 28, 30))
	}
	color := p.text
	if selected {
		color = a.songAccent
	}
	b.Children(func() {
		// #27 图标：hover 放大；选中态持续轻微呼吸。#30 播放中：图标外呼吸光环。
		ui.Box(c.Key("ic")).Size(19, 19).AlignItems(ui.Center).Justify(ui.Center).
			Draw(func(dr *ui.Painter, r ui.Rect) {
				if !playing || reduce {
					return
				}
				now := dr.Now()
				f := float64(now.UnixNano()%int64(1800*time.Millisecond)) / float64(1800*time.Millisecond)
				ph := float32(0.5 - 0.5*math.Cos(2*math.Pi*f))
				op := 0.16 + 0.30*ph
				rad := 9 + 4*ph
				cx, cy := r.X+r.W/2, r.Y+r.H/2
				dr.Stroke(ui.Rect{X: cx - rad, Y: cy - rad, W: 2 * rad, H: 2 * rad}, a.songAccent.Alpha(op), rad, 1.6)
			}).
			Children(func() {
				ic := ui.Icon(c, icon).Size(17, 17).TextColor(color)
				var s float32
				if selected && !reduce {
					s = ic.Loop("nav-ic-"+label, 2*time.Second, ui.EaseInOut)
				}
				sc := float32(1)
				if b.Hovered() {
					sc = 1.15
				}
				if selected && !reduce {
					sc *= 1 + 0.05*s
				}
				ic.Scale(sc, sc)
			})
		ui.Text(c, label).FontSize(13.5).FontWeight(500).TextColor(p.text)
		if selected {
			// A bar of the accent at the item's left edge, sliding in
			// from further left as it arrives.
			bar := ui.Box(c.Key("ind")).Absolute().Left(-10).Top(6).Size(3, 22).Radius(1.5).Background(a.songAccent)
			if !c.Preferences().ReduceMotion {
				bar.Transition(ui.ElementTransition{
					Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 350 * time.Millisecond, Bounce: 0.25}},
					Enter:     &ui.Motion{X: -10, Opacity: 0},
				})
			}
		}
	})
	// #29 hover 涟漪：指针处柔和亮斑，绘于背景层，不拦截点击。
	b.Draw(func(dr *ui.Painter, r ui.Rect) {
		x, y, over := b.PointerPosition()
		if !over {
			return
		}
		now := dr.Now()
		f := float64(now.UnixNano()%int64(1600*time.Millisecond)) / float64(1600*time.Millisecond)
		ph := float32(0.5 - 0.5*math.Cos(2*math.Pi*f))
		rad := 10 + 26*ph
		op := 0.05 + 0.07*ph
		dr.Fill(ui.Rect{X: x - rad, Y: y - rad, W: 2 * rad, H: 2 * rad}, a.songAccent.Alpha(op), rad)
	})
	if b.Clicked() {
		a.open(r)
	}
}

func (a *app) sidebarNowPlaying(c *ui.Context, tr *netease.Track) {
	p := a.pal
	card := ui.ButtonBase(c.Key("np-card")).Gap(10).Padding(8).Radius(12).AlignItems(ui.Center).FocusRing(false).Label("正在播放")
	bg := a.songAccent.Alpha(0.06)
	if card.Hovered() {
		bg = a.songAccent.Alpha(0.11)
	}
	card.Background(card.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
	card.Children(func() {
		cov := a.cover(c, tr.Album.PicURL, 96, 40, 6)
		cov.Shadow(0, 2, 6, 0, p.cardShade)
		if a.pl.Playing() && !c.Preferences().ReduceMotion {
			// #28 呼吸：原先用 cov.Loop 缩放封面，会让侧栏常驻 60fps 整窗重建
			// （在「为你推荐」等非正在播放页播放时尤其拖累）。改为 30fps 的 Draw
			// 呼吸光环——不触发视图重建，重绘成本大降，仍有呼吸感。
			cov.Draw(func(dr *ui.Painter, r ui.Rect) {
				f := float64(dr.Now().UnixNano()%int64(2000*time.Millisecond)) / float64(2000*time.Millisecond)
				ph := float32(0.5 - 0.5*math.Cos(2*math.Pi*f))
				// 外扩 4px，画一圈随相位呼吸的强调色光晕（30fps，不触发整窗重建）。
				rr := ui.Rect{X: r.X - 4, Y: r.Y - 4, W: r.W + 8, H: r.H + 8}
				op := 0.28 + 0.34*ph
				rad := 9 + 3*ph
				dr.Stroke(rr, a.songAccent.Alpha(op), rad, 3)
				dr.AnimationFrameRate(30)
				dr.AnimationFrameRate(30)
			})
		}
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, tr.Name).FontSize(12.5).FontWeight(600).SingleLine()
			a.artistLinks(c, tr, p.muted, p.text).FontSize(11.5).SingleLine()
		})
		// Three slim bars at the foot of the card, dancing while the
		// music does, still when it does not.
		ui.Box(c.Key("bars")).Size(17, 16).Draw(func(dr *ui.Painter, r ui.Rect) {
			lv, live := a.pl.Levels()
			a.sideBars.step(dr.Now(), lv, live)
			drawBars(dr, r, &a.sideBars, 3, ui.RGB(28, 28, 30), 2)
			if live {
				dr.AnimationFrameRate(20)
			}
		})
	})
	if card.Clicked() {
		a.nowPlaying = true
	}
}

// playerBarHeight is the height of the player along the foot of the pages.
const playerBarHeight = 84

// glassBarHeight is the height of the player as a pane of Liquid Glass
// floating above the foot of the pages: with its margin of 8 below and
// 8 of air above, it takes the same room as the bar.
const glassBarHeight = 68

// pageGap is the room between the bar over a page and its content.
const pageGap = 12

// insets returns the room the bars over a page take at its top and its
// bottom: pages pad their content by it, and scroll under the bars.
func (a *app) insets(c *ui.Context) (top, bottom float32) {
	top = topBar(c)
	if runtime.GOOS == "darwin" {
		// The edge reaches a little below the traffic lights' strip.
		top += topEdgePad
	}
	return top, playerBarHeight
}

// topEdgePad is the room the edge over a page leaves below the strip, on
// macOS.
const topEdgePad = 6

// buttonsHeight is the height of the window buttons off macOS, which fill
// the edge over a page.
const buttonsHeight = topBarHeight + topEdgePad

// topBar returns the height of the strip along the top of the window, as
// tall as the window buttons and the room above and below them, which
// what sits in it centers on.
func topBar(c *ui.Context) float32 {
	if runtime.GOOS != "darwin" {
		// The whole edge, which the buttons fill: what sits in the strip
		// centers on their glyphs.
		return buttonsHeight
	}
	return max(c.TitleBar().Height, topBarHeight)
}

// topBarHeight is the least height of the strip along the top.
const topBarHeight = 40

// startMark marks where a page's content starts, first in what scrolls:
// once it has moved up, content is under the bar at the top.
func (a *app) startMark(c *ui.Context) {
	b := ui.Box(c.Key("start-mark")).Height(0).Bounds()
	if b.W > 0 {
		top, _ := a.insets(c)
		a.underTop = b.Y < top+pageGap-0.5
	}
}

// endMark marks where a page's content ends, last in what scrolls: while it
// is below the top of the player, content is under the player.
func (a *app) endMark(c *ui.Context) {
	b := ui.Box(c.Key("end-mark")).Height(0).Bounds()
	if b.W > 0 {
		_, h := c.Size()
		_, bottom := a.insets(c)
		a.underBottom = b.Y > h-bottom+0.5
	}
}

// edgeShown returns how much of a scroll edge shows over el: fully while
// content is under its bar, and none at rest, as AppKit shows its edges.
func edgeShown(el ui.Element, under bool) float32 {
	target := float32(0)
	if under {
		target = 1
	}
	return el.AnimateTo("shown", target, ui.Animation{Duration: 180 * time.Millisecond})
}

// pageTop is the capsule over the top of a page: a floating pane of
// Liquid Glass, inset from every edge as the player's bar is from the foot,
// which drags the window, holds the back and forward buttons, and names the
// section the page has scrolled to — a name that rises into it as the one
// before sinks away.
func (a *app) pageTop(c *ui.Context) {
	top, _ := a.insets(c)
	y, h := topCapsule(top)
	// Where the strip was is still the window's to drag, along its whole
	// width.
	ui.Box(c.Key("top-strip")).Absolute().Top(0).Left(0).Right(0).Height(top).DragWindow()
	// The capsule itself: glass where the app wears it, and a quiet plate
	// of the page's own ground where it does not.
	shell := ui.Box(c.Key("top-capsule")).Absolute().Top(y).Left(topCapsuleSide).Right(topCapsuleSide).
		Height(h).Radius(h / 2).DragWindow()
	if a.settings.glassOn() {
		shell.Material(glass.Glass{Clarity: a.glassClarity(0.45)})
	} else {
		shell.Background(a.pal.bg).Border(1, a.pal.hairline)
	}
	shell.Shadow(0, 8, 24, -10, a.pal.cardShade)
	// What it holds: the way back, and the name of where the page has got
	// to; the name changes with the page, not with the bar.
	bar := ui.Row(c).Absolute().Top(y).Left(topCapsuleSide).Right(topCapsuleSide).Height(h).
		Padding(4, 12).Gap(8).AlignItems(ui.Center).Clip()
	bar.Children(func() {
		if len(a.history) > 0 || len(a.forward) > 0 {
			back := a.navButton(c, "back", iconBack, "返回").Disabled(len(a.history) == 0)
			if len(a.history) == 0 {
				back.Opacity(0.35)
			}
			if back.Clicked() {
				a.back()
			}
			if len(a.forward) > 0 && a.navButton(c, "forward", iconForward, "前进").Clicked() {
				a.goForward()
			}
		}
		title := a.navTitle
		if title == "" {
			title = a.navName()
		}
		a.navLabel(c, title)
	})
}

// The capsule's measures: a little air above it, and its height, which the
// strip over a page holds.
const (
	topCapsuleInset = 6
	topCapsuleSide  = 14
	topCapsuleTall  = 40
)

// topCapsule is where the capsule over a page sits and how tall it is.
func topCapsule(top float32) (y, h float32) {
	return topCapsuleInset, min(topCapsuleTall, top-2*topCapsuleInset)
}

// barLine is where the capsule's foot is: the line a section's name has to
// cross to be taken up by it.
func (a *app) barLine(c *ui.Context) float32 {
	top, _ := a.insets(c)
	y, h := topCapsule(top)
	return y + h
}

// navLabel is the name the capsule shows: a new one rises into it as the
// one before sinks away, rather than replacing it in place.
func (a *app) navLabel(c *ui.Context, title string) {
	t := ui.Text(c.Key("nav-"+title), title).FontSize(13).FontWeight(650).LetterSpacing(-0.1).
		MaxLines(1).TextColor(a.pal.text)
	if !c.Preferences().ReduceMotion && a.settings.transitions() {
		t.Transition(ui.ElementTransition{
			Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: 0.18}},
			Enter:     &ui.Motion{Y: 9, Opacity: 0},
			Exit:      &ui.Motion{Y: -9, Opacity: 0},
		})
	}
}

// navName is what the capsule says before a section has scrolled under it:
// the page's own name.
func (a *app) navName() string {
	switch a.route.page {
	case pageHome:
		return "为你推荐"
	case pageDaily:
		return "每日推荐"
	case pageWeekly:
		return "听歌周报"
	case pageFM:
		return "私人 FM"
	case pageWeather:
		return "天气电台"
	case pageCharts:
		return "排行榜"
	case pagePodcasts:
		return "播客"
	case pagePodcast:
		return "播客节目"
	case pageSearch:
		return "搜索结果"
	case pagePlaylist:
		return "歌单"
	case pageArtist:
		return "歌手"
	case pageAlbum:
		return "专辑"
	case pageUser:
		return "用户"
	}
	return "Echo"
}

// navButton is a page's back or forward button: a round bead of its own
// inside the capsule they share, which lights under the pointer and gives
// a little as it is pressed. Without the glass, it is the icon alone, with
// a soft circle under the pointer.
func (a *app) navButton(c *ui.Context, key string, icon *ui.SVG, label string) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(34, 34).Center().Radius(999).Label(label).TextColor(a.pal.muted)
	bg := ui.Transparent
	if a.settings.glassOn() {
		// A bead on the glass: quieter than the pane, brighter under the
		// pointer, so that the two read as buttons within it.
		bg = ui.RGBA(255, 255, 255, 0.13)
		if b.Hovered() {
			bg = ui.RGBA(255, 255, 255, 0.26)
		}
	} else if b.Hovered() {
		bg = ui.RGBA(128, 128, 128, 0.14)
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 130 * time.Millisecond}))
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.9
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() { ui.Icon(c, icon).Size(18, 18) })
	return b
}

func (a *app) content(c *ui.Context) {
	// A new page starts at its top. Not as it is opened: a control in a
	// list, as a page's tabs in its header, opens it as the list builds.
	if a.scrolled != a.route {
		a.trackList, a.pageScroll, a.scrolled = ui.ListState{}, ui.ScrollState{}, a.route
	}
	// Pages mark where their content is, as they scroll, and which section
	// the capsule names.
	a.underTop, a.underBottom, a.navTitle = false, false, ""
	// Keyed by where it shows, a page is a new one as the listener goes
	// elsewhere, and comes in faint, from a little below, rather than
	// appearing whole.
	key := fmt.Sprintf("page-%d-%d-%s-%d", a.route.page, a.route.id, a.route.query, a.route.kind)
	page := ui.Box(c.Key(key)).Grow(1)
	if !c.Preferences().ReduceMotion && a.settings.transitions() {
		page.Transition(ui.ElementTransition{
			Duration: 220 * time.Millisecond,
			Enter:    &ui.Motion{Y: 10, Opacity: 0},
		})
	}
	page.Children(func() { a.pageOf(c) })
}

// pageOf shows the page the route names.
func (a *app) pageOf(c *ui.Context) {
	switch a.route.page {
	case pageHome:
		a.homePage(c)
	case pageDaily:
		a.dailyPage(c)
	case pageWeekly:
		a.weeklyPage(c)
	case pageCharts:
		a.chartsPage(c)
	case pagePlaylist:
		a.playlistPage(c)
	case pageSearch:
		a.searchPage(c)
	case pageFM:
		a.fmPage(c)
	case pageWeather:
		a.weatherPage(c)
	case pageArtist:
		a.artistPage(c)
	case pageAlbum:
		a.albumPage(c)
	case pageUser:
		a.userPage(c)
	case pagePodcast:
		a.podcastPage(c)
	case pagePodcasts:
		a.podcastsPage(c)
	}
}

// centered shows a message, with a retry button if retry is set: what a
// page shows in place of what failed to load.
func (a *app) centered(c *ui.Context, err string, retry func()) {
	top, bottom := a.insets(c)
	ui.Column(c).Grow(1).Padding(top, 0, bottom, 0).Center().Gap(12).Children(func() {
		ui.Text(c, err).TextColor(a.pal.muted).FontSize(13)
		if retry != nil && ui.Button(c, "重试").Clicked() {
			retry()
		}
	})
}

// Loading what the pages show.

func (a *app) loadHome() {
	a.homeLoading, a.homeErr = true, ""
	signedIn := a.profile != nil
	at := time.Now()
	go func() {
		featured, err1 := netease.FeaturedPlaylists(30)
		charts, err2 := netease.Charts()
		daily, _ := netease.DailySongs()
		var rec []netease.Playlist
		if signedIn {
			rec, _ = netease.RecommendedPlaylists()
		}
		a.post(func() {
			a.homeLoading = false
			if err1 != nil && err2 != nil {
				// Loading the new day's again, what shows stays, to try
				// once more.
				if len(a.featured) == 0 && len(a.charts) == 0 {
					a.homeErr = "无法连接网易云音乐，请检查网络"
				}
				return
			}
			a.featured, a.charts, a.daily, a.recPlaylists = featured, charts, daily, rec
			a.homeAt = at
		})
	}()
}

// freshenHome loads the home page again once NetEase has picked the day's
// songs anew since it last did: the app may stay open overnight.
func (a *app) freshenHome() {
	if !a.homeLoading && !a.homeAt.IsZero() && dailyRenewed(a.homeAt, time.Now()) {
		a.loadHome()
	}
}

// beijing is NetEase's time, without daylight saving.
var beijing = time.FixedZone("CST", 8*60*60)

// dailyRenewed reports whether NetEase has picked new daily songs between
// loaded and now. It does at 6:00, Beijing time.
func dailyRenewed(loaded, now time.Time) bool {
	b := now.In(beijing)
	six := time.Date(b.Year(), b.Month(), b.Day(), 6, 0, 0, 0, beijing)
	if b.Before(six) {
		six = six.AddDate(0, 0, -1)
	}
	return loaded.Before(six)
}

func (a *app) loadPlaylist(id int64, thenPlay bool) {
	if pl := a.playlists[id]; pl != nil {
		if thenPlay {
			a.pl.PlayQueue(pl.Tracks, firstPlayable(pl.Tracks))
		}
		return
	}
	if a.loadingPl[id] {
		return
	}
	a.loadingPl[id] = true
	delete(a.plErr, id)
	go func() {
		pl, err := netease.PlaylistDetail(id)
		a.post(func() {
			delete(a.loadingPl, id)
			if err != nil {
				a.plErr[id] = "歌单加载失败"
				return
			}
			a.playlists[id] = pl
			if thenPlay {
				a.pl.PlayQueue(pl.Tracks, firstPlayable(pl.Tracks))
			}
		})
	}()
}

func firstPlayable(tracks []netease.Track) int {
	for i := range tracks {
		if tracks[i].Playable() {
			return i
		}
	}
	return 0
}

// homeTinted is the ground of the home's pages: the theme's background
// holding a little of the playing cover's color, dulled toward gray, so
// the room feels of the music without shouting of it. The last color
// stays while a cover loads, and when nothing plays the ground is plain.
func (a *app) homeTinted(p *palette) ui.Color {
	tr := a.pl.Current()
	if tr == nil || tr.Album.PicURL == "" {
		return p.bg
	}
	if art := a.art.get(netease.Sized(tr.Album.PicURL, 64), true); art != nil && !art.failed {
		a.homeArtURL, a.homeTint = tr.Album.PicURL, art.tint
	}
	if a.homeArtURL != tr.Album.PicURL {
		return p.bg // no tint of this cover yet: plain until it arrives
	}
	// Water the color down toward gray, then into the ground, plainly
	// there to see.
	soft := a.homeTint.Mix(ui.RGB(128, 128, 132), 0.25)
	return p.bg.Mix(soft, 0.18)
}

// coverAccent is the accent of the moment: the playing cover's color,
// whitened for tints and text; the theme's accent when nothing plays or
// the cover has yet to arrive.
func (a *app) coverAccent(p *palette) ui.Color {
	tr := a.pl.Current()
	if tr == nil || tr.Album.PicURL == "" {
		return p.accent
	}
	if art := a.art.get(netease.Sized(tr.Album.PicURL, 64), true); art != nil && !art.failed {
		a.homeArtURL, a.homeTint = tr.Album.PicURL, art.tint
	}
	if a.homeArtURL == tr.Album.PicURL {
		return a.homeTint.Mix(white, 0.45)
	}
	return p.accent
}
