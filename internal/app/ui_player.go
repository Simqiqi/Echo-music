package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/player"
)

// scrubState is a progress bar's own: the value dragged, while it is, and
// the levels of the music its smoke follows.
type scrubState struct {
	v float64
	// held is set while the pointer pressed on the bar is down, wherever
	// it has gone since: Pressed is false off the bar, which is thin.
	held   bool
	active bool // dragged since the last frame, to seek once let go
	level  barState
}

func secs(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// scrubber shows how far the track is, and seeks where it is dragged or
// clicked: the bar thickens under the pointer, and its knob shows. With a
// smoke color, the bar is a comet while music plays: what has played is a
// streak brightening to the playhead, which glows and trails smoke that
// rises and fades, thicker as the music is louder.
func (a *app) scrubber(c *ui.Context, st *scrubState, track, fill ui.Color, height float32, smoke ui.Color) ui.Element {
	pl := a.pl
	dur := pl.Duration().Seconds()
	// A stream that never told us its length has no position to seek to:
	// seeking would land on max(0, dur-1) == 0 and start the song over.
	seekable := dur > 0
	if !st.active && !st.held {
		st.v = pl.Position().Seconds()
	}
	s := ui.SliderBase(c, &st.v, 0, max(dur, 1)).Height(height).FocusRing(false).Label("播放进度").Cursor(ui.CursorPointer)
	if !seekable {
		s.Disabled(true)
	}
	// The press and its release come to the bar wherever the pointer is,
	// and go on to the slider, which follows the pointer.
	s.HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			st.held = ev.Button == 0
		case ui.InputPointerUp:
			st.held = false
		}
		return false
	})
	pressed := st.held
	switch {
	case pressed:
		st.active = true
	case st.active:
		// Let go: there, but not right at the end, which would end the
		// song as it begins to play.
		st.active = false
		if seekable {
			pl.Seek(secs(min(st.v, max(0, dur-1))))
		}
	case s.Changed():
		if seekable {
			pl.Seek(secs(min(st.v, max(0, dur-1))))
		}
	}
	hot := s.Hovered() || pressed
	thick := float32(4)
	if hot {
		thick = 6
	}
	thick = s.AnimateTo("thick", thick, ui.Snappy)
	knob := float32(0)
	if hot {
		knob = 1
	}
	knob = s.AnimateTo("knob", knob, ui.Snappy)
	// The smoke settles away as the music pauses, and while dragging.
	smoking := float32(0)
	if smoke.A > 0 && a.settings.Smoke && pl.Playing() && !pl.Loading() && !st.active {
		smoking = 1
	}
	smoking = s.AnimateTo("smoke", smoking, ui.Animation{Duration: 600 * time.Millisecond})
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		at := st.v
		if !st.active {
			at = pl.Position().Seconds()
		}
		frac := float32(0)
		if dur > 0 {
			frac = float32(min(1, max(0, at/dur)))
		}
		y := r.Y + (r.H-thick)/2
		head, cy := r.X+r.W*frac, r.Y+r.H/2
		p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: thick}, track, thick/2)
		// What has downloaded, a shade lighter.
		if got, ok := pl.Downloaded(); ok && float32(got) < 1 {
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W * float32(got), H: thick}, track, thick/2)
		}
		played := ui.Rect{X: r.X, Y: y, W: max(thick, head-r.X), H: thick}
		if smoking > 0.01 {
			// A streak of light, brightest at the head.
			p.FillGradient(played, ui.LinearGradient{Angle: 90, Oklab: true,
				From: fill.Mix(smoke, smoking).Alpha(0.3), To: fill.Mix(smoke, 0.8*smoking)}, thick/2)
			lv, live := pl.Levels()
			st.level.step(p.Now(), lv, live)
			bass := st.level.group(0, 6)
			drawSmoke(p, head, cy, p.Now(), bass, smoke, smoking)
			// The glow of the head, swelling with the bass, and its core.
			glow := 9 + 8*bass
			p.Shadow(ui.Rect{X: head, Y: cy}, 0, 0, 0, glow, 2.5+2.5*bass, smoke.Alpha(0.95*smoking))
			core := thick * (1 + 0.25*bass)
			p.Fill(ui.Rect{X: head - core/2, Y: cy - core/2, W: core, H: core}, white.Mix(smoke, 0.15).Alpha(smoking), core/2)
		} else {
			p.Fill(played, fill, thick/2)
		}
		if knob > 0.01 {
			k := 12 * knob
			p.Shadow(ui.Rect{X: head - k/2, Y: r.Y + (r.H-k)/2, W: k, H: k}, k/2, 0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.3))
			p.Fill(ui.Rect{X: head - k/2, Y: r.Y + (r.H-k)/2, W: k, H: k}, fill, k/2)
		}
		switch {
		case smoking > 0.01 && (pl.Playing() || smoking < 1):
			// The smoke drifts slowly, under a point a frame at 30 frames
			// a second, which show it smooth for a quarter of what a 120 Hz
			// display would draw.
			p.After(time.Second / 30)
		case pl.Playing() && !st.active && dur > 0:
			// Again when the fill has moved a pixel of the screen, which
			// takes a good part of a second, rather than every frame.
			perPixel := dur / float64(r.W*p.Scale())
			p.After(max(16*time.Millisecond, secs(perPixel)))
		}
	})
	return s
}

