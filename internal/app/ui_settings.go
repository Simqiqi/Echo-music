package app

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/player"
)

// openSettings shows the settings over the page instead of a window of
// their own: a window would take the whole of a small screen, and pull
// the eye from the music it leaves behind.
func (a *app) openSettings() {
	a.measureCache()
	a.showMain()
	a.settingsOpen = true
	a.redraw()
}

// changed applies the settings everywhere and keeps them.
func (a *app) changed() {
	a.settings.save()
	a.settings.applyAppearance()
	if a.notch != nil {
		a.notch.setEnabled(a.settings.Notch)
	}
	a.redraw()
}

// settingsSheet draws the settings as a card over whatever the window
// shows: the page dims behind it, and a click beside it, or Esc, puts it
// away. It scrolls within itself, so it takes a share of the window and
// not the window itself.
func (a *app) settingsSheet(c *ui.Context) {
	if !a.settingsOpen {
		return
	}
	p := a.pal
	// Liquid Glass: the card is a pane of it, as the sidebar and the
	// player bar are, so the settings sit on one capsule of the page's
	// own color rather than in a box of their own.
	liquid := a.settings.glassOn()
	ui.DialogBase(c.Key("settings"), &a.settingsOpen, func(backdrop, panel ui.Element) {
		// Glass bends what it shows: dim less of the page away, that it
		// has something to bend.
		dim := float32(0.3)
		if liquid {
			dim = 0.22
		}
		backdrop.Background(ui.RGBA(0, 0, 0, dim))
		// A share of the window, but never more than a card need be: the
		// body scrolls inside it.
		panel.Width(520).HeightPercent(86).MaxHeight(760).Radius(22).Gap(0).Clip().
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.96, Y: 10}, Exit: &ui.Motion{Scale: 0.98, Opacity: 0}})
		if liquid {
			panel.Material(glass.Glass{Clarity: a.glassClarity(0.5)}).Shadow(0, 30, 70, -14, ui.RGBA(0, 0, 0, 0.28))
		} else {
			panel.Background(p.bg).Border(1, p.hairline).Shadow(0, 34, 80, -18, ui.RGBA(0, 0, 0, 0.5))
		}
		// A head that stays, above what scrolls under it.
		ui.Row(c).Padding(15, 18, 11, 14).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "设置").FontSize(15.5).FontWeight(700).Grow(1)
			if iconButton(c.Key("settings-close"), iconX, 12, p.muted, "关闭设置").Clicked() {
				a.settingsOpen = false
			}
		})
		ui.Box(c).Height(1).Background(p.hairline)
		ui.Scroll(c).Grow(1).Shrink(1).Padding(16, 18, 22, 18).Gap(8).Children(func() {
			a.settingsBody(c, p, liquid)
		})
	})
}

// settingsTestView renders the settings body for a test, as the card
// shows it.
func (a *app) settingsTestView(c *ui.Context) {
	a.settingsBody(c, paletteFor(c.Theme(), a.settings.Accent), false)
}

