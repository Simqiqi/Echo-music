package app

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
)

var white = ui.RGB(255, 255, 255)

var npSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 450 * time.Millisecond, Bounce: 0.08}}

// npSettle is how long Now Playing takes to come or go, its spring at rest.
const npSettle = 600 * time.Millisecond

// npState tracks Now Playing opening and closing, and reports whether it is
// open and whether it has come to rest, open or closed. Call it once a
// build, before the window's content.
func (a *app) npState(c *ui.Context) (open, settled bool) {
	open = a.nowPlaying && a.pl.Current() != nil
	now := c.Now()
	if open != a.npOpen {
		a.npOpen, a.npChangedAt = open, now
		if open {
			// The way back is drawn as the view arrives, and stays
			// while the pointer has only just moved.
			a.npPokeAt = now
		}
		// A build once it is at rest: to stop painting what it covers, or
		// itself once gone.
		c.After(npSettle)
		// The window buttons light over its dark backdrop, and as the
		// theme over the pages (Windows).
		if a.win != nil {
			buttons := mygo.ThemeSystem
			if open {
				buttons = mygo.ThemeDark
			}
			a.win.SetWindowButtonsTheme(buttons)
		}
	}
	return open, now.Sub(a.npChangedAt) >= npSettle
}

// nowPlayingLayer holds Now Playing over the whole window, rising into
// view as it opens and sinking away as it closes. The layer moves itself,
// staying on top all the way, rather than leaving as an exit transition
// would, under the rest of the window: that would show it going over the
// sidebar's material while the page above it hid it at once. Its pose
// moves it in frames that only paint, building nothing.
func (a *app) nowPlayingLayer(c *ui.Context, open, settled bool) {
	layer := ui.Box(c.Key("now-playing-layer")).Absolute().Top(0).Left(0).Right(0).Bottom(0)
	pose := ui.Pose{Y: 60, Fade: 1}
	if open {
		pose = ui.Pose{}
	}
	layer.AnimatePose(pose, npSpring)
	if !open && settled || a.pl.Current() == nil {
		// Closed: nothing to paint or click.
		layer.Invisible()
		a.queueAt, a.wallState.track = -1, 0
		return
	}
	if !open {
		// On its way out, the window under it takes the clicks.
		layer.PassThrough()
	}
	layer.Children(func() { a.nowPlayingView(c) })
}

// nowPlayingView fills the window with what plays: its cover over a slow
// drift of its own colors, the controls, and lyrics that follow the song.
func (a *app) nowPlayingView(c *ui.Context) {
	pl := a.pl
	tr := pl.Current()
	w, h := c.Size()
	art := a.art.get(netease.Sized(tr.Album.PicURL, 600), true)
	// The colors stay the last song's while its cover loads, rather than
	// going gray between the two.
	shown := art
	if art == nil && tr.Album.PicURL != "" {
		shown = a.npArt
	}
	a.npArt = shown
	tint := ui.RGB(120, 120, 130)
	if shown != nil && !shown.failed {
		tint = shown.tint
	}
	accent := tint.Mix(white, 0.45)

	root := ui.Box(c.Key("now-playing")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		Background(ui.Hex("#0b0b0d")).TextColor(white)
	// Without the line, the particles page leans its record after the
	// pointer, wherever in the window it stands: the root sees every move.
	root.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputPointerMove {
			return false
		}
		if a.npPane == paneParticles {
			a.recTilt.tx = clamp01(ev.X/w)*2 - 1
			a.recTilt.ty = clamp01(ev.Y/h)*2 - 1
		}
		return false
	})
	// Asking for the pointer asks for a frame on every move over the view,
	// which is what a move means here: it brings the way back out, and puts
	// its countdown back on the clock. The moves themselves come to the
	// element under the pointer, not to the root, so they are read here.
	pointerIn := false
	if x, y, over := root.PointerPosition(); over {
		pointerIn = true
		if !a.npPokeSet || x != a.npPokeX || y != a.npPokeY {
			a.npPokeX, a.npPokeY, a.npPokeSet = x, y, true
			a.npPokeAt = c.Now()
		}
	}
	root.Children(func() {
		a.backdrop(c, shown, tint)
		// Over the backdrop, which is positioned too: positioned elements
		// paint over those in the flow.
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Children(func() { a.nowPlayingBody(c, tr, art, accent, w, h, pointerIn) })
		// The backdrop's own choice, over everything the view paints, at
		// the top right where nothing else is; and beside it, the switch
		// that keeps the line being sung on the particles page. On Windows
		// the caption buttons stand at the right, so the row keeps clear
		// of them.
		if _, corner, _, _ := a.npBareParts(); !corner {
			ui.Row(c).Absolute().Top(8).Right(12 + c.TitleBar().Right).Gap(8).AlignItems(ui.Center).Children(func() {
				if a.npPane == paneParticles {
					// The three particle controls used to sit here as three
					// wide pills; one menu keeps the corner small so the
					// record has the room.
					a.particleOptionsMenu(c, tint)
				}
				// The backdrop's own choice lives on the pages that read as
				// a backdrop — lyrics, the stage, and the two pickers — and
				// stays out of the way on the full-view pages that paint
				// over it (particles, the kaleidoscope) and on comments.
				switch a.npPane {
				case paneLyrics, paneStage, paneWall, paneWall3D:
					a.backdropPicker(c, tint, "np-backdrop")
				}
			})
		}
	})
}

