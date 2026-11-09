package app

import (
	"bytes"
	"image/png"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The size of a shared lyric card, in points: three by four, drawn at two
// pixels a point, which is the shape a phone screen shows it at.
const (
	cardW = 540
	cardH = 720
)

// lyricPick is one line the listener chose for a card, with its
// translation if it has one.
type lyricPick struct {
	text  string
	trans string
}

// lyricCard is what a shared card says: the line being sung, the ones
// around it, and what plays. picked, when set, replaces the automatic
// quoting with the lines the listener chose, in the order they sing them.
type lyricCard struct {
	title  string
	artist string
	before string
	line   string
	after  string
	trans  string // the line's translation, if it has one
	color  ui.Color
	cover  *ui.Bitmap
	picked []lyricPick
}

// lyricCardOf gathers what a card would say of what plays: the line being
// sung, or the first one to come while the song is still in its intro. It
// reports false when there is nothing to quote.
func (a *app) lyricCardOf(tr *netease.Track) (lyricCard, bool) {
	lines, loaded := a.pl.Lyrics()
	if !loaded || len(lines) == 0 {
		return lyricCard{}, false
	}
	names := make([]string, 0, len(tr.Artists))
	for _, ar := range tr.Artists {
		names = append(names, ar.Name)
	}
	card := lyricCard{
		title:  tr.Name,
		artist: strings.Join(names, " / "),
		color:  ui.RGB(120, 120, 130),
	}
	// The song's color, as the player bar and the backdrop take it, and
	// the cover to sit beside what plays.
	if art := a.art.get(netease.Sized(tr.Album.PicURL, 600), false); art != nil && !art.failed {
		card.color = art.tint
		card.cover = art.bmp
	}
	// Lines the listener picked win over the one being sung: the card
	// quotes them in the order they are sung.
	if picked := a.pickedLyricLines(); len(picked) > 0 {
		card.picked = picked
		card.line, card.trans = picked[0].text, picked[0].trans
		return card, true
	}
	at := a.lyricLineAt(lines, a.pl.Position())
	if at < 0 {
		return lyricCard{}, false
	}
	card.line = strings.TrimSpace(lines[at].Text)
	card.trans = strings.TrimSpace(lines[at].Translation)
	for i := at - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i].Text); t != "" {
			card.before = t
			break
		}
	}
	for i := at + 1; i < len(lines); i++ {
		if t := strings.TrimSpace(lines[i].Text); t != "" {
			card.after = t
			break
		}
	}
	return card, true
}

// lyricLineAt is the line a card quotes at pos: the one being sung, or
// the nearest with words in it. An intro has its first line some way off
// yet, and a blank line between verses quotes nothing.
func (a *app) lyricLineAt(lines []lyrics.Line, pos time.Duration) int {
	cur := lyrics.Index(lines, pos)
	at := cur
	if at < 0 || at >= len(lines) || strings.TrimSpace(lines[at].Text) == "" {
		at = -1
		for i := max(0, cur); i < len(lines) && at < 0; i++ {
			if strings.TrimSpace(lines[i].Text) != "" {
				at = i
			}
		}
		for i := min(len(lines)-1, cur); i >= 0 && at < 0; i-- {
			if strings.TrimSpace(lines[i].Text) != "" {
				at = i
			}
		}
	}
	return at
}