// settingsBody draws the settings themselves: groups of rows, in the order
// a listener would look for them. On glass, the groups are washes over it
// rather than boxes of their own, so the card reads as one pane.
func (a *app) settingsBody(c *ui.Context, p palette, liquid bool) {
	s := &a.settings
	// A step up from the card's ground, which the glass itself is.
	ground := p.sidebar
	switch {
	case liquid && p.dark:
		ground = ui.RGBA(255, 255, 255, 0.06)
	case liquid:
		ground = ui.RGBA(255, 255, 255, 0.42)
	}
	group := func(title string, rows func()) {
		ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(p.muted).Margin(14, 4, 4, 4)
		// On the card's ground, each group is a step of its own: the
		// quieter tone the window's sidebar would use, or a wash of the
		// glass.
		ui.Column(c).Radius(12).Background(ground).Border(1, p.hairline).Dividers(1, p.hairline).Children(rows)
	}
	row := func(label, desc string, control func()) {
		ui.Row(c).Padding(12, 16).Gap(16).AlignItems(ui.Center).MinHeight(48).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, label).FontSize(13.5).FontWeight(500)
				if desc != "" {
					ui.Text(c, desc).FontSize(12).TextColor(p.muted).LineHeight(1.4)
				}
			})
			control()
		})
	}

	// Only on macOS: elsewhere the icon is always in the notification
	// area.
	if runtime.GOOS == "darwin" {
		group("通用", func() {
			row("应用图标", "显示在菜单栏时，关闭窗口后可从菜单栏图标打开", func() {
				labels := make([]string, len(appIcons))
				chosen := appIcons[0].label
				for i, ic := range appIcons {
					labels[i] = ic.label
					if ic.id == s.AppIcon {
						chosen = ic.label
					}
				}
				if fieldSelect(c.Key("app-icon"), &chosen, labels, p).Changed() {
					for _, ic := range appIcons {
						if ic.label == chosen {
							s.AppIcon = ic.id
						}
					}
					a.changed()
					a.changeAppIcon()
				}
			})
		})
	}

	// Only on macOS, the one with a notch player.
	if a.notch != nil {
		group("刘海播放器", func() {
			row("显示刘海播放器", "在屏幕顶部的刘海处显示迷你播放器，鼠标悬停即可展开", func() {
				if ui.Switch(c.Key("notch"), &s.Notch).Label("显示刘海播放器").Changed() {
					a.changed()
				}
			})
			row("播放时显示实时活动", "在刘海两侧显示封面与音频波形，可能遮挡刘海旁的菜单栏图标", func() {
				if ui.Switch(c.Key("activity"), &s.NotchActivity).Label("播放时显示实时活动").Disabled(!s.Notch).Changed() {
					a.changed()
				}
			})
			row("在无刘海的屏幕上常驻显示", "关闭时，将鼠标移至屏幕顶部中央即可唤出，不会遮挡菜单栏", func() {
				if ui.Switch(c.Key("virtual"), &s.NotchVirtual).Label("在无刘海的屏幕上常驻显示").Disabled(!s.Notch).Changed() {
					a.changed()
				}
			})
		})
	}

	group("播放", func() {
		row("播放页标签自动收起", "鼠标离开标签栏 2 秒后，从右向左收成一个小圆；移回即可展开", func() {
			on := s.tabsAutoHide()
			if ui.Switch(c.Key("np-tabs-autohide"), &on).Label("播放页标签自动收起").Changed() {
				s.NPTabsAutoHide = &on
				a.changed()
			}
		})
		row("间奏时显示热评", "前奏、间奏与尾奏时，歌词处换成这首歌的热门评论", func() {
			if ui.Switch(c.Key("hot-comments"), &s.HotComments).Label("间奏时显示热评").Changed() {
				a.changed()
			}
		})
		row("均衡器", "十段调整，从低音到高音，对每一首歌都生效", func() {
			labels := make([]string, 0, len(player.EQPresets)+1)
			for _, pr := range player.EQPresets {
				labels = append(labels, pr.Label)
			}
			labels = append(labels, eqCustom)
			chosen := s.eqShape()
			if fieldSelect(c.Key("eq"), &chosen, labels, p).Changed() {
				if g, ok := eqPresetGains(chosen); ok {
					a.setEqGains(g)
				}
			}
		})
		a.eqBands(c, p)
		row("音量均衡", "按网易云音乐测得的响度调整每首歌的音量，让歌曲听起来一样响", func() {
			if ui.Switch(c.Key("normalize"), &s.Normalize).Label("音量均衡").Changed() {
				a.pl.SetVolume(a.volume)
				a.changed()
			}
		})
	})

	group("缓存", func() {
		row("歌曲缓存", "听过的歌曲保存在本机，再次播放无需等待，也不耗流量；"+a.cacheUsage(), func() {
			labels := make([]string, len(cacheLimits))
			chosen := cacheLimits[2].label
			for i, l := range cacheLimits {
				labels[i] = l.label
				if l.mb == s.CacheMB {
					chosen = l.label
				}
			}
			if fieldSelect(c.Key("cache"), &chosen, labels, p).Changed() {
				for _, l := range cacheLimits {
					if l.label == chosen {
						s.CacheMB = l.mb
					}
				}
				a.changed()
				a.applyCacheLimit()
			}
		})
		row("清除缓存", "删除保存在本机的全部歌曲", func() {
			if rowButton(c, "clear-cache", "清除", p).Disabled(a.cacheUsed <= 0).Clicked() {
				a.clearCache()
			}
		})
	})

	group("外观", func() {
		row("液态玻璃", "侧边栏与播放条化作悬浮的玻璃面板，内容从它们下面经过时被轻轻磨砂、折射，如 macOS 26 的质感", func() {
			on := s.glassOn()
			if ui.Switch(c.Key("glass"), &on).Label("液态玻璃").Changed() {
				s.Glass = &on
				a.changed()
			}
		})
		if s.glassOn() {
			row("玻璃清晰度", "往右：面板更通透，下面的内容看得更清楚；往左：更磨砂，更安静", func() {
				v := s.clarity()
				if claritySlider(c, "glass-clarity", &v, clarityLeast, clarityMost, p).Changed() {
					s.GlassClarity = &v
					a.changed()
				}
			})
		}
		row("强调色", "进度条、喜欢的心、选中的项都染上这一种颜色", func() {
			if id, ok := accentSwatches(c, "accent", s.Accent, p); ok {
				s.Accent = id
				a.changed()
			}
		})
		row("外观", "", func() {
			modes := []string{"system", "light", "dark"}
			i := 0
			for k, m := range modes {
				if m == s.Appearance {
					i = k
				}
			}
			if segmented(c.Key("appearance"), &i, "跟随系统", "浅色", "深色") {
				s.Appearance = modes[i]
				a.changed()
			}
		})
	})

	group("侧边栏", func() {
		// The nine entries of the directory as chips rather than rows:
		// nine rows would run the card past the foot of a small screen,
		// where a chip of two dozen points is a press and nothing more.
		entries := []struct {
			id    string
			label string
			icon  *ui.SVG
			on    *bool
		}{
			{"home", "为你推荐", iconHome, &s.SidebarHome},
			{"daily", "每日推荐", iconDaily, &s.SidebarDaily},
			{"weekly", "听歌周报", iconWeekly, &s.SidebarWeekly},
			{"fm", "私人 FM", iconRadio, &s.SidebarFM},
			{"weather", "天气电台", iconWeather, &s.SidebarWeather},
			{"charts", "排行榜", iconCharts, &s.SidebarCharts},
			{"podcasts", "播客", iconPodcast, &s.SidebarPodcasts},
			{"downloads", "下载管理", iconDownloads, &s.SidebarDownloads},
			{"newalbums", "新碟速递", iconNewAlbums, &s.SidebarNewAlbums},
		}
		// The chips follow the listener's order, so what is toggled here
		// stands in the same sequence as the directory itself.
		byID := make(map[string]int, len(entries))
		for i, it := range entries {
			byID[it.id] = i
		}
		ordered := make([]struct {
			id    string
			label string
			icon  *ui.SVG
			on    *bool
		}, 0, len(entries))
		for _, id := range s.sidebarOrder() {
			if i, ok := byID[id]; ok {
				ordered = append(ordered, entries[i])
			}
		}
		row("动态收缩", "鼠标离开侧边栏时收成一列图标，把宽度让给内容，移回来自动展开；关掉则一直展开", func() {
			on := s.sidebarAutoCollapse()
			if ui.Switch(c.Key("sidebar-collapse"), &on).Label("动态收缩").Changed() {
				s.SidebarAutoCollapse = &on
				a.changed()
			}
		})
		ui.Row(c).Wrap().Padding(10, 12).Gap(6).Children(func() {
			for _, it := range ordered {
				on := *it.on
				b := ui.ButtonBase(c.Key("sb-" + it.id)).Height(26).PaddingX(11).Gap(6).
					Radius(999).AlignItems(ui.Center).FocusRing(false).Label(it.label)
				// On: the accent's own light. Off: a hairline and quiet
				// text, so the two read at a glance.
				fg, bg, edge := p.muted, ui.Transparent, p.hairline
				switch {
				case on:
					fg, bg, edge = p.accent, p.accent.Alpha(0.14), p.accent.Alpha(0.3)
				case b.Hovered():
					bg = p.hover
				}
				b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond})).
					Border(1, edge)
				if b.Pressed() {
					b.AnimatePose(ui.Pose{Scale: 0.96}, ui.Snappy)
				}
				b.Children(func() {
					ui.Icon(c, it.icon).Size(13, 13).TextColor(fg)
					ui.Text(c, it.label).FontSize(12.5).FontWeight(500).TextColor(fg).SingleLine()
				})
				if b.Clicked() {
					*it.on = !on
					a.changed()
				}
			}
		})

		// Order: a short row per page with two arrows. Keeping the arrows
		// here, rather than in the directory itself, avoids fighting the
		// sidebar's drag-to-move-window gesture for the same pointer press.
		ui.Text(c, "顺序").FontSize(11).FontWeight(600).TextColor(p.muted).Padding(10, 4, 0, 4)
		dir := a.directoryEntries()
		ui.Column(c).Padding(2, 8, 8, 8).Gap(2).Children(func() {
			for i, e := range dir {
				e, i := e, i
				ui.Row(c).Height(30).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, fmt.Sprintf("%d", i+1)).FontSize(11).FontWeight(600).TextColor(p.faint).Width(14)
					ui.Icon(c, e.icon).Size(13, 13).TextColor(p.muted)
					ui.Text(c, e.label).FontSize(12.5).Grow(1).TextColor(p.text).SingleLine()
					if !e.visible {
						ui.Text(c, "已隐藏").FontSize(10.5).TextColor(p.faint)
					}
					if i > 0 && iconButton(c.Key("sb-up-"+e.id), iconUp, 13, p.muted, "上移").Clicked() {
						s.moveSidebarEntry(i, i-1)
						a.changed()
					}
					if i < len(dir)-1 && iconButton(c.Key("sb-down-"+e.id), iconDown, 13, p.muted, "下移").Clicked() {
						s.moveSidebarEntry(i, i+1)
						a.changed()
					}
				})
			}
			if len(s.SidebarOrder) > 0 {
				if ui.ButtonBase(c.Key("sb-order-reset")).Height(26).PaddingX(11).Radius(999).
					AlignItems(ui.Center).FocusRing(false).Label("恢复默认顺序").Clicked() {
					s.resetSidebarOrder()
					a.changed()
				}
			}
		})
	})

	group("实验性功能", func() {
		row("智能过渡", "正在播放的歌快结束时，下一首的开头提前轻轻接入，两首歌自然叠在一起，中间不停不顿；切换页面时新页面也从下方轻轻浮现。关掉则样样干净利落", func() {
			on := s.transitions()
			if ui.Switch(c.Key("transition"), &on).Label("智能过渡").Changed() {
				s.Transition = &on
				a.changed()
			}
		})
		row("播放进度条", "在播放栏左侧显示一条进度条，播放进度化作拖着烟雾的光点，随音乐起伏；会多用一些电量", func() {
			if ui.Switch(c.Key("progressbar"), &s.ProgressBar).Label("播放进度条").Changed() {
				a.changed()
			}
		})
	})

	group("账号", func() {
		if a.profile == nil {
			row("未登录", "登录后，每日推荐与歌单将根据你的口味生成", func() {
				if ui.PrimaryButton(c.Key("login"), "扫码登录").Clicked() {
					a.showMain()
					a.openLogin()
					a.win.Invalidate()
				}
			})
			return
		}
		prof := a.profile
		ui.Row(c).Padding(12, 16).Gap(12).AlignItems(ui.Center).Children(func() {
			a.avatar(c, prof.AvatarURL, 36, p.press)
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, prof.Nickname).FontSize(13.5).FontWeight(600)
				ui.Text(c, "已登录网易云音乐").FontSize(12).TextColor(p.muted)
			})
			if rowButton(c, "logout", "退出登录", p).Clicked() {
				a.signOut()
			}
		})
	})

	if a.updates {
		group("更新", func() {
			desc := "每天检查一次是否有新版本"
			if t := lastUpdateCheck(); !t.IsZero() {
				desc += "；上次检查：" + commentTime(t, time.Now())
			}
			row("自动检查更新", desc, func() {
				on := updateChecks()
				if ui.Switch(c.Key("update-checks"), &on).Label("自动检查更新").Changed() {
					setUpdateChecks(on)
				}
			})
			row("自动下载并安装", "在后台下载新版本，下次打开时生效", func() {
				on := updateDownloads()
				if ui.Switch(c.Key("update-downloads"), &on).Label("自动下载并安装").Disabled(!updateChecks()).Changed() {
					setUpdateDownloads(on)
				}
			})
			row("当前版本 "+mygo.App.Version(), "", func() {
				if rowButton(c, "check-updates", "检查更新", p).Clicked() {
					checkForUpdates()
				}
			})
		})
	}

	group("反馈与帮助", func() {
		row("遇到问题了？", "闪退、播放异常、界面错乱，附上复现步骤会更快修好", func() {
			if rowButton(c, "report-bug", "报告问题", p).Clicked() {
				a.writeFeedback("问题反馈")
			}
		})
		row("想让它更好？", "想加的功能、想改的交互，都欢迎告诉我们", func() {
			if rowButton(c, "suggest", "提建议", p).Clicked() {
				a.writeFeedback("功能建议")
			}
		})
		row("连接自检", "检查登录、网易云接口、播放地址、更新镜像与汽水音乐；失败项会随反馈邮件一起发出", func() {
			label := "开始检查"
			if a.checking {
				label = "检查中…"
			}
			if rowButton(c, "connection-check", label, p).Disabled(a.checking).Clicked() {
				a.startChecks()
			}
		})
		if a.checking {
			ui.Text(c, "正在检查…").FontSize(12).TextColor(p.muted).Padding(2, 16, 6, 16)
		}
		for _, r := range a.checks {
			res := r
			ui.Row(c).Padding(4, 16, 4, 16).Gap(8).AlignItems(ui.Center).Children(func() {
				dot := ui.RGB(74, 190, 120)
				if !res.ok {
					dot = ui.RGB(232, 92, 92)
				}
				ui.Box(c).Size(7, 7).Radius(4).Background(dot)
				ui.Text(c, res.name).FontSize(12.5).TextColor(p.text).Width(84)
				ui.Text(c, res.detail).FontSize(11.5).TextColor(p.muted).
					Grow(1).Shrink(1).MinWidth(0).SingleLine()
			})
		}
	})
}