// Smoke from the playhead: wisps shed one after another, each drifting
// back along the bar and rising, growing and fading over its life. Where a
// wisp is follows from the time alone, so nothing is kept between frames.
const (
	smokeLife  = 1.4 // seconds
	smokeWisps = 36  // alive at once, at most
)

// drawSmoke draws the smoke of a playhead at (x, y) at now, thicker as
// loud (0 to 1) is higher, at opacity shown.
func drawSmoke(p *ui.Painter, x, y float32, now time.Time, loud float32, color ui.Color, shown float32) {
	t := float64(now.UnixMilli()) / 1000
	interval := smokeLife / smokeWisps
	first := math.Floor(t / interval)
	for j := range smokeWisps {
		k := first - float64(j)
		age := float32(t - k*interval)
		if age < 0 || age > smokeLife {
			continue
		}
		// A wisp of its own: how fast it drifts and rises, how it curls,
		// how large it grows. Quiet music sheds fewer of them.
		r := func(o float64) float32 { return lineRandom(k, o) }
		if r(5) > 0.6+0.4*loud {
			continue
		}
		f := age / smokeLife
		wx := x - age*(12+22*r(1))
		wy := y - age*(12+18*r(2)) + float32(math.Sin(float64(age)*(2.5+2*float64(r(3)))+float64(r(4))*2*math.Pi))*6*f
		size := 3 + f*(12+10*r(4))
		alpha := 0.5 * float32(math.Pow(float64(1-f), 1.4)) * shown * (0.75 + 0.5*loud)
		p.Shadow(ui.Rect{X: wx, Y: wy}, 0, 0, 0, size, size*0.45, color.Alpha(alpha))
	}
}

// tickTimes redraws the view as the next second of the track begins, for
// the times shown.
func (a *app) tickTimes(c *ui.Context) {
	if a.pl.Playing() && a.pl.Ready() {
		pos := a.pl.Position()
		c.After(time.Second - pos%time.Second + 10*time.Millisecond)
	}
}

