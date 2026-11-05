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

// lyricCard is what a shared card says: the line being sung, the ones
// around it, and what plays.
type lyricCard struct {
	title  string
	artist string
	before string
	line   string
	after  string
	trans  string // the line's translation, if it has one
	color  ui.Color
	cover  *ui.Bitmap
}

// lyricCardOf gathers what a card would say of what plays: the line being
// sung, or the first one to come while the song is still in its intro. It
// reports false when there is nothing to quote.
func (a *app) lyricCardOf(tr *netease.Track) (lyricCard, bool) {
	lines, loaded := a.pl.Lyrics()
	if !loaded || len(lines) == 0 {
		return lyricCard{}, false
	}
	pos := a.pl.Position()
	cur := lyrics.Index(lines, pos)
	at := cur
	// The line sung, or the nearest one with words in it: the intro of a
	// song has its first line some way off yet, and a blank line between
	// verses quotes nothing.
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
		if at < 0 {
			return lyricCard{}, false
		}
	}

	names := make([]string, 0, len(tr.Artists))
	for _, ar := range tr.Artists {
		names = append(names, ar.Name)
	}
	card := lyricCard{
		title:  tr.Name,
		artist: strings.Join(names, " / "),
		line:   strings.TrimSpace(lines[at].Text),
		trans:  strings.TrimSpace(lines[at].Translation),
		color:  ui.RGB(120, 120, 130),
	}
	// The song's color, as the player bar and the backdrop take it, and
	// the cover to sit beside what plays.
	if art := a.art.get(netease.Sized(tr.Album.PicURL, 600), false); art != nil && !art.failed {
		card.color = art.tint
		card.cover = art.bmp
	}
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

// writeLyricCard draws the card, and puts it on the clipboard.
func (a *app) writeLyricCard(card lyricCard) {
	a.writeCard(func(c *ui.Context) { a.shareCard(c, card) }, "歌词卡片已复制，粘贴即可分享")
}

// writeCard renders a card of the shared size and copies its PNG, saying
// what happened. The lyric card and the listening poster both go this way.
func (a *app) writeCard(draw func(*ui.Context), done string) {
	img := ui.Render(draw, cardW, cardH, 2)
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
	ui.Column(c).Size(cardW, cardH).Clip().Children(func() {
		cardBackdrop(c, card.cover, ink)
		// Absolute too: the backdrop is a stack of absolute layers, and a
		// flowing column would be painted beneath them.
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).
			Padding(44, 46, 40, 46).Gap(0).Children(func() {
			ui.Text(c, "“").FontSize(74).FontWeight(800).LineHeight(0.82).
				TextColor(ink.Mix(white, 0.5))
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

// shareLyricsButton is the button over the lyrics that copies a card of
// them, at the pane's top right, over the words rather than in the flow.
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
		a.shareLyrics()
	}
	return b
}