// applyEq sets the player's equalizer to the bands the settings hold.
func (a *app) applyEq() {
	a.pl.EQ.Set(a.settings.eqGains())
}

// eqGains is the ten bands the settings stand at, padded to ten and
// clamped to what the filters take.
func (s settings) eqGains() [10]float64 {
	var out [10]float64
	if len(s.EqGains) == 0 {
		// 只记了预设名字的那些设置（这个字段之前只有名字）：按名字还原一次，
		// 之后就由十段增益自己说了算。
		if g, ok := eqPresetGains(s.EqPreset); ok {
			return g
		}
		return out
	}
	for i := range out {
		if i < len(s.EqGains) {
			out[i] = min(max(s.EqGains[i], -12), 12)
		}
	}
	return out
}

// setEqGains records the bands, and keeps the preset's name only while
// the bands are still the preset's own.
func (a *app) setEqGains(gains [10]float64) {
	s := &a.settings
	s.EqGains = make([]float64, 10)
	copy(s.EqGains, gains[:])
	s.EqPreset = ""
	for _, p := range player.EQPresets {
		if p.Label != "" && p.Gains == gains {
			s.EqPreset = p.Label
			break
		}
	}
	a.applyEq()
	// 均衡器只动声音，不动外观：所以不走 changed() 里那一趟主题应用。
	a.settings.save()
	a.redraw()
}