func (a *app) nowPlayingBody(c *ui.Context, tr *netease.Track, art *artwork, accent ui.Color, w, h float32, pointerIn bool) {
	pl := a.pl
	a.commentsFor(tr)
	tabs, _, foot, _ := a.npBareParts()
	// An episode has what it is about in place of lyrics; a song, lyrics
	// in place of that.
	switch program := tr.ProgramID != 0; {
	case program && (a.npPane == paneWall || a.npPane == paneWall3D || a.npPane == paneParticles ||
		a.npPane == paneKaleido):
		a.npPane = paneProgram
	case program && (a.npPane == paneLyrics || a.npPane == paneStage || a.npPane == paneInk ||
		a.npPane == paneNeon):
		a.npPane = paneProgram
	case !program && a.npPane == paneProgram:
		a.npPane = a.lyricsPaneChosen()
	}
	if a.npPane != paneQueue {
		a.queueAt = -1
	}
	if a.npPane != paneWall {
		a.wallState.track = 0
	}
	if a.npPane != paneWall3D {
		a.wall3.track = 0
	}
	{
		// The top strip drags the window, and stands the way back in its
		// middle: a chevron down, drawn while the pointer moves and put
		// away once it rests, gone altogether while Tab keeps the show
		// alone. Esc is the way back too, for a keyboard.
		ui.Row(c).Height(topBar(c)).DragWindow()
		ui.Row(c).Absolute().Top(6).Left(0).Right(0).Justify(ui.Center).Children(func() {
			a.npBackButton(c, pointerIn && !tabs && a.npBackDrawn(c))
		})
		if a.npPane == paneInk {
			// The page writes the song out: the tabs above, the foot's
			// controls below, and between them nothing but the words.
			reveal(a.npTabsRow(c, w), pointerIn && !tabs).Children(func() { a.paneTabs(c) })
			a.inkPane(c, tr, accent, w)
			if !foot {
				a.stageBar(c, tr, art, accent)
			}
			return
		}
		if a.npPane == paneNeon {
			// The page hangs the song up as a sign at night: the tabs
			// above, the foot's controls below, and between them the tubes.
			reveal(a.npTabsRow(c, w), pointerIn && !tabs).Children(func() { a.paneTabs(c) })
			a.neonPane(c, tr, accent, w)
			if !foot {
				a.stageBar(c, tr, art, accent)
			}
			return
		}
		if a.npPane == paneStage {
			// The stage takes the whole view: the lyrics are the show, the
			// controls a slim bar at the foot. Tab keeps the show alone.
			reveal(a.npTabsRow(c, w), pointerIn && !tabs).Children(func() {
				a.paneTabs(c)
				a.stageStyleButton(c)
			})
			a.stage(c, tr, accent)
			if !foot {
				a.stageBar(c, tr, art, accent)
			}
			return
		}
		if a.npPane == paneWall {
			// The wall takes the whole view too, under the song playing.
			a.wallPane(c, tr, art, accent, w, h, pointerIn)
			return
		}
		if a.npPane == paneWall3D {
			// And so does the wall turning in depth, the posters its own
			// show.
			a.wall3DPane(c, tr, accent, w, h, pointerIn)
			return
		}
		if a.npPane == paneParticles {
			// And so does the record of particles the cover comes apart
			// into.
			a.particlePane(c, tr, art, accent, w, h, pointerIn)
			return
		}
		// The visual panes take the whole view as well: one field
		// of light, with only the words and the tabs over it.
		switch a.npPane {
		case paneKaleido:
			a.visualPane(c, tr, art, accent, w, h, pointerIn, drawKaleido)
			return
		}

		side := float32(math.Min(float64(w)*0.36, 440))
		// The cover leaves room above it for the tabs, starting as far
		// under them as the wall's song does.
		coverSize := min(side, h-436)
		// The column is as wide as the cover, for the cover, what plays
		// and the tabs over it to start at one edge, but wide enough for
		// the controls.
		side = max(coverSize, 280)
		padX := npSideX(w)
		reveal(a.npTabsRow(c, w), pointerIn && !tabs).Children(func() { a.paneTabs(c) })
		ui.Row(c).Grow(1).Padding(0, padX, 44, padX).Gap(72).AlignItems(ui.Stretch).Children(func() {
			var coverTop float32
			ui.Column(c).Width(side).Justify(ui.Center).Gap(0).Children(func() {
				coverTop = ui.Box(c.Key("cover-top")).Height(0).LayoutBounds().Y
				a.npCover(c, tr, art, coverSize)

				// A slim spectrum under the cover, dancing with the music:
				// the same bands the notch shows, drawn thin, and kept as
				// wide as the cover above, so the two sit evenly together
				// and the bars hold clear of the lyrics pane.
				ui.Box(c.Key("spectrum")).Width(coverSize).Height(16).
					AlignSelf(ui.Center).Margin(16, 0, 0, 0).
					Draw(func(p *ui.Painter, r ui.Rect) {
						lv, live := pl.Levels()
						a.npBars.step(p.Now(), lv, live)
						drawBars(p, r, &a.npBars, 24, accent.Mix(white, 0.3), 2)
						if live || a.npBars.group(0, player.Bands) > 0.01 {
							p.AnimationFrameRate(30)
						}
					})

				// What plays, its progress and its controls, on the
				// backdrop itself rather than on a pane over it.
				info := ui.Column(c).Gap(0)
				info.Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(16, 0, 14, 0).Children(func() {
						ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
							if r := pl.Radio(); r != "" {
								a.radioChip(c, r)
							}
							ui.Text(c, tr.Name).FontSize(22).FontWeight(700).LetterSpacing(-0.3).SingleLine()
							a.artistLinks(c, tr, white.Alpha(0.6), white, true).FontSize(15).SingleLine()
						})
						a.vinylButton(c.Key("vinyl"), accent)
						a.likeButton(c.Key("like"), tr, 22, white.Alpha(0.7))
						a.songMenu(c.Key("more"), tr, 22, white.Alpha(0.7)).Margin(0, -8, 0, 0)
					})
					a.scrubber(c.Key("np-scrub"), &a.npScrub, white.Alpha(0.18), white.Alpha(0.9), 16, accent.Mix(white, 0.35))
					// As tall with the quality's badge as without it: the badge goes
					// while the next song loads, and the column would move with it.
					ui.Row(c).Height(16).AlignItems(ui.Center).Children(func() {
						ui.Text(c, clock(pl.Position())).FontSize(11).FontFeatures("tnum").TextColor(white.Alpha(0.5))
						ui.Spacer(c)
						qualityBadge(c, pl.Stream())
						ui.Spacer(c)
						status := "-" + clock(pl.Duration()-pl.Position())
						if pl.Loading() {
							status = "正在缓冲…"
						}
						ui.Text(c, status).FontSize(11).FontFeatures("tnum").TextColor(white.Alpha(0.5))
					})
					ui.Box(c).Margin(14, 0, 18, 0).Children(func() {
						a.transport(c, white, white.Alpha(0.45), accent, 28)
					})
					ui.Row(c).Justify(ui.Center).Children(func() {
						a.volumeSlider(c, white.Alpha(0.7), white.Alpha(0.18), 180)
					})
				})
			})

			pane := ui.Column(c.Key("pane")).Grow(1).Basis(0).MinWidth(0)
			pane.Children(func() {
				// The pane starts level with the cover beside it, but below the
				// tabs, which start level with the pane's top: they come and go
				// with the pointer, so they sit out of the flow, never over it.
				b := pane.LayoutBounds()
				if b.H > 0 && coverTop > 0 {
					if top := max(coverTop-b.Y, npTabsHeight+8); top != a.npPaneTop {
						a.npPaneTop = top
						c.AnimationFrame()
					}
				} else {
					c.AnimationFrame() // not laid out yet: the next frame places it
				}
				ui.Box(c).Height(a.npPaneTop)
				switch a.npPane {
				case paneLyrics:
					a.lyricsPane(c, tr, h)
				case paneComments:
					a.commentsPane(c, tr)
				case paneProgram:
					a.programPane(c, tr)
				default:
					a.queuePane(c)
				}
			})
		})
	}
}