// volumeButton is the speaker, which opens the volume in a popover above
// it, as an AppKit popover: a native slider standing up, the volume in
// percent over it. The wheel over the speaker turns it up and down
// without opening it.
func (a *app) volumeButton(c *ui.Context, color ui.Color) {
	b := iconButton(c.Key("volume-button"), volumeIcon(a.volume), 16, color, "音量")
	if b.Clicked() {
		a.volumeOpen = !a.volumeOpen
	}
	b.HandleInput(func(ev ui.InputEvent) bool { return a.scrollVolume(ev) })
	pop := popoverColors(a.pal.dark)
	ui.PopoverBase(c.Key("volume-popover"), b, &a.volumeOpen, func(panel ui.Element) {
		const arrow = 7
		panel.AttachTo(b, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, arrow+3, 0).Width(52).Padding(11, 0, 12, 0).
			Radius(12).Background(pop.fill).Border(0.5, pop.edge).Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.2)).
			AlignItems(ui.Center).Gap(8).Label("音量")
		panel.Draw(func(p *ui.Painter, r ui.Rect) {
			// The arrow pointing at the speaker, its sides edged as the
			// panel is, over the panel's border.
			cx, foot := r.X+r.W/2, r.Y+r.H
			tip := (&ui.Path{}).MoveTo(cx-arrow-1, foot-0.75).LineTo(cx-1.2, foot+arrow-1.2).
				QuadTo(cx, foot+arrow, cx+1.2, foot+arrow-1.2).LineTo(cx+arrow+1, foot-0.75).Close()
			p.FillPath(tip, pop.fill)
			edge := (&ui.Path{}).MoveTo(cx-arrow-1, foot).LineTo(cx-1.2, foot+arrow-1.2).
				QuadTo(cx, foot+arrow, cx+1.2, foot+arrow-1.2).LineTo(cx+arrow+1, foot)
			p.StrokePath(edge, 0.5, pop.edge)
		})
		ui.Textf(c, "%d%%", int(a.volume*100+0.5)).FontSize(11).FontWeight(500).FontFeatures("tnum").TextColor(a.pal.muted)
		a.nativeVolumeSlider(c, pop)
	})
}

// popover holds the colors of a popover as AppKit draws them.
type popover struct {
	fill, edge, track, knob ui.Color
}

func popoverColors(dark bool) popover {
	if dark {
		return popover{fill: ui.Hex("#2b2b2e"), edge: ui.RGBA(255, 255, 255, 0.14), track: ui.RGBA(255, 255, 255, 0.16), knob: ui.Hex("#ececec")}
	}
	return popover{fill: ui.Hex("#f6f6f6"), edge: ui.RGBA(0, 0, 0, 0.12), track: ui.RGBA(0, 0, 0, 0.12), knob: ui.Hex("#ffffff")}
}

// nativeVolumeSlider is a vertical slider as AppKit draws one: a thin
// track, filled with the accent from the foot to a round knob.
func (a *app) nativeVolumeSlider(c *ui.Context, pop popover) {
	const knob, track = 20, 4
	s := ui.SliderBase(c.Key("volume"), &a.volume, 0, 1).Vertical().Size(knob+8, 124).PaddingY(knob / 2).
		Step(0.01).AutoFocus().FocusRing(false).Label("音量")
	if s.Changed() {
		a.pl.SetVolume(a.volume)
	}
	s.HandleInput(func(ev ui.InputEvent) bool { return a.scrollVolume(ev) })
	pressed := s.Pressed()
	v := float32(a.volume)
	accent := a.pal.accent
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		top, foot := r.Y+knob/2, r.Y+r.H-knob/2
		x := r.X + (r.W-track)/2
		y := foot - (foot-top)*v
		p.Fill(ui.Rect{X: x, Y: top - track/2, W: track, H: foot - top + track}, pop.track, track/2)
		p.Fill(ui.Rect{X: x, Y: y, W: track, H: foot - y + track/2}, accent, track/2)
		k := ui.Rect{X: r.X + (r.W-knob)/2, Y: y - knob/2, W: knob, H: knob}
		p.Shadow(k, knob/2, 0, 0.5, 1.5, 0, ui.RGBA(0, 0, 0, 0.3))
		p.Shadow(k, knob/2, 0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.12))
		fill := pop.knob
		if pressed {
			fill = fill.Mix(ui.RGB(0, 0, 0), 0.06)
		}
		p.Fill(k, fill, knob/2)
		p.Stroke(k, ui.RGBA(0, 0, 0, 0.08), knob/2, 0.5)
	})
}

