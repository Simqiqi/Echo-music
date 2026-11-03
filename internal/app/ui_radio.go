package app

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/simqiqi/simqiqi-music/internal/lyrics"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// Radios play on without end: the personal FM, in one of its modes, as
// "fm:<mode>", and the heart mode of the liked songs, as "heart".

const heartRadio = "heart"

var fmModes = []struct{ mode, label string }{
	{netease.FMDefault, "推荐"},
	{netease.FMFamiliar, "熟悉"},
	{netease.FMExplore, "探索"},
}

func fmRadio(mode string) string { return "fm:" + mode }

// radioLabel names a radio as the listener knows it.
func radioLabel(name string) string {
	if name == heartRadio {
		return "心动模式"
	}
	for _, m := range fmModes {
		if name == fmRadio(m.mode) {
			return "私人 FM · " + m.label
		}
	}
	return "私人 FM"
}

func playable(tracks []netease.Track) []netease.Track {
	var out []netease.Track
	for _, t := range tracks {
		if t.Playable() {
			out = append(out, t)
		}
	}
	return out
}

// likedPlaylist returns the ID of the listener's liked songs, or 0.
func (a *app) likedPlaylist() int64 {
	for _, pl := range a.mine {
		if pl.SpecialType == 5 {
			return pl.ID
		}
	}
	return 0
}

// radioFeed returns what gives the radio name more tracks: it loads them
// on a goroutine, and gives them to got on the main thread, none if it
// cannot.
func (a *app) radioFeed(name string) func(got func([]netease.Track)) {
	return func(got func([]netease.Track)) {
		var fetch func() ([]netease.Track, error)
		switch {
		case strings.HasPrefix(name, "fm:"):
			mode := strings.TrimPrefix(name, "fm:")
			fetch = func() ([]netease.Track, error) { return netease.PersonalFM(mode) }
		case name == heartRadio:
			// More like the last song queued, from the song it began with.
			q, pid := a.pl.Queue(), a.likedPlaylist()
			if len(q) == 0 || pid == 0 {
				got(nil)
				return
			}
			start, last := q[0].ID, q[len(q)-1].ID
			fetch = func() ([]netease.Track, error) { return netease.HeartMode(last, start, pid) }
		default:
			got(nil)
			return
		}
		go func() {
			// The FM gives a song or three at a time, some of them VIP
			// songs the listener may not play: ask again for those.
			var tracks []netease.Track
			for tries := 0; tries < 3 && len(tracks) == 0; tries++ {
				more, err := fetch()
				if err != nil {
					break
				}
				tracks = playable(more)
			}
			a.post(func() { got(tracks) })
		}()
	}
}

// playFM plays the personal FM in the mode chosen, or goes on playing it.
func (a *app) playFM() {
	name := fmRadio(fmModes[a.fmMode].mode)
	if a.pl.Radio() == name {
		if !a.pl.Playing() {
			a.pl.TogglePlay()
		}
		return
	}
	a.radioStarting = name
	feed := a.radioFeed(name)
	feed(func(tracks []netease.Track) {
		if a.radioStarting != name {
			return
		}
		a.radioStarting = ""
		if len(tracks) == 0 {
			a.toast = "私人 FM 暂时无法播放，请稍后再试"
			return
		}
		a.pl.PlayRadio(name, tracks, feed)
	})
}

// exitRadio stops the radio playing, but not the song it plays.
func (a *app) exitRadio() {
	name := a.pl.Radio()
	if name == "" {
		return
	}
	a.pl.EndRadio()
	a.fmSeed = ""
	label := radioLabel(name)
	if name != heartRadio {
		label = "私人 FM"
	}
	a.toast = "已退出" + label
}

// trashFM skips the song playing on the FM, which plays it no more.
func (a *app) trashFM() {
	tr := a.pl.Current()
	if tr == nil {
		return
	}
	id := tr.ID
	a.pl.Next()
	go netease.FMTrash(id)
}