// npMaxWidth is how wide the cover and the pane beside it go, at most.
const npMaxWidth = 1280

// npSideX is how far in from the sides of a view w wide the cover and the
// pane beside it are: in a wide window they keep together in the middle,
// rather than the pane taking all the width the cover leaves.
func npSideX(w float32) float32 {
	return max(72, (w-npMaxWidth)/2)
}

// npTabsHeight is how tall the row of Now Playing's tabs is, for what is
// under it to start below it.
const npTabsHeight = 28

// npTabsWidth measures the row as it is drawn: every label at 13pt semibold,
// its padding on either side, the gap between them and the glass capsule's
// own padding. The comments' count changes with each song — "评论 32.2万"
// is far wider than "评论" — so a fixed width used to crop the last tab as
// the count grew. Measuring keeps the capsule, the hover probe and the clip
// all wide enough, and on a narrow view the tabs' padding gives way first so
// nothing is cut.
func (a *app) npTabsWidth(c *ui.Context, w float32) float32 {
	const gap = float32(6)
	const capsule = float32(12) // PaddingX(6) at both ends
	// A little air for the counter's rolled glyphs and font rounding.
	const spare = float32(8)
	labels := make([]string, 0, len(a.npTabs()))
	key := ""
	for _, t := range a.npTabs() {
		label := t.label
		if t.pane == paneComments {
			label = a.commentsTab()
		}
		labels = append(labels, label)
		key += label + "\x00"
	}
	room := max(float32(200), w-2*npSideX(w)-16)
	if key == a.npTabWidthKey && room == a.npTabWidthRoom {
		return a.npTabWidthVal
	}
	textW := float32(0)
	for _, label := range labels {
		tw, _ := c.MeasureText(0, ui.Span{Text: label, Size: 13, Weight: 600})
		textW += tw
	}
	measure := func(padX float32) float32 {
		return capsule + textW + 2*padX*float32(len(labels)) +
			gap*float32(max(0, len(labels)-1)) + spare
	}
	padX := float32(14)
	for padX > 8 && measure(padX) > room {
		padX -= 2
	}
	a.npTabPadX = padX
	a.npTabWidthKey, a.npTabWidthRoom = key, room
	a.npTabWidthVal = min(measure(padX), room)
	return a.npTabWidthVal
}