// eqPresetGains is the shape a preset names, and whether it names one.
func eqPresetGains(label string) ([10]float64, bool) {
	for _, p := range player.EQPresets {
		if p.Label == label {
			return p.Gains, true
		}
	}
	return [10]float64{}, false
}

// eqCustom is what the picker says once the listener moves a band away
// from every preset's own shape.
const eqCustom = "自定义"

// eqShape is the name of the shape the bands are in: a preset's, when they
// are still that preset's own, else 自定义 — or 关闭 when every band
// stands at nought.
func (s settings) eqShape() string {
	gains := s.eqGains()
	flat := true
	for _, g := range gains {
		if g != 0 {
			flat = false
			break
		}
	}
	if flat {
		return "关闭"
	}
	if s.EqPreset != "" {
		return s.EqPreset
	}
	return eqCustom
}

// eqBandLabel names a band by its middle frequency, as the sliders show
// it: 31, 62, 125 … 8k, 16k.
func eqBandLabel(i int) string {
	f := player.EQFreqs[i]
	if f >= 1000 {
		return fmt.Sprintf("%gk", f/1000)
	}
	return fmt.Sprintf("%g", f)
}

// eqBandValue is a band's gain as its readout shows it.
func eqBandValue(db float64) string {
	if db == 0 {
		return "0"
	}
	return fmt.Sprintf("%+.1f", db)
}