// scrollVolume turns the volume up as the wheel or the fingers scroll up.
func (a *app) scrollVolume(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputScroll {
		return false
	}
	a.volume = min(1, max(0, a.volume-float64(ev.DY)/400))
	a.pl.SetVolume(a.volume)
	return true
}

// volumeIcon is the speaker for volume v: silent, or with one or two waves.
func volumeIcon(v float64) *ui.SVG {
	switch {
	case v == 0:
		return iconMute
	case v < 0.4:
		return iconVolumeLow
	default:
		return iconVolume
	}
}

// muteButton is the speaker, which mutes and unmutes.
func (a *app) muteButton(c *ui.Context, color ui.Color) {
	icon := iconVolume
	if a.volume == 0 {
		icon = iconMute
	}
	if iconButton(c.Key("mute"), icon, 16, color, "静音").Clicked() {
		if a.volume > 0 {
			a.mutedFrom, a.volume = a.volume, 0
		} else {
			a.volume = max(a.mutedFrom, 0.5)
		}
		a.pl.SetVolume(a.volume)
	}
}

// volumeSlider is a slim slider with the speaker before it, which mutes.
func (a *app) volumeSlider(c *ui.Context, color, track ui.Color, width float32) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		a.muteButton(c, color)
		s := ui.SliderBase(c.Key("volume"), &a.volume, 0, 1).Width(width).Height(16).FocusRing(false).Label("音量")
		if s.Changed() {
			a.pl.SetVolume(a.volume)
		}
		hot := s.Hovered() || s.Pressed()
		thick := float32(4)
		if hot {
			thick = 5
		}
		thick = s.AnimateTo("thick", thick, ui.Snappy)
		v := float32(a.volume)
		s.Draw(func(p *ui.Painter, r ui.Rect) {
			y := r.Y + (r.H-thick)/2
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: thick}, track, thick/2)
			p.Fill(ui.Rect{X: r.X, Y: y, W: max(thick, r.W*v), H: thick}, color, thick/2)
		})
	})
}

// transport is the row of controls: shuffle, previous, play, next, repeat.
func (a *app) transport(c *ui.Context, color, dim, on ui.Color, size float32) {
	pl := a.pl
	ui.Row(c).Gap(size * 0.5).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		shuffle := dim
		if pl.Shuffle() {
			shuffle = on
		}
		if iconButton(c.Key("shuffle"), iconShuffle, size*0.62, shuffle, "随机播放").Clicked() {
			pl.ToggleShuffle()
		}
		if iconButton(c.Key("prev"), iconPrev, size*0.85, color, "上一首").Clicked() {
			pl.Prev()
		}
		if playButton(c.Key("play"), pl.Playing(), size*1.25, color).Clicked() {
			pl.TogglePlay()
		}
		if iconButton(c.Key("next"), iconNext, size*0.85, color, "下一首").Clicked() {
			pl.Next()
		}
		repeat, ricon := dim, iconRepeat
		switch pl.Repeat() {
		case player.RepeatAll:
			repeat = on
		case player.RepeatOne:
			repeat, ricon = on, iconRepeat1
		}
		if iconButton(c.Key("repeat"), ricon, size*0.62, repeat, "循环").Clicked() {
			pl.CycleRepeat()
		}
	})
}