// npTabsRow is the row of Now Playing's tabs, in the same place for every
// pane, so that they stay put going from one to another: at the top, level
// with the cover's left. They start from the left, so that only the last,
// the comments, moves as its count changes. With Liquid Glass on, one
// capsule of it floats under the whole row: the cover and the effects
// behind frost along its rim, without moving the tabs by a pixel.
func (a *app) npTabsRow(c *ui.Context, w float32) ui.Element {
	row := ui.Row(c.Key("tabs")).Absolute().Top(topBar(c)).Left(npSideX(w)).Height(npTabsHeight).Gap(6).AlignItems(ui.Center)
	a.npTabRowX = row.LayoutBounds().X
	// Two seconds after the pointer leaves, the row draws itself in from
	// the right until only a small dot remains; moving back over where
	// the row was opens it again. Only a real window folds: a test needs
	// the tabs where they are to click them. Folded, the labels are not
	// built at all, so not so much as a character peeks out of the dot.
	a.npTabsV = 1
	tabsFullW := a.npTabsWidth(c, w)
	if a.win != nil && a.settings.tabsAutoHide() && !a.npBare {
		probe := ui.Box(c.Key("tabs-probe")).Absolute().Top(topBar(c) - 10).
			Left(npSideX(w) - 10).Width(tabsFullW + 20).Height(npTabsHeight + 20).PassThrough()
		x, y, over := probe.PointerPosition()
		hovered := over && x >= 0 && y >= 0
		if hovered {
			a.npTabsLeft = time.Time{}
		} else if a.npTabsLeft.IsZero() {
			a.npTabsLeft = c.Now()
		}
		shown := hovered || (!a.npTabsLeft.IsZero() && c.Now().Sub(a.npTabsLeft) < 2*time.Second)
		target := float32(0)
		if shown {
			target = 1
		}
		v := row.AnimateTo("tabs-fold", target,
			ui.Animation{Duration: glideDur, Ease: ui.EaseOut})
		a.npTabsV = v
		row.Width(max(1, tabsFullW*v)).Clip()
		if v < 0.99 {
			ui.Box(c.Key("tabs-dot")).Absolute().Left(npSideX(w)).
				Top(topBar(c)+(npTabsHeight-10)/2).Size(10, 10).Radius(5).
				Background(a.pal.accent).Opacity(1 - v).PassThrough()
		}
	}
	// The capsule goes in with the row it holds: folded, only the dot is
	// left, rather than a sliver of glass beside it. Its padding shrinks
	// with the row too, so nothing of it outlives the labels.
	if a.settings.glassOn() && a.npTabsV > 0.02 {
		v := a.npTabsV
		row.PaddingX(6*v).Radius(npTabsHeight/2).
			Material(glass.Glass{Clarity: a.glassClarity(0.45), Tint: ui.RGBA(255, 255, 255, 0.07*v)}).
			Shadow(0, 6, 18, -8, ui.RGBA(0, 0, 0, 0.18*v))
	}
	// The row takes the press and keeps it, so a choice can be carried
	// along with the pointer: sliding across picks whatever tab is under
	// it, and letting go picks the one under it, which is also a click.
	row.HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			a.npTabDrag = true
			return true
		case ui.InputPointerMove:
			if !a.npTabDrag {
				return false
			}
			if pane := a.npTabAt(ev.X); pane >= 0 && pane != a.npPane {
				a.choosePane(pane)
			}
			return true
		case ui.InputPointerUp:
			if !a.npTabDrag {
				return false
			}
			a.npTabDrag = false
			if pane := a.npTabAt(ev.X); pane >= 0 {
				a.choosePane(pane)
			}
			return true
		}
		return false
	})
	return row
}

// npTabAt is the tab under x, a pointer's position inside the row.
func (a *app) npTabAt(x float32) int {
	wx := x + a.npTabRowX
	for k, box := range a.npTabBoxes {
		if wx >= box.X && wx <= box.X+box.W {
			return a.npTabPanes[k]
		}
	}
	return -1
}

// reveal fades e in while the pointer is in the window, and out when it
// leaves.
func reveal(e ui.Element, shown bool) ui.Element {
	pose := ui.Pose{Fade: 1}
	if shown {
		pose = ui.Pose{}
	}
	return e.AnimatePose(pose, ui.Animation{Duration: 200 * time.Millisecond})
}

// npBackIdle is how long the way back stays drawn once the pointer stops
// moving.
const npBackIdle = 2 * time.Second

// npBackDrawn says whether the way back is out this build: while the
// pointer moves over the view, and a moment after it rests. The countdown
// puts one build on the clock, the one that puts it away: a build for
// every frame of the countdown would be frames the view does not need.
func (a *app) npBackDrawn(c *ui.Context) bool {
	if a.npPokeAt.IsZero() {
		return false
	}
	now := c.Now()
	until := a.npPokeAt.Add(npBackIdle)
	if !now.Before(until) {
		return false
	}
	if a.npBackUntil != until {
		a.npBackUntil = until
		c.After(until.Sub(now))
	}
	return true
}