// pickedLyricLines returns every line the listener chose, in the order
// they are sung. There is no cap: a card grows to hold them all.
func (a *app) pickedLyricLines() []lyricPick {
	var out []lyricPick
	for i, on := range a.shareSel {
		if !on || i >= len(a.shareLines) {
			continue
		}
		line := a.shareLines[i]
		if strings.TrimSpace(line.text) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// pickedCardLayout is how many points the picked lines are set in, and how
// tall the card has to be to hold every one of them. The type gives way as
// lines are added, down to a floor it stays readable at; the card keeps
// its 3:4 shape until the words need more room than that, then grows.
func pickedCardLayout(card lyricCard) (size float32, height int) {
	n := len(card.picked)
	size = 30
	switch {
	case n >= 12:
		size = 16
	case n >= 9:
		size = 18
	case n >= 7:
		size = 20
	case n >= 5:
		size = 22
	case n == 4:
		size = 24
	case n == 3:
		size = 26
	case n == 2:
		size = 28
	}
	// The quote mark and padding above the words, and the cover, the
	// brand and the padding below them: everything but the lines.
	const head, foot = float32(112), float32(150)
	// A line may wrap once; leave room for it, and for the gap between.
	per := size*1.36*1.5 + 16
	need := head + foot + per*float32(n) + 24
	for _, p := range card.picked {
		if p.trans != "" {
			need += size*0.52*1.5 + 8
		}
	}
	return size, int(max(cardH, need))
}

// shareSelCount is how many lines are picked.
func (a *app) shareSelCount() int {
	n := 0
	for _, on := range a.shareSel {
		if on {
			n++
		}
	}
	return n
}

// shareLyrics copies a card of the lyrics, with the cover and the song's
// color, so it can be pasted into a chat or a post.
func (a *app) shareLyrics() {
	tr := a.pl.Current()
	if tr == nil {
		return
	}
	card, ok := a.lyricCardOf(tr)
	if !ok {
		a.toast = "还没有歌词，分享不了"
		a.redraw()
		return
	}
	if card.cover != nil || tr.Album.PicURL == "" {
		a.writeLyricCard(card)
		return
	}
	// The cover is not in the cache: fetch it, then draw on the main
	// thread, where the renderer lives.
	url := netease.Sized(tr.Album.PicURL, 600)
	go func() {
		b, err := netease.Fetch(url)
		mygo.RunOnMain(func() {
			if err == nil {
				if bmp, derr := ui.DecodeBitmap(b); derr == nil {
					card.cover = bmp
				}
			}
			a.writeLyricCard(card)
		})
	}()
}

// writeLyricCard draws the card, and puts it on the clipboard. A card of
// lines the listener picked grows past the shared size to hold them all.
func (a *app) writeLyricCard(card lyricCard) {
	h := cardH
	if len(card.picked) > 0 {
		_, h = pickedCardLayout(card)
	}
	a.writeCardSize(func(c *ui.Context) { a.shareCard(c, card) },
		cardW, h, "歌词卡片已复制，粘贴即可分享")
}

// writeCard renders a card of the shared size and copies its PNG, saying
// what happened. The listening poster goes this way.
func (a *app) writeCard(draw func(*ui.Context), done string) {
	a.writeCardSize(draw, cardW, cardH, done)
}

// writeCardSize renders a card of w×h and copies its PNG.
func (a *app) writeCardSize(draw func(*ui.Context), w, h int, done string) {
	img := ui.Render(draw, w, h, 2)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil || a.putImage(buf.Bytes()) != nil {
		a.toast = "没能生成图片"
		a.redraw()
		return
	}
	a.toast = done
	a.redraw()
}

// putImage hands a card's image to the clipboard, or to a test in its
// place.
func (a *app) putImage(data []byte) error {
	if a.saveImage != nil {
		return a.saveImage(data)
	}
	return mygo.Clipboard.WriteImage(data)
}

// cardBackdrop carries a cover across the whole card and lets it fade
// into the dark where the words sit: the photograph comes through the
// middle and along one edge, with the song's own color gathered over it.
// Without a cover, the color alone makes the same ground as before.
func cardBackdrop(c *ui.Context, cover *ui.Bitmap, ink ui.Color) {
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		Background(ui.Hex("#0c0c0f"))
	if cover != nil {
		ui.Image(c, cover).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover)
	}
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
		// The cover's color, and then the dark the white words need:
		// heaviest at the head and foot, thinner through the middle.
		p.FillGradient(r, ui.LinearGradient{Angle: 150, Oklab: true, Stops: []ui.GradientStop{
			{Color: ink.Alpha(0.5), At: 0},
			{Color: ink.Alpha(0.18), At: 0.42},
			{Color: ink.Alpha(0), At: 0.82},
		}}, 0)
		p.FillGradient(r, ui.LinearGradient{Angle: 90, Oklab: true, Stops: []ui.GradientStop{
			{Color: ui.RGBA(8, 8, 12, 0.82), At: 0},
			{Color: ui.RGBA(8, 8, 12, 0.3), At: 0.36},
			{Color: ui.RGBA(8, 8, 12, 0.42), At: 0.66},
			{Color: ui.RGBA(8, 8, 12, 0.92), At: 1},
		}}, 0)
	})
}