// startHeartMode plays songs like one the listener likes: the one playing,
// if they like it, or else one of their liked songs at random.
func (a *app) startHeartMode() {
	if a.profile == nil {
		a.showMain()
		a.openLogin()
		return
	}
	pid := a.likedPlaylist()
	if pid == 0 {
		a.toast = "喜欢一些歌曲后，即可开启心动模式"
		return
	}
	if a.pl.Radio() == heartRadio {
		a.nowPlaying = true
		return
	}
	var seed *netease.Track
	fromPlaying := false
	if cur := a.pl.Current(); cur != nil && a.liked[cur.ID] {
		s := *cur
		seed, fromPlaying = &s, true
	}
	liked := a.playlists[pid]
	a.radioStarting = heartRadio
	go func() {
		if seed == nil {
			if liked == nil {
				liked, _ = netease.PlaylistDetail(pid)
			}
			if liked != nil {
				if ok := playable(liked.Tracks); len(ok) > 0 {
					seed = &ok[rand.IntN(len(ok))]
				}
			}
		}
		var more []netease.Track
		var err error
		if seed != nil {
			more, err = netease.HeartMode(seed.ID, seed.ID, pid)
		}
		a.post(func() {
			if a.radioStarting != heartRadio {
				return
			}
			a.radioStarting = ""
			if seed == nil || err != nil {
				a.toast = "心动模式暂时无法开启，请稍后再试"
				return
			}
			more = playable(more)
			a.fmSeed = seed.Name
			if fromPlaying && a.pl.Current() != nil && a.pl.Current().ID == seed.ID {
				a.pl.RadioFrom(heartRadio, more, a.radioFeed(heartRadio))
				return
			}
			a.pl.PlayRadio(heartRadio, append([]netease.Track{*seed}, more...), a.radioFeed(heartRadio))
		})
	}()
}

// fmSky is the landscape the personal FM's card wears: a photograph from
// the day's wallpapers — the same pool the weather radio wears — taken
// anew each time the page opens.
type fmSky struct {
	url     string    // the photograph chosen
	shown   string    // the one that showed last, while the next comes
	tries   int       // photographs turned away for failing to come
	hover   bool      // the pointer is on the card
	sweepAt time.Time // when the light began to pass over it
}

// pick takes another landscape than the one it wears, from the day's
// photographs, and starts it loading.
func (s *fmSky) pick(a *app, post func(func())) {
	if a.wx == nil {
		a.wx = newWx()
		a.wx.art = a.art
	}
	a.wx.ensureShot(post)
	shots := a.wx.shots
	if len(shots) == 0 {
		return
	}
	n := a.wx.rng.Intn(len(shots))
	for i := range shots {
		c := shots[(n+i)%len(shots)]
		if c.URL != s.url && !a.wx.washed[c.URL] {
			s.url = c.URL
			a.art.get(c.URL, false)
			return
		}
	}
	if s.url == "" {
		s.url = shots[n].URL
		a.art.get(s.url, false)
	}
}

// bmp is the landscape's bitmap, or the one that showed while the next
// comes: the card never goes bare between openings.
func (s *fmSky) bmp(a *app) *ui.Bitmap {
	if s.url != "" {
		if art := a.art.get(s.url, false); art != nil {
			if art.bmp != nil {
				s.shown, s.tries = s.url, 0
				return art.bmp
			}
			// The one chosen cannot be had: take another, a few times
			// over, rather than sit on a photograph that never comes.
			if art.failed && s.url != s.shown && s.tries < 3 {
				s.tries++
				s.pick(a, a.post)
			}
		}
	}
	if s.shown != "" {
		if art := a.art.get(s.shown, false); art != nil && art.bmp != nil {
			return art.bmp
		}
	}
	return nil
}

// fmPage is the personal FM: what it plays, large, over its cover's
// colors, with the FM's own controls, and heart mode below.
func (a *app) fmPage(c *ui.Context) {
	p := a.pal
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("fm")).TrackScroll(&a.pageScroll).Grow(1).Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		// The mode the FM plays in, if it plays.
		for i, m := range fmModes {
			if a.pl.Radio() == fmRadio(m.mode) {
				a.fmMode = i
			}
		}
		ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
			ui.Box(c).Grow(1).Children(func() { a.heading(c, "私人 FM", "无尽推荐") })
			mode := a.fmMode
			ui.Box(c).Margin(0, 0, 22, 0).Children(func() {
				labels := make([]string, len(fmModes))
				for i, m := range fmModes {
					labels[i] = m.label
				}
				// The modes, in liquid glass: the page's own light
				// shows through them.
				if glassSegmented(c.Key("fm-mode"), &mode, labels...) {
					a.fmMode = mode
					// Playing the FM, it changes to the mode at once.
					if strings.HasPrefix(a.pl.Radio(), "fm:") {
						a.playFM()
					}
				}
			})
		})
		a.fmCard(c)
		a.fmRecent(c)
		a.section(c, "心动模式")
		a.heartCard(c, p)
	})
}