// npBackButton is Now Playing's way back to the page under it: a chevron
// down at the middle of the top strip, a bead of glass over the words. It
// goes where it came from by itself: away once the pointer rests, and
// under Tab with the rest of the chrome. Faded out, the clicks pass
// through it, so the strip still drags the window there.
func (a *app) npBackButton(c *ui.Context, drawn bool) {
	b := ui.ButtonBase(c.Key("np-back")).Size(36, 36).Center().Radius(999).Label("返回")
	// A press holds it out until it is let go: the click it carries would
	// go with it if it went away under the button.
	if b.Pressed() {
		drawn = true
	}
	fill := white.Alpha(0.10)
	if b.Hovered() {
		fill = white.Alpha(0.20)
	}
	b.Background(b.AnimateColor("bg", fill, ui.Animation{Duration: 130 * time.Millisecond}))
	pose := ui.Pose{Fade: 1}
	if drawn {
		pose = ui.Pose{}
	}
	if b.Pressed() {
		pose.Scale = 0.9
	}
	b.AnimatePose(pose, ui.Animation{Duration: 200 * time.Millisecond})
	if !drawn {
		b.PassThrough()
	}
	// The icon lets the pointer through to the button it stands on, which
	// is what hears the click; faded out, both let it through.
	b.Children(func() { ui.Icon(c, iconDown).Size(18, 18).TextColor(white.Alpha(0.92)).PassThrough() })
	if b.Clicked() {
		a.nowPlaying = false
	}
}

// lyricsPaneChosen is the pane of lyrics the listener chose last: what
// the app starts on, before the player is made. Now Playing shows an
// episode's in place of either.
func (a *app) lyricsPaneChosen() int {
	if a.settings.LyricsStage {
		return paneStage
	}
	return paneLyrics
}

// The panes of Now Playing.
const (
	paneLyrics = iota
	paneQueue
	paneStage
	paneComments
	paneProgram
	paneWall
	paneWall3D
	paneParticles
	// The shader-painted visual pane.
	paneKaleido
	// The lyrics written out as ink.
	paneInk
	// The lyrics hung up as a neon sign.
	paneNeon
)

// npTab is a tab of Now Playing: its pane, and what it says.
type npTab struct {
	pane  int
	label string
}

// npTabs returns the tabs of Now Playing, in order, for what plays: ⌘1
// chooses the first, ⌘2 the second, and on.
func (a *app) npTabs() []npTab {
	if tr := a.pl.Current(); tr != nil && tr.ProgramID != 0 {
		return []npTab{{paneProgram, "简介"}, {paneComments, "评论"}}
	}
	return []npTab{{paneLyrics, "歌词"}, {paneStage, "动态"}, {paneInk, "墨迹"},
		{paneNeon, "霓虹"},
		{paneWall, "选歌"}, {paneWall3D, "海报墙"}, {paneParticles, "粒子"},
		{paneKaleido, "万花筒"},
		{paneComments, "评论"}}
}

// npBareParts says what Tab has put away on this pane. The row of tabs
// and the backdrop choice go on every pane; the foot's song and controls
// go as well on the visual panes, where only the show and its words stay.
// The poster wall keeps nothing but its posters.
func (a *app) npBareParts() (tabs, corner, foot, line bool) {
	if !a.npBare {
		return false, false, false, false
	}
	switch a.npPane {
	case paneStage, paneInk, paneNeon, paneWall, paneParticles:
		return true, true, true, false
	case paneWall3D:
		return true, true, true, true
	default:
		return true, true, false, false
	}
}

func (a *app) paneTabs(c *ui.Context) {
	// The row's input handler reads these boxes on the next pointer move,
	// so they are collected afresh with every build.
	a.npTabBoxes = a.npTabBoxes[:0]
	a.npTabPanes = a.npTabPanes[:0]
	if a.npTabsV < 0.35 {
		return
	}
	for _, t := range a.npTabs() {
		label := t.label
		if t.pane == paneComments {
			label = a.commentsTab()
		}
		a.paneTab(c, t.pane, label)
	}
}

// choosePane shows pane i in Now Playing, remembering lyrics or the stage,
// whichever was chosen, for the next song.
func (a *app) choosePane(i int) {
	// The record opens square to the eye: a lean left over from the page
	// before is not the lean of this one.
	if i == paneParticles && a.npPane != paneParticles {
		a.recTilt = tiltState{}
	}
	a.npPane = i
	if (i == paneLyrics || i == paneStage) && a.settings.LyricsStage != (i == paneStage) {
		a.settings.LyricsStage = i == paneStage
		a.settings.save()
	}
}

// radioChip says which radio plays, over the title: a click on it, or on
// its cross, leaves the radio, playing on the song.
func (a *app) radioChip(c *ui.Context, name string) {
	icon, exit := iconRadio, "退出私人 FM"
	if name == heartRadio {
		icon, exit = iconHeartFill, "退出心动模式"
	}
	chip := ui.ButtonBase(c.Key("radio-chip")).Gap(5).Padding(3, 6, 3, 8).Margin(0, 0, 4, -8).Radius(999).
		AlignItems(ui.Center).AlignSelf(ui.Start).FocusRing(false).Label(exit).Tooltip(exit)
	bg, fg, cross := ui.Transparent, white.Alpha(0.6), float32(0.5)
	if chip.Hovered() {
		bg, fg, cross = white.Alpha(0.12), white.Alpha(0.85), 1
	}
	chip.Background(chip.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond})).TextColor(fg)
	chip.Children(func() {
		ui.Icon(c, icon).Size(12, 12)
		ui.Text(c, radioLabel(name)).FontSize(11.5).FontWeight(700).LetterSpacing(0.3)
		ui.Icon(c, iconX).Size(11, 11).Opacity(cross)
	})
	if chip.Clicked() {
		a.exitRadio()
	}
}

