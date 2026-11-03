package app

import (
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/player"
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
		row("音质", "无损与 Hi-Res 需要黑胶 VIP，没有时播放所能播放的最高音质；下一首歌曲开始生效", func() {
			labels := make([]string, len(qualityLevels))
			chosen := qualityLevels[2].label
			for i, q := range qualityLevels {
				labels[i] = q.label
				if q.level == s.Quality {
					chosen = q.label
				}
			}
			if fieldSelect(c.Key("quality"), &chosen, labels, p).Changed() {
				for _, q := range qualityLevels {
					if q.label == chosen {
						s.Quality = q.level
					}
				}
				a.changed()
			}
		})
		row("间奏时显示热评", "前奏、间奏与尾奏时，歌词处换成这首歌的热门评论", func() {
			if ui.Switch(c.Key("hot-comments"), &s.HotComments).Label("间奏时显示热评").Changed() {
				a.changed()
			}
		})
		if a.dj.voice != nil {
			row("DJ 讲解", "私人 FM 在每首歌开始时用女声介绍它、推荐理由与创作背景，像电台 DJ 在你耳边串讲", func() {
				on := s.DJ
				if ui.Switch(c.Key("dj"), &on).Label("DJ 讲解").Changed() {
					s.DJ = on
					if !on {
						a.djCancel()
					}
					a.changed()
					a.djArm()
				}
			})
		}
		row("均衡器", "按预设调整音色的走向，对每一首歌都生效", func() {
			labels := make([]string, len(player.EQPresets))
			chosen := labels[0]
			for i, p := range player.EQPresets {
				labels[i] = p.Label
				if p.Label == s.EqPreset {
					chosen = p.Label
				}
			}
			if fieldSelect(c.Key("eq"), &chosen, labels, p).Changed() {
				s.EqPreset = chosen
				a.applyEq()
				a.changed()
			}
		})
		row("音量均衡", "按网易云音乐测得的响度调整每首歌的音量，让歌曲听起来一样响", func() {
			if ui.Switch(c.Key("normalize"), &s.Normalize).Label("音量均衡").Changed() {
				a.pl.SetVolume(a.volume)
				a.changed()
			}
		})
	})

	if a.captionsShown() {
		group("播客", func() { a.captionSettings(row, c) })
	}

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

	group("实验性功能", func() {
		row("智能过渡", "一首歌播完前缓缓淡出、下一首缓缓淡入；切换页面时新页面也从下方轻轻浮现。关掉则样样干净利落", func() {
			on := s.transitions()
			if ui.Switch(c.Key("transition"), &on).Label("智能过渡").Changed() {
				s.Transition = &on
				a.changed()
			}
		})
		row("黑胶唱片", "播放页的封面化作缓缓旋转的黑胶唱片，暂停时停在原地；再点封面上的唱片按钮也可开关", func() {
			if ui.Switch(c.Key("vinyl"), &s.Vinyl).Label("黑胶唱片").Changed() {
				a.changed()
			}
		})
		row("进度条烟雾特效", "播放时进度条化作拖着烟雾的光点，随音乐起伏；会多用一些电量", func() {
			if ui.Switch(c.Key("smoke"), &s.Smoke).Label("进度条烟雾特效").Changed() {
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

	// At the very foot: who keeps this, and where they are found.
	group("开发者", func() { a.developerCard(c, p) })
}

// applyEq sets the player's equalizer to the shape the settings name.
func (a *app) applyEq() {
	for _, p := range player.EQPresets {
		if p.Label == a.settings.EqPreset {
			a.pl.EQ.Set(p.Gains)
			return
		}
	}
	a.pl.EQ.Set([10]float64{})
}