// shareCard draws a lyric card: the cover held behind the words, the line
// being sung in the middle, and what plays at the foot.
func (a *app) shareCard(c *ui.Context, card lyricCard) {
	ink := card.color
	h := cardH
	pickSize := float32(30)
	if len(card.picked) > 0 {
		pickSize, h = pickedCardLayout(card)
	}
	ui.Column(c).Size(cardW, float32(h)).Clip().Children(func() {
		cardBackdrop(c, card.cover, ink)
		// Absolute too: the backdrop is a stack of absolute layers, and a
		// flowing column would be painted beneath them.
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).
			Padding(44, 46, 40, 46).Gap(0).Children(func() {
			ui.Text(c, "“").FontSize(74).FontWeight(800).LineHeight(0.82).
				TextColor(ink.Mix(white, 0.5))
			if len(card.picked) > 0 {
				// The listener's own lines: all of them read as the quoted
				// words, their size giving way and the card growing as more
				// of them are picked.
				for i, p := range card.picked {
					margin := float32(6)
					if i > 0 {
						margin = 16
					}
					ui.Text(c, p.text).FontSize(pickSize).FontWeight(800).LineHeight(1.36).
						LetterSpacing(-0.4).MaxLines(2).TextColor(white).Margin(margin, 0, 0, 0)
					if p.trans != "" {
						ui.Text(c, p.trans).FontSize(pickSize*0.52).LineHeight(1.5).MaxLines(2).
							TextColor(white.Alpha(0.6)).Margin(8, 0, 0, 0)
					}
				}
			} else {
				if card.before != "" {
					ui.Text(c, card.before).FontSize(16).LineHeight(1.5).MaxLines(1).
						TextColor(white.Alpha(0.34)).Margin(16, 0, 0, 0)
				}
				ui.Text(c, card.line).FontSize(31).FontWeight(800).LineHeight(1.36).
					LetterSpacing(-0.4).MaxLines(4).TextColor(white).Margin(6, 0, 0, 0)
				if card.trans != "" {
					ui.Text(c, card.trans).FontSize(15.5).LineHeight(1.5).MaxLines(2).
						TextColor(white.Alpha(0.6)).Margin(12, 0, 0, 0)
				}
				if card.after != "" {
					ui.Text(c, card.after).FontSize(16).LineHeight(1.5).MaxLines(1).
						TextColor(white.Alpha(0.34)).Margin(12, 0, 0, 0)
				}
			}

			ui.Box(c).Grow(1)

			// What plays, as the player bar would say it.
			ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
				cover := ui.Box(c).Size(62, 62).Radius(12).Clip().Center().Background(white.Alpha(0.1))
				cover.Children(func() {
					if card.cover != nil {
						ui.Image(c, card.cover).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover)
					} else {
						ui.Icon(c, iconNote).Size(22, 22).TextColor(white.Alpha(0.45))
					}
				})
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, card.title).FontSize(15.5).FontWeight(600).SingleLine().TextColor(white)
					ui.Text(c, card.artist).FontSize(12.5).SingleLine().TextColor(white.Alpha(0.6))
				})
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(26, 0, 0, 0).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(ink.Mix(white, 0.3))
				ui.Text(c, "Echo云音乐").FontSize(11.5).LetterSpacing(0.2).TextColor(white.Alpha(0.45))
			})
		})
	})
}

// shareLyricsButton is the button over the lyrics that opens the card's
// line picker, at the pane's top right, over the words rather than in the
// flow.
func (a *app) shareLyricsButton(c *ui.Context) ui.Element {
	b := ui.ButtonBase(c.Key("share-lyrics")).Absolute().Top(0).Right(-4).Size(30, 30).Center().
		Radius(999).Label("分享歌词").FocusRing(false)
	bg, fg := ui.Transparent, white.Alpha(0.42)
	if b.Hovered() {
		bg, fg = white.Alpha(0.1), white
	}
	b.TextColor(fg)
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 140 * time.Millisecond}))
	b.Children(func() { ui.Icon(c, iconShare).Size(15, 15) })
	if b.Clicked() {
		a.openSharePicker()
	}
	return b
}