// qualityBadge marks a lossless stream, as it plays.
func qualityBadge(c *ui.Context, st *netease.Stream) {
	if st == nil || !netease.Lossless(st.Level) {
		return
	}
	label := "无损"
	if st.Level == "hires" {
		label = "Hi-Res"
	}
	ui.Text(c, label).FontSize(9.5).FontWeight(800).LetterSpacing(0.6).TextColor(ui.Hex("#f5d58a")).
		Padding(1, 6).Radius(4).Border(1, ui.Hex("#f5d58a").Alpha(0.55))
}

// stageBar is the slim bar along the foot of the stage: what plays, where
// it is, and the controls.
func (a *app) stageBar(c *ui.Context, tr *netease.Track, art *artwork, accent ui.Color) {
	pl := a.pl
	ui.Column(c).Padding(0, 48, 28, 48).Gap(10).Children(func() {
		a.scrubber(c.Key("stage-scrub"), &a.npScrub, white.Alpha(0.18), white.Alpha(0.9), 14, accent.Mix(white, 0.35))
		ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(12).AlignItems(ui.Center).Children(func() {
				if bmp := bitmap(art); bmp != nil {
					cov := ui.Box(c.Key(tr.ID)).Size(44, 44)
					in := artIn(cov, bmp)
					cov.Children(func() { artImage(c, bmp, 8, in) })
				}
				ui.Column(c).Shrink(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, tr.Name).FontSize(14).FontWeight(700).SingleLine()
					a.artistLinks(c, tr, white.Alpha(0.55), white).FontSize(12).SingleLine()
				})
				a.likeButton(c.Key("like"), tr, 17, white.Alpha(0.6))
				a.songMenu(c.Key("more"), tr, 17, white.Alpha(0.6)).Margin(0, 0, 0, -10)
			})
			a.transport(c, white, white.Alpha(0.45), white, 22)
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Justify(ui.End).Children(func() {
				ui.Text(c, clock(pl.Position())+" / "+clock(pl.Duration())).FontSize(11.5).FontFeatures("tnum").TextColor(white.Alpha(0.5))
			})
		})
	})
}

func (a *app) paneTab(c *ui.Context, i int, label string) {
	padX := a.npTabPadX
	if padX <= 0 {
		padX = 14
	}
	b := ui.ButtonBase(c.Key(i)).Padding(6, padX).Radius(999).FocusRing(false).Label(label)
	bg, fg := ui.Transparent, white.Alpha(0.55)
	if a.npPane == i {
		bg, fg = white.Alpha(0.14), white
	} else if b.Hovered() || b.Pressed() {
		bg = white.Alpha(0.06)
	}
	a.npTabBoxes = append(a.npTabBoxes, b.LayoutBounds())
	a.npTabPanes = append(a.npTabPanes, i)
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	b.Children(func() {
		if i != paneComments {
			ui.Text(c, label).FontSize(13).FontWeight(600).TextColor(fg)
			return
		}
		// The count rolls as it changes, from one song's to the next.
		n, text := a.commentCount(), ""
		if n > 0 {
			text = " " + count(n)
		}
		ui.Text(c, "评论").FontSize(13).FontWeight(600).TextColor(fg)
		rollCount(c, &a.tabCount, n, text, fg)
	})
	if b.Clicked() {
		a.choosePane(i)
	}
}

func (a *app) lyricsPane(c *ui.Context, tr *netease.Track, h float32) {
	pl := a.pl
	lines, loaded := pl.Lyrics()
	if len(lines) == 0 {
		col := ui.Column(c).Grow(1)
		if loaded {
			col.Center()
		}
		dim, _ := a.quietShown(c, col, tr, lines, loaded)
		quietUnder(col, dim)
		col.Children(func() {
			if !loaded {
				lyricsSkeleton(c, npShimmer(c))
				return
			}
			ui.Text(c, "纯音乐，请欣赏").FontSize(24).FontWeight(700).TextColor(white.Alpha(0.4))
		})
		a.quietCard(c, dim, h*0.18)
		return
	}
	pos := pl.Position()
	cur := lyrics.Index(lines, pos)
	// Redraw as the next line begins.
	if pl.Playing() && cur+1 < len(lines) {
		c.After(lines[cur+1].At - pos + 20*time.Millisecond)
	}
	if a.lyricTrack != tr.ID {
		a.lyricTrack, a.lyricAt = tr.ID, -2
		a.lyricScroll, a.lyricGlide = ui.ScrollState{}, lyricGlide{}
	}
	lag := a.lyricGlide.step(c, &a.lyricScroll)

	// No scrollbar here: the lyrics glide on their own, and a bar at the
	// pane's edge only cluttered them; pushed past the window, it never
	// shows.
	scroll := ui.Scroll(c.Key("lyrics")).Grow(1).TrackScroll(&a.lyricScroll).Padding(0, 8, h*0.5, 0).ScrollbarInsets(0, -40, 0, 0)
	dim, quiet := a.quietShown(c, scroll, tr, lines, loaded)
	if dim > 0.98 {
		// Under the hot comment, gone, not drawn faint.
		scroll.Invisible()
	}
	defer a.quietCard(c, dim, h*0.18)
	a.timedLines(c, scroll, lines, pos, cur, lag, linesStyle{size: 30, weight: 800, lineHeight: 1.25, quiet: quiet}, nil)
	// A card of these words, for sharing, over the pane's top right.
	a.shareLyricsButton(c)
}