// eqBands is the ten sliders under the equalizer row: one per band, the
// bass first, each with what it lifts or lowers. Moving any of them makes
// the shape the listener's own, and the picker above says so.
func (a *app) eqBands(c *ui.Context, p palette) {
	gains := a.settings.eqGains()
	moved := false
	ui.Column(c).Padding(4, 16, 14, 16).Gap(2).Children(func() {
		for i := range gains {
			v := gains[i]
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, eqBandLabel(i)).FontSize(11).TextColor(p.muted).Width(34).TextAlign(ui.End)
				sl := ui.SliderBase(c.Key("eq-band-"+fmt.Sprintf("%d", i)), &v, -12, 12).
					Grow(1).Height(22).PaddingX(9).Step(0.5).FocusRing(false).Label("均衡器 " + eqBandLabel(i))
				eqSliderDraw(sl, &v, p)
				if sl.Changed() {
					gains[i] = v
					moved = true
				}
				ui.Text(c, eqBandValue(v)).FontSize(11).TextColor(p.faint).Width(30).TextAlign(ui.Start)
			})
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(6, 0, 0, 0).Children(func() {
			ui.Text(c, "拖动任意一段即成为自定义").FontSize(11).TextColor(p.faint).Grow(1)
			reset := ui.ButtonBase(c.Key("eq-flat")).Height(24).PaddingX(10).Radius(6).
				AlignItems(ui.Center).FocusRing(false).Label("归零")
			if reset.Hovered() {
				reset.Background(p.hover)
			}
			reset.Children(func() {
				ui.Text(c, "归零").FontSize(11.5).FontWeight(600).TextColor(p.muted)
			})
			if reset.Clicked() {
				a.setEqGains([10]float64{})
			}
		})
	})
	if moved {
		a.setEqGains(gains)
	}
}