// fmCard shows the song the FM plays, or how to start it.
func (a *app) fmCard(c *ui.Context) {
	pl := a.pl
	onFM := strings.HasPrefix(pl.Radio(), "fm:")
	var tr *netease.Track
	if onFM {
		tr = pl.Current()
	}
	// The card wears a landscape, another each time the page opens.
	if a.sky.url == "" {
		a.sky.pick(a, a.post)
	}
	bmp := a.sky.bmp(a)
	base := ui.Hex("#1b1b20")
	card := ui.Box(c.Key("fm-card")).Height(360).Radius(24).Background(base).TextColor(white).
		Shadow(0, 18, 44, -16, a.pal.cardShade)
	// The light passes over the card as the pointer comes onto it, once
	// each time it does.
	if card.Hovered() {
		if !a.sky.hover {
			a.sky.hover = true
			a.sky.sweepAt = c.Now()
		}
	} else {
		a.sky.hover = false
	}
	card.Children(func() {
		if bmp != nil {
			ui.Image(c, bmp).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover).Radius(24)
		} else {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(24).
				LinearGradient(ui.LinearGradient{Angle: 135, Oklab: true, From: ui.Oklch(0.32, 0.06, 20), To: ui.Oklch(0.24, 0.05, 300)})
		}
		// The landscape fades out to the left, into the card's own dark,
		// and stands clear at its right: the words keep their ground and
		// the photograph keeps its weather.
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(24).
			LinearGradient(ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
				{Color: base.Alpha(0.96), At: 0},
				{Color: base.Alpha(0.92), At: 0.18},
				{Color: base.Alpha(0.8), At: 0.38},
				{Color: base.Alpha(0.58), At: 0.56},
				{Color: base.Alpha(0.3), At: 0.74},
				{Color: base.Alpha(0.08), At: 0.9},
				{Color: base.Alpha(0), At: 1},
			}})
		// Its foot, so the words stand off it.
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(24).
			LinearGradient(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
				{Color: ui.RGBA(0, 0, 0, 0), At: 0},
				{Color: ui.RGBA(0, 0, 0, 0.06), At: 0.55},
				{Color: ui.RGBA(0, 0, 0, 0.38), At: 1},
			}})
		// A clock of its own while the light crosses, and no longer.
		if !a.sky.sweepAt.IsZero() && c.Now().Sub(a.sky.sweepAt) <= time.Second {
			ui.Box(c).Size(1, 1).Loop("fmsweep", 900*time.Millisecond, ui.Linear)
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Clip(r, 24, func() { lightSweep(p, r, a.sky.sweepAt, c.Now()) })
		})
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(36, 40).Gap(40).AlignItems(ui.Center).Children(func() {
			a.fmCovers(c, tr)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
				label := "私人 FM · " + fmModes[a.fmMode].label
				if onFM {
					label = radioLabel(pl.Radio())
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconRadio).Size(14, 14).TextColor(white.Alpha(0.75))
					ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(white.Alpha(0.75))
				})
				if tr == nil {
					ui.Text(c, "听见更多好音乐").FontSize(30).FontWeight(800).LetterSpacing(-0.6)
					ui.Text(c, "根据你的口味，一首接一首，不停推荐").FontSize(13.5).TextColor(white.Alpha(0.75))
					ui.Row(c).Margin(18, 0, 0, 0).Children(func() {
						label := "开始收听"
						if a.radioStarting != "" && strings.HasPrefix(a.radioStarting, "fm:") {
							label = "正在调频…"
						}
						if pillButton(c.Key("start"), iconPlay, label, white, ui.Hex("#1d1d1f")).Clicked() {
							a.playFM()
						}
					})
					return
				}
				ui.Text(c, tr.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
				switch {
				case pl.Loading():
					ui.Text(c, "正在缓冲…").FontSize(14).TextColor(white.Alpha(0.7)).SingleLine()
				case pl.Err() != "":
					ui.Text(c, pl.Err()).FontSize(14).TextColor(white).SingleLine()
				default:
					a.artistLinks(c, tr, white.Alpha(0.7), white, true).FontSize(14).SingleLine()
				}
				if reason := a.fmReason(); reason != "" {
					ui.Row(c).Margin(2, 0, 4, 0).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(5, 5).Radius(999).Background(white.Alpha(0.55)).Shrink(0)
						ui.Text(c, reason).FontSize(12.5).TextColor(white.Alpha(0.62)).SingleLine()
					})
				}
				ui.Row(c).Gap(14).AlignItems(ui.Center).Margin(14, 0, 0, -8).Children(func() {
					if a.profile != nil {
						if iconButton(c.Key("trash"), iconTrash, 20, white.Alpha(0.8), "不喜欢").Clicked() {
							a.trashFM()
						}
					}
					a.likeButton(c.Key("like"), tr, 20, white.Alpha(0.8))
					play := ui.ButtonBase(c.Key("play")).Size(56, 56).Center().Radius(999).Background(white).TextColor(ui.Hex("#1d1d1f")).Label("播放/暂停")
					pose := ui.Pose{}
					if play.Pressed() {
						pose.Scale = 0.92
					} else if play.Hovered() {
						pose.Scale = 1.05
					}
					play.AnimatePose(pose, ui.Snappy)
					play.Children(func() { playPause(c, play, pl.Playing(), 22) })
					if play.Clicked() {
						pl.TogglePlay()
					}
					if iconButton(c.Key("next"), iconNext, 20, white.Alpha(0.8), "下一首").Clicked() {
						pl.Next()
					}
				})
				a.fmLyricPeek(c)
			})
		})
	})
}