// openSharePicker opens the picker over the playing view, with the line
// being sung already chosen so that confirming at once does what the
// button used to do by itself.
func (a *app) openSharePicker() {
	lines, loaded := a.pl.Lyrics()
	if !loaded || len(lines) == 0 {
		a.toast = "还没有歌词，分享不了"
		a.redraw()
		return
	}
	if tr := a.pl.Current(); tr != nil {
		a.sharePickTrack = tr.ID
	}
	a.shareLines = make([]lyricPick, len(lines))
	for i, l := range lines {
		a.shareLines[i] = lyricPick{
			text:  strings.TrimSpace(l.Text),
			trans: strings.TrimSpace(l.Translation),
		}
	}
	a.shareSel = make([]bool, len(lines))
	if at := a.lyricLineAt(lines, a.pl.Position()); at >= 0 && at < len(a.shareSel) {
		a.shareSel[at] = true
	}
	a.shareScroll = ui.ScrollState{}
	a.sharePickOpen = true
	a.redraw()
}

// closeSharePicker puts the picker away and forgets its picks, so the
// next card starts from the line being sung again.
func (a *app) closeSharePicker() {
	a.sharePickOpen = false
	a.shareSel, a.shareLines = nil, nil
}

// sharePicker is the card's own choosing: every line of the song, the
// picked ones marked, and a button that copies a card of them.
func (a *app) sharePicker(c *ui.Context) {
	if !a.sharePickOpen {
		return
	}
	// The lines are a snapshot: another song playing under the picker
	// would make their indices mean something else.
	if tr := a.pl.Current(); tr == nil || tr.ID != a.sharePickTrack {
		a.closeSharePicker()
		return
	}
	p := a.pal
	ui.DialogBase(c.Key("share-picker"), &a.sharePickOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.42))
		panel.Width(460).Height(560).MaxHeight(760).Radius(22).Gap(0).Clip().
			Background(p.bg).Border(1, p.hairline).
			Shadow(0, 30, 70, -14, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.96, Y: 10}, Exit: &ui.Motion{Scale: 0.98, Opacity: 0}})

		ui.Row(c).Padding(16, 18, 12, 18).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, "选择歌词").FontSize(15.5).FontWeight(700)
				ui.Text(c, "可选任意多行，按歌曲顺序排进卡片；行多时卡片自动变长").
					FontSize(12).TextColor(p.muted)
			})
			if iconButton(c.Key("share-close"), iconX, 12, p.muted, "关闭").Clicked() {
				a.closeSharePicker()
			}
		})
		ui.Box(c).Height(1).Background(p.hairline)

		ui.Scroll(c).Grow(1).Shrink(1).TrackScroll(&a.shareScroll).
			Padding(8, 12).Gap(2).Children(func() {
			for i, line := range a.shareLines {
				if line.text == "" {
					continue
				}
				at := i
				on := a.shareSel[at]
				row := ui.ButtonBase(c.Key(at)).Padding(9, 12).Radius(9).Gap(10).
					AlignItems(ui.Center).FocusRing(false).Label(line.text)
				fill := ui.Transparent
				switch {
				case on:
					fill = p.accent.Alpha(0.14)
				case row.Hovered():
					fill = p.hover
				}
				row.Background(fill)
				row.Children(func() {
					box := ui.Box(c).Size(18, 18).Radius(5).Center().Border(1, p.hairline)
					if on {
						box.Background(p.accent).Border(1, p.accent)
					}
					box.Children(func() {
						if on {
							ui.Icon(c, iconCheck).Size(12, 12).TextColor(p.onAccent)
						}
					})
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, line.text).FontSize(13.5).MaxLines(2).LineHeight(1.4).TextColor(p.text)
						if line.trans != "" {
							ui.Text(c, line.trans).FontSize(11.5).SingleLine().TextColor(p.muted)
						}
					})
				})
				if row.Clicked() {
					a.shareSel[at] = !on
				}
			}
		})
		ui.Box(c).Height(1).Background(p.hairline)

		ui.Row(c).Padding(12, 18).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "已选 %d 行", a.shareSelCount()).FontSize(12).TextColor(p.muted).Grow(1)
			if rowButton(c, "share-cancel", "取消", p).Clicked() {
				a.closeSharePicker()
			}
			if ui.PrimaryButton(c.Key("share-copy"), "生成卡片").Clicked() {
				if a.shareSelCount() == 0 {
					a.toast = "先选一句歌词"
					return
				}
				a.sharePickOpen = false
				a.shareLyrics()
				a.shareSel, a.shareLines = nil, nil
			}
		})
	})
}
