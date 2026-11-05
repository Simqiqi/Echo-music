package app

import (
	"fmt"
	"math"
	"runtime"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
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
			// and frost. The player runs the whole width of the foot, so
			// the sidebar stops above it and leaves that much of its
			// column clear.
			bg := a.homeTinted(&p)
			liquid := a.settings.glassOn()
			a.ground(page, bg)
			side := ui.Column(c).Width(sidebarWidth)
			foot := float32(playerBarHeight)
			if !liquid {
				foot += 8
			}
			switch {
			case liquid:
				side.Padding(sidebarInset, sidebarInset, foot, sidebarInset)
			case !c.Vibrancy():
				side.Padding(0, 0, foot, 0).Background(p.sidebar)
			default:
				side.Padding(0, 0, foot, 0)
			}
			side.Children(func() {
				pill := ui.Column(c).Fill()
				if liquid {
					// A breath of the pane's own tone keeps the rim from
					// reading as a drawn line, and a soft shadow lets it
					// float over the page as glass does.
					tint := ui.RGBA(255, 255, 255, 0.10)
					if p.dark {
						tint = ui.RGBA(16, 16, 20, 0.10)
					}
					pill.Radius(sidebarRadius).
						Material(glass.Glass{Clarity: a.glassClarity(0.5), Tint: tint}).
						Shadow(0, 10, 30, -12, ui.RGBA(0, 0, 0, 0.12))
				}
				pill.Children(func() {
					if liquid {
						// Light from above, laid over the glass before its
						// contents: the rim fades into a sheen instead of
						// a hard outline around the whole sidebar.
						ui.Box(c.Key("side-sheen")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
							Radius(sidebarRadius).PassThrough().
							Draw(func(dr *ui.Painter, r ui.Rect) {
								top := ui.RGBA(255, 255, 255, 0.16)
								if a.pal.dark {
									top = ui.RGBA(255, 255, 255, 0.055)
								}
								dr.FillGradient(r, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
									{Color: top, At: 0},
									{Color: top.Alpha(0.04), At: 0.32},
									{Color: ui.Transparent, At: 0.7},
								}}, sidebarRadius)
							})
					}
					a.sidebar(c)
				})
			})
			// Keyed, so that a popover of the sidebar opening or closing
			// keeps what the page and its bars hold, their edges' fades.
			main := ui.Box(c.Key("main")).Grow(1)
			if !liquid {
				main.BorderWidth(0, 0, 0, 1).BorderColor(p.hairline)
			}
			main.Children(func() {
				a.content(c)
				// The top capsule floats over the page, which scrolls
				// under it: positioned, it paints over it.
				a.pageTop(c)
			})
			// The player is a pane of the page, not of its content: it
			// reaches from one side of the window to the other.
			a.playerBar(c)
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
	case pageDownloads:
		a.loadDownloads()
	case pageFeed:
		a.loadFeed(true)
	case pageNewAlbums:
		a.loadNewAlbums()
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
				a.navItem(c, iconDownloads, "下载管理", route{page: pageDownloads}, 7)
				a.navItem(c, iconFeed, "关注动态", route{page: pageFeed}, 8)
				a.navItem(c, iconNewAlbums, "新碟速递", route{page: pageNewAlbums}, 9)
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
		bg = a.songAccent.Alpha(0.13)
	case b.Hovered():
		bg = p.hover
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
	// Chosen items wear only this wash and the accent bar at their left:
	// a hard ink outline cuts the glass pane in two.
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
				if playing && !reduce {
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
	shell := ui.Box(c.Key("top-capsule")).Absolute().Top(y).Left(topCapsuleSide).Right(topCapsuleRight(c)).
		Height(h).Radius(h / 2).DragWindow()
	if a.settings.glassOn() {
		shell.Material(glass.Glass{Clarity: a.glassClarity(0.45)})
	} else {
		shell.Background(a.pal.bg).Border(1, a.pal.hairline)
	}
	shell.Shadow(0, 8, 24, -10, a.pal.cardShade)
	// What it holds: the way back, and the name of where the page has got
	// to; the name changes with the page, not with the bar.
	bar := ui.Row(c).Absolute().Top(y).Left(topCapsuleSide).Right(topCapsuleRight(c)).Height(h).
		Padding(4, 12).Gap(8).AlignItems(ui.Center).Clip()
	bar.Children(func() {
		// The way back, and the name of where the page has got to, at the
		// capsule's left.
		left := ui.Row(c).AlignItems(ui.Center).Gap(8).Shrink(0)
		left.Children(func() {
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
			a.netDot(c)
		})
		// What plays, at the capsule's middle: the sidebar's card came
		// here, where the eye already rests.
		ui.Box(c.Key("np-center")).Grow(1).Shrink(1).MinWidth(0).Center().DragWindow().Children(func() {
			if tr := a.pl.Current(); tr != nil {
				a.topNowPlaying(c, tr)
			}
		})
		// Who listens, at the capsule's right: the avatar and its menu.
		a.updateDot(c)
		a.topAccount(c)
	})
}