// linesStyle is how timedLines sets its lines: the size, weight and
// leading of their type; quiet nearly hides them, under a hot comment.
type linesStyle struct {
	size       float32
	weight     int
	lineHeight float32
	quiet      bool
}

// charKey tells apart the characters of a sung line's words, for the
// fill that sweeps through them.
type charKey struct {
	word, char int
}

// timedLines fills scroll with lines that follow the track playing at
// pos: the line at cur lit, and brought to a third of the way down once
// it changes, but none with cur -1 before the first; a click on one seeks
// to it. The lines are held back by lag, how far the glide to the line
// lit has still to go (lyricGlide). foot, if set, follows the lines.
func (a *app) timedLines(c *ui.Context, scroll ui.Element, lines []lyrics.Line, pos time.Duration, cur int, lag float32, st linesStyle, foot func()) {
	pl := a.pl
	var active ui.Element
	scroll.Children(func() {
		ui.Column(c).Gap(6).Translate(0, -lag).Children(func() {
			for i, l := range lines {
				line := ui.ButtonBase(c.Key(i)).Column().AlignItems(ui.Start).Padding(10, 12).Radius(12).Gap(6).FocusRing(false).Label(l.Text)
				on := i == cur
				alpha := float32(0.28)
				if on {
					alpha = 1
				} else if line.Hovered() {
					alpha = 0.55
				}
				if st.quiet {
					// Nearly gone under a hot comment.
					alpha *= quietFaded
				}
				if line.Hovered() {
					line.Background(white.Alpha(0.06))
				}
				scale := float32(0.94)
				if on {
					scale = 1
				}
				line.TransformOrigin(0, 0.5).AnimatePose(ui.Pose{Scale: scale}, ui.Animation{Spring: ui.SpringOptions{Duration: 600 * time.Millisecond, Bounce: 0.1}})
				line.Children(func() {
					// The line fades as a pose, in frames that only paint, rather
					// than building the view at the display's rate as it does.
					text := ui.Column(c).AlignItems(ui.Start).Gap(6)
					text.AnimatePose(ui.Pose{Fade: 1 - alpha}, ui.Animation{Duration: 380 * time.Millisecond})
					text.Children(func() {
						if on && len(l.Words) > 0 {
							// The line sung fills character by character, as
							// karaoke does: each word knows when it starts
							// and ends, and the fill sweeps through its
							// characters in that time, rather than lighting
							// the word whole at once.
							ui.RichText(c.Key("words")).FontSize(st.size).FontWeight(st.weight).LineHeight(st.lineHeight).LetterSpacing(-0.4).Children(func() {
								for wi, wd := range l.Words {
									p := clamp01(float32((pos - wd.Start).Seconds() / max(0.05, (wd.End-wd.Start).Seconds())))
									runes := []rune(wd.Text)
									for ci, ch := range runes {
										// How far the fill has come through
										// this character of the word.
										lit := clamp01(p*float32(len(runes)) - float32(ci))
										ui.Text(c.Key(charKey{wi, ci}), string(ch)).TextColor(white.Alpha(0.38 + 0.62*lit))
									}
								}
							})
							if pl.Playing() {
								// Words brighten smoothly enough at 25 frames a second.
								c.After(40 * time.Millisecond)
							}
						} else {
							ui.Text(c, l.Text).FontSize(st.size).FontWeight(st.weight).LineHeight(st.lineHeight).LetterSpacing(-0.4).TextColor(white)
						}
						if l.Translation != "" {
							ui.Text(c, l.Translation).FontSize(17).FontWeight(600).TextColor(white.Alpha(0.7))
						}
					})
				})
				if line.Clicked() {
					pl.Seek(l.At)
				}
				if on {
					active = line
				}
			}
		})
		if foot != nil {
			foot()
		}
	})
	// Bring the line sung to a third of the way down, once it changed.
	if cur != a.lyricAt {
		box := scroll.Bounds()
		target := float32(0)
		if active.Valid() {
			// Where the glide is going, without where it still is.
			if lb := active.LayoutBounds(); lb.H > 0 && box.H > 0 {
				target = lb.Y - box.Y + a.lyricScroll.Y - box.H*0.3
			} else {
				return // not laid out yet: the next frame scrolls
			}
		}
		a.lyricAt = cur
		a.lyricGlide.to(c, &a.lyricScroll, min(max(0, target), a.lyricScroll.MaxY))
	}
}

// lyricGlide scrolls the lyrics to the line sung, as a critically damped
// spring, as ScrollTo's would. ScrollTo moves the offset in every frame,
// and MyGo builds the view again in the frame after each that moved it,
// and its spring creeps over the last pixel for half a second: with a line
// following another every second or two, most of a song. So the offset
// goes to the line at once, in one frame, and the lines are held back by
// what the glide has still to go, its lag, which settles to nothing within
// half a pixel.
type lyricGlide struct {
	on bool
	// The lag t after start: (a + b·t)·e^(−ωt), in DIPs.
	a, b  float64
	start time.Time
	// The offset it goes to, set in the next frame (pending), with the lag
	// that holds the lines back from it: the layout of the frame that
	// chose it takes the offset too, the lines built without the lag.
	target  float32
	pending bool
	set     float32 // the offset it set
}

// lyricOmega is the spring's ω: a line or two away, it comes within half a
// pixel in about 800 ms.
const lyricOmega = 10