// qualityButton is the player bar's picker of stream quality: a small pill
// that names the level the next song will play at and opens the levels to
// choose from. It sits beside the volume, where the ear reaches for it,
// rather than in the settings.
func (a *app) qualityButton(c *ui.Context) {
	p := a.pal
	label, short := qualityLabel(a.settings.Quality)
	chosen := label
	sel := ui.SelectBase(c.Key("quality"), &chosen)
	tr := sel.Trigger.Height(28).PaddingX(8).Gap(5).Radius(9).AlignItems(ui.Center).
		FocusRing(false).Label("音质：" + label)
	bg := ui.Transparent
	if tr.Hovered() || sel.Open() {
		bg = p.hover
	}
	tr.Background(tr.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	chevron := ui.Pose{}
	if sel.Open() {
		chevron.Rotate = 180
	}
	tr.Children(func() {
		ui.Icon(c, iconQuality).Size(14, 14).TextColor(p.muted)
		ui.Text(c, short).FontSize(11.5).FontWeight(600).TextColor(p.text).SingleLine()
		ui.Icon(c, iconDown).Size(11, 11).TextColor(p.faint).AnimatePose(chevron, ui.Snappy)
	})
	sel.Popup(func(panel ui.Element) {
		pop := popoverColors(p.dark)
		panel.MinWidth(216).Padding(6).Radius(13).Background(pop.fill).Border(1, pop.edge).
			Shadow(0, 18, 44, -10, ui.RGBA(0, 0, 0, 0.32)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.97, Y: -4}, Exit: &ui.Motion{Opacity: 0}})
		for _, q := range qualityLevels {
			it := sel.Item(q.label).Padding(8, 10).Radius(9).Gap(8).AlignItems(ui.Center).Label(q.label)
			if it.Highlighted() {
				it.Background(p.hover)
			}
			it.Children(func() {
				ui.Text(c, q.label).FontSize(12.5).Grow(1).SingleLine()
				if q.label == label {
					ui.Icon(c, iconCheck).Size(13, 13).TextColor(p.accent)
				}
			})
		}
		ui.Text(c, "下一首歌曲开始生效").FontSize(11).TextColor(p.faint).Padding(5, 10, 2, 10)
	})
	if tr.Changed() {
		for _, q := range qualityLevels {
			if q.label == chosen {
				a.settings.Quality = q.level
			}
		}
		a.changed()
	}
}