// netDot is the lamp beside a page's name: it breathes while something
// loads, and stays amber for a minute after a failure, so that a slow
// network reads as slow rather than as broken. A click says what went
// wrong.
func (a *app) netDot(c *ui.Context) {
	p := a.pal
	fail, failed := lastDiagnostic(time.Minute)
	busy := a.pl.Loading()
	if !busy && !failed {
		return
	}
	color, detail := p.accent, "正在加载"
	if failed {
		color, detail = ui.Hex("#f5a524"), fail.where+"："+fail.detail
	}
	dot := ui.ButtonBase(c.Key("net-dot")).Size(18, 18).Center().Radius(999).
		FocusRing(false).Label("网络状态")
	var lamp ui.Element
	dot.Children(func() {
		lamp = ui.Box(c).Size(7, 7).Radius(3.5).Background(color)
	})
	if busy && !failed {
		t := lamp.Loop("net-breathe", 1200*time.Millisecond, ui.Linear)
		lamp.Opacity(0.35 + 0.65*(1-float32(math.Abs(float64(2*t-1)))))
	}
	if dot.Clicked() {
		a.toast = detail
	}
}

// updateDot is the mark the top bar wears when a newer release is out: a
// click opens the updater's window, which is where it installs from.
func (a *app) updateDot(c *ui.Context) {
	if !a.updatePeek.out() {
		return
	}
	p := a.pal
	b := ui.ButtonBase(c.Key("update-dot")).Size(22, 22).Center().Radius(999).
		FocusRing(false).Label("有新版本 " + a.updatePeek.tag)
	if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(p.accent)
	})
	if b.Clicked() {
		checkForUpdates()
	}
}

// topNowPlaying is the card of what plays, as the top bar holds it: the
// cover, then the name with who sings it, the line being sung under them,
// and the heart that keeps it — for a click elsewhere to open Now Playing.
// It takes only the room the middle of the bar can spare.
func (a *app) topNowPlaying(c *ui.Context, tr *netease.Track) {
	p := a.pal
	sung, _, until, _ := a.sungLine()
	// Redraw as the next line begins, so the words keep the song's time.
	if until > 0 {
		c.After(until)
	}
	card := ui.ButtonBase(c.Key("np-top")).Gap(8).Padding(3, 8).Radius(9).AlignItems(ui.Center).
		FocusRing(false).Label("正在播放").MaxWidth(420).Shrink(1).MinWidth(0)
	bg := ui.Transparent
	if card.Hovered() {
		bg = p.hover
	}
	card.Background(card.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
	card.Children(func() {
		a.cover(c, tr.Album.PicURL, 96, 24, 5).Shadow(0, 1, 4, 0, p.cardShade)
		// Two rows in the room of one: what plays, and what is being sung
		// of it. Without words the card is the one it was.
		words := ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Justify(ui.Center).Gap(1)
		words.Children(func() {
			line := ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0)
			line.Children(func() {
				ui.Text(c, tr.Name).FontSize(12).FontWeight(600).SingleLine().Shrink(1).MinWidth(0)
				a.artistLinks(c, tr, p.muted, p.text).FontSize(11).SingleLine().Shrink(1).MinWidth(0)
			})
			if sung != "" {
				ui.Text(c, sung).FontSize(10.5).TextColor(p.muted).SingleLine().Shrink(1).MinWidth(0)
			}
		})
		a.likeButton(c, tr, 15, p.muted).Key("top-heart")
	})
	if card.Clicked() {
		a.nowPlaying = true
	}
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

// topCapsuleRight is the capsule's inset from the window's right edge:
// its own margin plus the room the window controls take there. Windows
// draws them at the top right of a hidden title bar, over where the
// capsule used to reach; macOS has nothing to the right.
func topCapsuleRight(c *ui.Context) float32 {
	return topCapsuleSide + c.TitleBar().Right
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
	case pageDownloads:
		return "下载管理"
	case pageFeed:
		return "关注动态"
	case pageNewAlbums:
		return "新碟速递"
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
	case pageDownloads:
		a.downloadsPage(c)
	case pageFeed:
		a.feedPage(c)
	case pageNewAlbums:
		a.newAlbumsPage(c)
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