// eqSliderDraw draws a band's slider as the settings draw one: a thin
// track, filled with the accent from the middle out — a band lifted
// rightwards, a band lowered leftwards — with a round knob on it.
func eqSliderDraw(s ui.Element, value *float64, p palette) {
	const knob, track = 16, 4
	s.Draw(func(pt *ui.Painter, r ui.Rect) {
		left, right := r.X+knob/2, r.X+r.W-knob/2
		y := r.Y + (r.H-track)/2
		v := float32(max(0, min(1, (*value+12)/24)))
		x := left + (right-left)*v
		mid := left + (right-left)/2
		pt.Fill(ui.Rect{X: left, Y: y, W: right - left, H: track}, p.press, track/2)
		lo, hi := min(mid, x), max(mid, x)
		if hi-lo > 0.5 {
			pt.Fill(ui.Rect{X: lo, Y: y, W: hi - lo, H: track}, p.accent, track/2)
		}
		k := ui.Rect{X: x - knob/2, Y: r.Y + (r.H-knob)/2, W: knob, H: knob}
		pt.Shadow(k, knob/2, 0, 0.5, 1.5, 0, ui.RGBA(0, 0, 0, 0.3))
		fill := ui.RGB(255, 255, 255)
		if s.Pressed() {
			fill = fill.Mix(ui.RGB(0, 0, 0), 0.06)
		}
		pt.Fill(k, fill, knob/2)
	})
}