// playerBar runs along the foot of the content: what plays, its controls
// and its progress, and the volume.
func (a *app) playerBar(c *ui.Context) {
	p := a.pal
	pl := a.pl
	tr := pl.Current()
	a.tickTimes(c)
	if a.picked.id != 0 && (tr == nil || tr.ID != a.picked.id || pl.Playing()) {
		// Played, or gone: it says what it is again.
		a.picked = pickedUp{}
	}
	// Liquid Glass: the bar floats as a pane of it, what scrolls under it
	// frosted and bent along its rim; else, while content is under the
	// bar, its edge frosts it, with a hairline along the top.
	liquid := a.settings.glassOn()
	// The foot keeps the drag but no longer draws a line: the progress
	// shows around the capsule's rim instead.
	// Invisible, but keep a real hit area: a hair-thin slider reads a
	// press as its very left end and seeks back to the start.
	ui.Box(c.Key("foot-line")).Absolute().Left(0).Right(0).Bottom(0).Height(18).
		PaddingX(20).Center().Children(func() {
		a.scrubber(c.Key("bar-scrub"), &a.barScrub, ui.Transparent, ui.Transparent, 14, ui.Transparent).Grow(1)
	})
	const capsuleBottom, capsuleHeight = 18, 56
	var shell, hot ui.Element
	if liquid {
		hot = ui.Box(c.Key("bottom-hot")).Absolute().Left(sidebarInset + 12).Right(sidebarInset + 12).
			Bottom(capsuleBottom).Height(capsuleHeight).Background(ui.RGBA(0, 0, 0, 0.002))
		shell = ui.Box(c.Key("bottom-edge")).Absolute().Left(sidebarInset + 12).Right(sidebarInset + 12).
			Bottom(capsuleBottom).Height(capsuleHeight).Radius(capsuleHeight / 2).Material(glass.Glass{})
	} else {
		shell = ui.Box(c.Key("bottom-edge")).Absolute().Left(sidebarInset+12).Right(sidebarInset+12).
			Bottom(capsuleBottom).Height(capsuleHeight).Radius(capsuleHeight/2).
			Background(p.bg).Shadow(0, 10, 28, -12, p.cardShade)
	}
	// The controls keep the middle, between sides that share what is left:
	// narrower, the middle gives up to 140 DIPs, and what plays keeps room
	// for its cover, some of its title and its heart.
	const controls = 4*32 + 78 + 2*4 // four buttons and the quality pill
	bar := ui.Row(c).Absolute().PaddingX(16).Gap(16).AlignItems(ui.Center).
		Left(sidebarInset + 12).Right(sidebarInset + 12).Bottom(capsuleBottom).Height(capsuleHeight)
	bar.Children(func() {
		// What plays.
		ui.Row(c).Grow(1).Basis(0).MinWidth(150).Children(func() {
			if tr == nil {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(38, 38).Radius(8).Background(p.hover).Center().Children(func() {
						ui.Icon(c, iconNote).Size(17, 17).TextColor(p.faint)
					})
					ui.Text(c, "未在播放").FontSize(13).TextColor(p.muted)
				})
				return
			}
			info := ui.ButtonBase(c.Key("info")).Gap(10).AlignItems(ui.Center).FocusRing(false).Shrink(1).MinWidth(0).Label("打开正在播放")
			hovered := info.Hovered()
			info.Children(func() {
				ui.Box(c).Size(38, 38).Shrink(0).Children(func() {
					a.cover(c, tr.Album.PicURL, 120, 38, 8).Shadow(0, 2, 8, 0, p.cardShade)
					if hovered {
						ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(8).Center().
							Background(ui.RGBA(0, 0, 0, 0.35)).TextColor(ui.RGB(255, 255, 255)).Children(func() {
							ui.Icon(c, iconDown).Size(18, 18).Rotate(180)
						})
					}
				})
				ui.Column(c).Shrink(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, tr.Name).FontSize(13.5).FontWeight(600).SingleLine()
					// What it is waiting on shows in place of the artists.
					var sub string
					color := p.muted
					switch {
					case a.picked.id == tr.ID:
						sub = a.picked.heard
					case pl.Loading():
						sub = "正在缓冲…"
					case pl.Err() != "":
						sub, color = pl.Err(), p.accent
					}
					if sub == "" {
						a.artistLinks(c, tr, p.muted, p.text).FontSize(12).SingleLine()
					} else {
						ui.Text(c, sub).FontSize(12).TextColor(color).SingleLine()
					}
				})
			})
			if info.Clicked() {
				a.nowPlaying = true
			}
			a.likeButton(c.Key("like"), tr, 16, p.muted).Margin(0, 0, 0, 4).Shrink(0)
		})

		// The controls sit in the middle of the capsule; the progress has
		// left for the line along the window's foot.
		ui.Column(c).Width(320).Shrink(1).MinWidth(250).Gap(0).AlignItems(ui.Stretch).Children(func() {
			a.transport(c, p.text, p.muted, p.accent, 17)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.Center).Margin(-4, 0, 0, 0).Children(func() {
				ui.Text(c, clock(pl.Position())).FontSize(10.5).FontFeatures("tnum").TextColor(p.faint).Width(34).TextAlign(ui.End)
				ui.Text(c, "/").FontSize(10.5).TextColor(p.faint)
				ui.Text(c, clock(pl.Duration())).FontSize(10.5).FontFeatures("tnum").TextColor(p.faint).Width(34)
			})
		})

		// The words of the moment in the room the buttons leave, then the
		// buttons themselves.
		ui.Row(c).Grow(1).Basis(0).MinWidth(controls).Justify(ui.End).AlignItems(ui.Center).Gap(4).Children(func() {
			a.lyricPeek(c)
			lyr := p.muted
			if a.nowPlaying {
				lyr = p.accent
			}
			label := "歌词"
			if tr != nil && tr.ProgramID != 0 {
				label = "节目简介"
			}
			if iconButton(c.Key("lyrics"), iconLyrics, 16, lyr, label).Clicked() && tr != nil {
				a.nowPlaying, a.npPane = true, a.lyricsPaneChosen()
			}
			// The window itself, put away for a card of the player.
			if mini := iconButton(c.Key("mini"), iconShrink, 15, p.muted, "迷你播放器"); mini.Clicked() && tr != nil {
				a.showMini()
			} else if tr == nil {
				mini.Opacity(0.35).Disabled(true)
			}
			a.sleepButton(c, p.text.Alpha(0.75), p.accent)
			a.volumeButton(c, p.text.Alpha(0.75))
			a.qualityButton(c)
		})
	})
	// The progress runs around the capsule's rim: a quiet track all the
	// way round, and the played part drawn from the top, clockwise, so
	// the glass itself wears the song's time.
	ui.Box(c.Key("play-ring")).Absolute().Left(sidebarInset + 12).Right(sidebarInset + 12).
		Bottom(capsuleBottom).Height(capsuleHeight).Radius(capsuleHeight / 2).PassThrough().
		Draw(func(dr *ui.Painter, r ui.Rect) {
			inset := ui.Rect{X: r.X + 2, Y: r.Y + 2, W: r.W - 4, H: r.H - 4}
			rad := inset.H / 2
			dr.Stroke(inset, p.hairline, rad, 1.4)
			total := ringLen(inset, rad)
			dur := pl.Duration().Seconds()
			start, length := float32(0), float32(0)
			switch {
			case tr == nil:
				// Nothing plays: the rim stays quiet.
			case !pl.Playing():
				// Paused, the rainbow runs the rim back and forth, a
				// short arc going one way and then the other, as a sign
				// the bar waits: never all the way round, as a wheel.
				start, length = sweepAt(c.Now(), total, sweepPause, 0.18)
				dr.After(time.Second / 30)
			case dur > 0:
				// Even a song just begun shows a sliver of color.
				frac := float32(min(1, max(0, pl.Position().Seconds()/dur)))
				length = max(total*frac, total*0.015)
			default:
				// Radio and streams that never tell their length still
				// wear the rainbow: the same arc, travelling faster.
				start, length = sweepAt(c.Now(), total, sweepLive, 0.2)
				dr.After(time.Second / 40)
			}
			if length > 1 {
				// A point every three pixels of rim: sampled at a fixed
				// count, the capsule's tight corners came out as a few
				// straight lines, and the arc visibly kinked as it turned
				// them.
				n := int(min(float32(1600), max(float32(144), length/3)))
				pts := make([][2]float32, 0, n+1)
				for i := 0; i <= n; i++ {
					pts = append(pts, ringAt(start+length*float32(i)/float32(n), inset, rad))
				}
				// A rainbow runs the whole wheel of hues as the ring goes
				// round, starting at the wallpaper's own random hue. Each
				// band shares its ends with the next, so the joins stay
				// continuous.
				const bands = 24
				for b := 0; b < bands; b++ {
					from := b * n / bands
					// One band's end runs into the next, so a bend where
					// they meet wears no notch of a butt cap.
					to := min((b+1)*n/bands+2, n+1)
					hue := wrapHue(a.wash.hue + 360*float32(b)/bands)
					dr.StrokePolyline(pts[from:to], 2.6, ui.Oklch(0.72, 0.16, hue))
				}
			}
			if pl.Playing() && dur > 0 {
				dr.After(120 * time.Millisecond)
			}
		})
	if liquid {
		// Over it, the bar lifts a little, as a pane of glass rising to be
		// touched; the content under it carries on behind.
		lift := float32(0)
		if hot.Hovered() {
			lift = 2
		}
		v := bar.AnimateTo("lift", lift, ui.Animation{Duration: glideDur, Ease: ui.EaseOut})
		shell.Translate(0, -v)
		bar.Translate(0, -v)
	}
}