// fmCovers stacks the cover playing over the next one's, which peeks out
// behind it, tilted, as the FM's own player shows them.
func (a *app) fmCovers(c *ui.Context, tr *netease.Track) {
	const size = 232
	ui.Box(c).Size(size+36, size).Shrink(0).Children(func() {
		var next *netease.Track
		if tr != nil {
			if order, at := a.pl.PlayOrder(); at >= 0 && at+1 < len(order) {
				next = &a.pl.Queue()[order[at+1]]
			}
		}
		if next != nil {
			back := ui.Box(c.Key(next.ID)).Absolute().Top(16).Left(36).Size(size-32, size-32).Radius(14).Opacity(0.55).Background(white.Alpha(0.12)).
				Shadow(0, 10, 24, -8, ui.RGBA(0, 0, 0, 0.4))
			back.AnimatePose(ui.Pose{Rotate: 6, X: 14}, ui.Animation{Spring: ui.SpringOptions{Duration: 500 * time.Millisecond, Bounce: 0.2}})
			back.Children(func() { a.coverFill(c, next.Album.PicURL, 400, 14) })
		}
		front := ui.Box(c.Key("front")).Absolute().Top(0).Left(0).Size(size, size).Radius(16).
			Background(white.Alpha(0.12)).Shadow(0, 20, 44, -10, ui.RGBA(0, 0, 0, 0.55))
		if tr == nil {
			front.Center().Children(func() {
				ui.Icon(c, iconRadio).Size(64, 64).TextColor(white.Alpha(0.85))
			})
			return
		}
		scale := float32(1)
		if !a.pl.Playing() {
			scale = 0.94
		}
		front.AnimatePose(ui.Pose{Scale: scale}, ui.Animation{Spring: ui.SpringOptions{Duration: 600 * time.Millisecond, Bounce: 0.25}})
		front.Children(func() {
			ui.Box(c.Key(tr.ID)).Fill().Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.92}}).
				Children(func() { a.coverFill(c, tr.Album.PicURL, 600, 16) })
		})
	})
}