// at returns the lag at now, and how fast it changes, in DIPs a second.
func (g *lyricGlide) at(now time.Time) (lag, v float64) {
	if !g.on {
		return 0, 0
	}
	t := now.Sub(g.start).Seconds()
	e := math.Exp(-lyricOmega * t)
	return (g.a + g.b*t) * e, (g.b - lyricOmega*(g.a+g.b*t)) * e
}

// to scrolls s to target from the next frame, gliding from where it
// shows, at the speed it went if it was on its way.
func (g *lyricGlide) to(c *ui.Context, s *ui.ScrollState, target float32) {
	now := c.Now()
	lag, v := g.at(now)
	shown := float64(s.Y) + lag
	g.target, g.pending = target, true
	g.on = !c.Preferences().ReduceMotion
	g.start, g.a = now, shown-float64(target)
	g.b = v + lyricOmega*g.a
	c.AnimationFrame()
}

// step returns the lag of the lines in this frame, and asks for the next
// while there is one. The listener scrolling stops the glide where it
// shows.
func (g *lyricGlide) step(c *ui.Context, s *ui.ScrollState) float32 {
	if g.pending {
		s.Y, g.set, g.pending = g.target, g.target, false
	}
	if !g.on {
		return 0
	}
	lag, v := g.at(c.Now())
	if s.Y != g.set {
		// The listener took over: the offset takes what showed.
		s.Y, g.on = max(0, min(s.Y+float32(lag), s.MaxY)), false
		return 0
	}
	if math.Abs(lag) < 0.5 && math.Abs(v) < 20 {
		g.on = false
		return 0
	}
	c.AnimationFrame()
	return float32(lag)
}

// queuePane lists the whole queue in play order, the track playing marked
// among the others: a click plays one there, and the list stays as it is.
// queueDrag is a row of the queue on its way to another place: where it
// was taken from, where it would land, and how far the pointer has
// carried it from where it stood.
type queueDrag struct {
	from, to int // rows in the play order; from is -1 when none is dragged
	off      ui.Point
	moving   bool // it has left its place
}

func (a *app) queuePane(c *ui.Context) {
	pl := a.pl
	order, at := pl.PlayOrder()
	// Opened, it shows the track playing at its top; after, it follows it
	// as little as keeps it in view.
	if at >= 0 && at != a.queueAt {
		if a.queueAt < 0 {
			a.queueList.ScrollTo(at, ui.Start)
		} else {
			a.queueList.ScrollIntoView(at)
		}
		a.queueAt = at
	}
	a.queueList.Key = func(row int) any { return order[row] }
	// A row may be carried elsewhere in the queue: pressed, it comes with
	// the pointer, the rows it passes making room, and lands where it
	// shows. A press let go where it began is only a click.
	const queueRow = 60 // a row and the gap under it
	d := &a.queueDrag
	ui.List(c.Key("queue"), &a.queueList, len(order), func(r int) {
		i := order[r]
		tr := &pl.Queue()[i]
		playing := r == at
		row := ui.ButtonBase(c.Key(i)).Height(58).PaddingX(10).Gap(14).Radius(12).AlignItems(ui.Center).FocusRing(false).Label(tr.Name).
			ContextMenu(a.songItems(tr, 0))
		switch {
		case playing:
			row.Background(white.Alpha(0.12))
		case row.Hovered():
			row.Background(white.Alpha(0.08))
		}
		if !tr.Playable() {
			row.Opacity(0.4)
		}
		if d.from == r {
			row.Draggable(&d.off, ui.DraggableOptions{Axis: ui.Vertical})
			if row.Dragging() {
				d.moving = true
			}
			if d.moving {
				// It is a pane of glass lifted off the list: shadowed, and
				// slightly apart from what it passes.
				d.to = min(max(d.from+int(math.Round(float64(d.off.Y/queueRow))), 0), len(order)-1)
				row.Shadow(0, 14, 32, -8, a.pal.cardShade).Background(white.Alpha(0.16))
			} else if !row.Pressed() && !row.Dragging() {
				// Let go where it was taken: no errand.
				d.from = -1
			}
			if d.moving && !row.Dragging() && !row.Pressed() {
				from, to := d.from, d.to
				d.from, d.moving = -1, false
				d.off = ui.Point{}
				pl.Move(from, to)
			}
		} else if d.moving {
			// Room for it: what it passed moves up, or down, one row.
			switch {
			case d.from < r && r <= d.to:
				row.AnimatePose(ui.Pose{Y: -queueRow}, ui.Snappy)
			case r >= d.to && r < d.from:
				row.AnimatePose(ui.Pose{Y: queueRow}, ui.Snappy)
			}
		}
		row.Children(func() {
			a.cover(c, tr.Album.PicURL, 96, 42, 7)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, tr.Name).FontSize(14).FontWeight(600).SingleLine()
				a.artistLinks(c, tr, white.Alpha(0.55), white).FontSize(12.5).SingleLine()
			})
			ui.Text(c, clock(tr.Length())).FontSize(12).FontFeatures("tnum").TextColor(white.Alpha(0.4))
		})
		if row.Pressed() && d.from < 0 {
			d.from, d.to, d.off, d.moving = r, r, ui.Point{}, false
		}
		if row.Clicked() && tr.Playable() && !playing {
			pl.Jump(i)
		}
	}).Grow(1).Padding(8, 0, 40, 0).Gap(2)
}