// The sweeps: how long the rainbow takes to run the rim and come back,
// while the bar is paused, and while a stream that never tells its length
// plays.
const (
	sweepPause = 7 * time.Second
	sweepLive  = 4 * time.Second
)

// sweepAt places an arc of share of the rim that runs it back and forth:
// at now it has gone this far, with a cosine so that it slows and turns at
// each end. It never wraps the start, so it reads as a breath back and
// forth, not a wheel going round.
func sweepAt(now time.Time, total float32, period time.Duration, share float32) (start, length float32) {
	length = total * share
	travel := total - length
	ph := float32(now.UnixNano()%int64(period)) / float32(period)
	start = (0.5 - 0.5*float32(math.Cos(2*math.Pi*float64(ph)))) * travel
	return start, length
}

// ringLen is the perimeter of a capsule of this rect and corner radius.
func ringLen(r ui.Rect, rad float32) float32 {
	return 2*(r.W-2*rad) + 2*(r.H-2*rad) + 2*float32(math.Pi)*rad
}

// ringAt maps a distance walked clockwise from the capsule's top middle
// back onto its rim. The four corners are quarter circles; between them
// the four straight runs. It is what lets a fraction of the song become
// a fraction of the outline, rather than a fraction of a bar.
func ringAt(s float32, r ui.Rect, rad float32) [2]float32 {
	const half = float32(math.Pi / 2)
	arc := half * rad
	segs := [...]float32{
		r.W/2 - rad, arc, r.H - 2*rad, arc,
		r.W - 2*rad, arc, r.H - 2*rad, arc, r.W/2 - rad,
	}
	total := ringLen(r, rad)
	if s < 0 {
		s = 0
	}
	if s > total {
		s = total
	}
	i := 0
	for i < len(segs)-1 && s > segs[i] {
		s -= segs[i]
		i++
	}
	x, y, w, h := r.X, r.Y, r.W, r.H
	cos, sin := func(a float32) float32 { return float32(math.Cos(float64(a))) },
		func(a float32) float32 { return float32(math.Sin(float64(a))) }
	switch i {
	case 0:
		return [2]float32{x + w/2 + s, y}
	case 1:
		a := -half + s/rad
		return [2]float32{x + w - rad + rad*cos(a), y + rad + rad*sin(a)}
	case 2:
		return [2]float32{x + w, y + rad + s}
	case 3:
		a := s / rad
		return [2]float32{x + w - rad + rad*cos(a), y + h - rad + rad*sin(a)}
	case 4:
		return [2]float32{x + w - rad - s, y + h}
	case 5:
		a := half + s/rad
		return [2]float32{x + rad + rad*cos(a), y + h - rad + rad*sin(a)}
	case 6:
		return [2]float32{x, y + h - rad - s}
	case 7:
		a := float32(math.Pi) + s/rad
		return [2]float32{x + rad + rad*cos(a), y + rad + rad*sin(a)}
	default:
		return [2]float32{x + rad + s, y}
	}
}