// heartCard starts heart mode: songs like those the listener likes. It
// stands on the page warmed by the heart — its own color with a breath of
// red in it, a glow at its foot, and a button of beaten glass.
func (a *app) heartCard(c *ui.Context, p palette) {
	on := a.pl.Radio() == heartRadio
	warm := ui.Oklch(0.66, 0.2, 12)
	card := ui.Row(c).Padding(22, 26).Gap(20).Radius(22).AlignItems(ui.Center).
		Shadow(0, 10, 28, -14, p.cardShade)
	card.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Clip(r, 22, func() {
			pt.FillGradient(r, ui.LinearGradient{Angle: 120, Oklab: true, Stops: []ui.GradientStop{
				{At: 0, Color: p.bg.Mix(warm, 0.13)},
				{At: 0.5, Color: p.bg.Mix(warm, 0.06)},
				{At: 1, Color: p.bg.Mix(ui.Oklch(0.58, 0.14, 342), 0.09)},
			}}, 22)
			// A glow under the heart, brighter while it beats.
			glow := float32(0.45)
			if on {
				glow = 1
			}
			pt.FillGradient(ui.Rect{X: r.X - r.W*0.05, Y: r.Y - r.H*0.3, W: r.W * 0.6, H: r.H * 1.6},
				ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
					{At: 0, Color: warm.Alpha(0.12 * glow)},
					{At: 1, Color: warm.Alpha(0)},
				}}, 0)
			// Its rim, lit warmer as the heart is on.
			pt.Stroke(ui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1},
				warm.Alpha(0.14+0.1*glow), 22, 1)
		})
	})
	card.Children(func() {
		// The heart itself, a drop of red glass.
		beat := ui.Box(c).Size(58, 58).Radius(19).Shrink(0).Center().
			LinearGradient(ui.LinearGradient{Angle: 150, Oklab: true,
				From: ui.Oklch(0.74, 0.18, 16), To: ui.Oklch(0.56, 0.23, 350)}).
			Shadow(0, 10, 22, -8, warm.Alpha(0.45)).TextColor(white)
		pose := ui.Pose{}
		if card.Hovered() {
			pose.Scale = 1.04
		}
		beat.AnimatePose(pose, ui.Animation{Spring: ui.SpringOptions{Duration: 500 * time.Millisecond, Bounce: 0.3}})
		beat.Children(func() {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(19).
				LinearGradient(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
					{At: 0, Color: ui.RGBA(255, 255, 255, 0.36)},
					{At: 0.6, Color: ui.RGBA(255, 255, 255, 0)},
				}})
			ui.Icon(c, iconHeartFill).Size(27, 27)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(5).Children(func() {
			ui.Text(c, "从你喜欢的音乐出发").FontSize(15.5).FontWeight(800).LetterSpacing(-0.2)
			ui.Text(c, "根据你红心过的歌曲，智能推荐风格相似的音乐，与你的喜欢穿插播放").FontSize(12.5).
				TextColor(p.muted).LineHeight(1.45)
		})
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if on {
				if glassPill(c, c.Key("exit-heart"), iconX, "退出心动模式", p).Clicked() {
					a.exitRadio()
				}
			}
			label := "开启心动模式"
			switch {
			case a.profile == nil:
				label = "登录后开启"
			case on:
				label = "正在播放"
			case a.radioStarting == heartRadio:
				label = "正在开启…"
			}
			// The heart's own button: a lozenge of lit red, warmer under
			// the pointer.
			btn := ui.ButtonBase(c.Key("heart")).Height(40).PaddingX(20).Gap(8).Radius(999).
				AlignItems(ui.Center).TextColor(white).Background(ui.Transparent)
			var hot float32
			if btn.Hovered() {
				hot = 1
			}
			hot = btn.AnimateTo("hot", hot, ui.Animation{Duration: 140 * time.Millisecond})
			btn.Draw(func(pt *ui.Painter, r ui.Rect) {
				pt.Shadow(r, r.H/2, 0, 7, 18, -4, warm.Alpha(0.28+0.24*hot))
				pt.FillGradient(r, ui.LinearGradient{Angle: 150, Oklab: true, Stops: []ui.GradientStop{
					{At: 0, Color: ui.Oklch(0.7+0.04*hot, 0.19, 16)},
					{At: 1, Color: ui.Oklch(0.57+0.04*hot, 0.23, 352)},
				}}, r.H/2)
				pt.Stroke(ui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1},
					ui.RGBA(255, 255, 255, 0.24), r.H/2-0.5, 1)
			})
			press := ui.Pose{}
			if btn.Pressed() {
				press.Scale = 0.96
			}
			btn.AnimatePose(press, ui.Snappy)
			btn.Children(func() {
				ui.Icon(c, iconHeartFill).Size(15, 15)
				ui.Text(c, label).FontSize(13.5).FontWeight(650).SingleLine()
			})
			if btn.Clicked() {
				a.startHeartMode()
			}
		})
	})
}