// miniSpectrumState is what the little spectrum of the bar remembers from
// the frame before: each band of it falls as the music lets it, so that the
// bars breathe instead of flickering.
type miniSpectrumState struct {
	v  [player.Bands]float32
	at time.Time
}

// miniSpectrum is a little spectrum of what plays now, drawn at the left of
// the bar between the song and the controls: a row of thin bars, each
// rising with its band of the music and falling as the band lets go. Still
// when the music is still.
func (a *app) miniSpectrum(c *ui.Context, color ui.Color) {
	st := &a.miniSpec
	box := ui.Box(c.Key("mini-spectrum")).Height(22).Grow(1).MinWidth(30).MaxWidth(120).
		AlignSelf(ui.Center).Margin(0, 8, 0, 4).Shrink(1)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		lv, live := a.pl.Levels()
		dt := float32(min(max(p.Now().Sub(st.at).Seconds(), 0.001), 0.1))
		st.at = p.Now()
		n := len(st.v)
		step := r.W / float32(n)
		bw := max(step*0.55, 1.5)
		for i, v := range lv {
			want := clamp01(v * 1.6)
			prev := st.v[i]
			var cur float32
			if want > prev {
				cur = want // rise at once, fall at ease
			} else {
				cur = max(prev-dt*2.2, want)
			}
			st.v[i] = cur
			h := max(cur*float32(r.H-4), 2)
			x := r.X + float32(i)*step + (step-bw)/2
			p.Fill(ui.Rect{X: x, Y: r.Y + r.H - h, W: bw, H: h}, color.Alpha(0.4+0.6*cur), bw/2)
		}
		if live {
			// While the music plays the spectrum keeps asking for frames.
			p.AnimationFrameRate(30)
		}
	})
}