// glassPill is a quiet button of the page's own glass, for the way out of
// what the heart's button starts.
func glassPill(c *ui.Context, key any, icon *ui.SVG, label string, p palette) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Height(40).PaddingX(18).Gap(8).Radius(999).AlignItems(ui.Center).TextColor(p.text)
	fill := p.hover
	if b.Hovered() {
		fill = p.press
	}
	b.Background(b.AnimateColor("bg", fill, ui.Animation{Duration: 120 * time.Millisecond}))
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.96
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(14, 14)
		}
		ui.Text(c, label).FontSize(13.5).FontWeight(600).SingleLine()
	})
	return b
}

// fmReason says why the personal FM plays what it plays now: where the
// heart mode began, or what the mode the listener chose leans toward.
func (a *app) fmReason() string {
	if a.pl.Radio() == heartRadio && a.fmSeed != "" {
		return "从你喜欢的《" + a.fmSeed + "》出发"
	}
	switch fmModes[a.fmMode].mode {
	case netease.FMFamiliar:
		return "偏向你熟悉的风格与声音"
	case netease.FMExplore:
		return "带你去还没听过的角落"
	default:
		return "顺着你的口味，一首接一首"
	}
}

// fmLyricPeek shows the line being sung and the one after it, over the
// FM's own card: the words at a glance, in the card's own white. A click
// opens them in the lyrics pane.
func (a *app) fmLyricPeek(c *ui.Context) {
	pl := a.pl
	lines, loaded := pl.Lyrics()
	if !loaded || len(lines) == 0 || pl.Current() == nil {
		return
	}
	pos := pl.Position()
	cur := lyrics.Index(lines, pos)
	if pl.Playing() && cur+1 < len(lines) {
		c.After(lines[cur+1].At - pos + 20*time.Millisecond)
	}
	sung := ""
	if cur >= 0 {
		sung = strings.TrimSpace(lines[cur].Text)
	}
	next := ""
	for i := cur + 1; i < len(lines) && next == ""; i++ {
		next = strings.TrimSpace(lines[i].Text)
	}
	if sung == "" && next == "" {
		return
	}
	row := ui.ButtonBase(c.Key("fm-lyric")).Column().AlignItems(ui.Start).Gap(3).
		Margin(16, 0, 0, 0).Padding(8, 10).Radius(10).FocusRing(false).Label("歌词速览")
	bg := ui.Transparent
	if row.Hovered() {
		bg = white.Alpha(0.08)
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
	row.Children(func() {
		if sung != "" {
			ui.Text(c, sung).FontSize(13).FontWeight(600).TextColor(white.Alpha(0.92)).SingleLine()
		}
		if next != "" {
			ui.Text(c, next).FontSize(12).TextColor(white.Alpha(0.55)).SingleLine()
		}
	})
	if row.Clicked() {
		a.nowPlaying, a.npPane = true, paneLyrics
	}
}

// fmRecent lists the songs the personal FM just played, so the listener
// may return to one: the FM flows on and forgets them otherwise. A click
// jumps back to that song in the radio's queue.
func (a *app) fmRecent(c *ui.Context) {
	pl := a.pl
	radio := pl.Radio()
	if radio == "" || (radio != heartRadio && !strings.HasPrefix(radio, "fm:")) {
		return
	}
	order, at := pl.PlayOrder()
	if at <= 0 {
		return
	}
	var idx []int
	for k := at - 1; k >= 0 && len(idx) < 6; k-- {
		idx = append(idx, order[k])
	}
	if len(idx) == 0 {
		return
	}
	a.section(c, "刚刚播过")
	p := a.pal
	ui.Column(c).Gap(8).Children(func() {
		for _, qi := range idx {
			tr, at := &pl.Queue()[qi], qi
			card := ui.ButtonBase(c.Key("fmr" + strconv.Itoa(at))).Padding(8).Radius(12).Gap(12).
				AlignItems(ui.Center).FocusRing(false).Label(tr.Name)
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
					ui.Text(c, tr.Name).FontSize(13).FontWeight(600).TextColor(p.text).SingleLine()
					ui.Text(c, tr.ArtistNames()).FontSize(11.5).TextColor(p.muted).SingleLine()
				})
				if a.profile != nil {
					a.likeButton(c.Key("fmrl"+strconv.Itoa(at)), tr, 16, p.muted)
				}
			})
			if card.Clicked() {
				pl.Jump(at)
			}
		}
	})
}
