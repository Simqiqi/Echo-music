package app

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The ink pane writes the song out a character at a time: each one lands
// as a wet stroke that spreads and dries, the line before it left as a
// stain above, the line to come only pencilled in below. Where the stage
// bounces to the beat, this page is still, and reads like a page being
// written — so it wants a background and nothing else.

const (
	inkSet   = 0.30 // seconds a stroke takes to settle: its blur gone
	inkDry   = 0.70 // seconds its wet halo takes to dry
	inkGhost = 0.10 // how much of a character not yet sung shows
)

// inkPane shows the lyrics as ink on the page the backdrop makes. It takes
// the whole view: the tabs above, the foot's controls below, and between
// them the three lines that make a page — the one before, the one being
// written, and the one to come.
func (a *app) inkPane(c *ui.Context, tr *netease.Track, accent ui.Color, w float32) {
	lines, loaded := a.pl.Lyrics()
	pos := a.pl.Position()
	cur := lyrics.Index(lines, pos)
	if a.pl.Playing() {
		// The strokes set in a third of a second and dry in under one, so
		// the stage's own pacing — a frame a moment — carries them.
		stageNextFrame(c, lines, cur, pos, 0)
	}
	f := min(52, max(30, w*0.036))

	// In the flow, grown, so the foot's controls stay at the foot: the
	// page itself is absolute within, and covers just what is above them.
	ui.Box(c.Key("ink-page")).Grow(1).Children(func() {
		// The line just left: its ink still wet, it lifts off the page —
		// shrinking as it rises, into the small dried line it becomes —
		// while the new line is written under it.
		if cur > 0 {
			if settle := float32((pos - lines[cur].At).Seconds()); settle < 0.55 {
				q := easeOutCubic(clamp01(settle / 0.55))
				ui.Row(c.Key("leaving")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
					Justify(ui.Center).AlignItems(ui.Center).PassThrough().Children(func() {
					s := lerp(1, 0.56, q)
					ui.Text(c, lines[cur-1].Text).FontSize(f).FontWeight(800).SingleLine().
						TextColor(white.Alpha(0.9*(1-q))).Blur(max(0.3, f*0.06*q)).
						Scale(s, s).Translate(0, -f*(0.35+1.05*q))
				})
			}
		}
		ui.Column(c.Key("page")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
			Justify(ui.Center).AlignItems(ui.Center).Padding(0, w*0.10).PassThrough().Children(func() {
			if !loaded {
				return
			}
			if len(lines) == 0 {
				ui.Text(c, "纯音乐，请欣赏").FontSize(26).FontWeight(700).TextColor(white.Alpha(0.35))
				return
			}
			// The line before, dried: it appears once the leaving line has
			// faded, and keeps its place above the writing.
			if cur > 0 {
				settle := float32((pos - lines[cur].At).Seconds())
				alpha := 0.24 * clamp01((settle-0.55)/0.3)
				ui.Row(c.Key("before")).Height(f * 0.9).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					ui.Text(c, lines[cur-1].Text).FontSize(f * 0.56).FontWeight(700).
						TextColor(white.Alpha(alpha)).SingleLine().Blur(f * 0.012)
				})
			} else {
				// Kept, so that the writing line does not jump up and down
				// from one line to the next.
				ui.Row(c.Key("before-space")).Height(f * 0.9)
			}
			if cur >= 0 {
				a.inkLine(c, &lines[cur], pos, f, accent)
			} else {
				ui.Row(c.Key("await")).Height(f * 1.6)
			}
			if cur+1 < len(lines) {
				ui.Row(c.Key("after")).Height(f * 0.9).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					ui.Text(c, lines[cur+1].Text).FontSize(f * 0.56).FontWeight(700).
						TextColor(white.Alpha(0.13)).SingleLine()
				})
			} else {
				ui.Row(c.Key("after-space")).Height(f * 0.9)
			}
		})
	})
}

// inkLine is the line being written: its words, each landing as it is
// sung, and the translation under it as a note in the margin.
func (a *app) inkLine(c *ui.Context, l *lyrics.Line, pos time.Duration, f float32, accent ui.Color) {
	ui.Column(c.Key(l.At)).AlignItems(ui.Center).Gap(f * 0.28).Children(func() {
		if len(l.Words) == 0 {
			// No word times to write to: the line as a whole, set at once.
			ui.Row(c.Key("plain")).MinHeight(f * 1.4).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				ui.Text(c, l.Text).FontSize(f).FontWeight(800).TextColor(white)
			})
			return
		}
		ui.Row(c.Key("strokes")).MinHeight(f * 1.4).Justify(ui.Center).AlignItems(ui.Center).
			Wrap().Children(func() {
			for i, word := range l.Words {
				inkStroke(c.Key(i), word, pos, f, accent)
			}
		})
		if l.Translation != "" {
			ui.Text(c.Key("note"), l.Translation).FontSize(max(14, f*0.32)).FontWeight(600).
				TextColor(white.Alpha(0.42)).TextAlign(ui.Center)
		}
	})
}

// inkStroke draws one word as it is at pos: pencilled in before it is
// sung, landing as a wet stroke when it is, and dry after.
func inkStroke(c *ui.Context, word lyrics.Word, pos time.Duration, f float32, accent ui.Color) {
	end := max(word.End, word.Start+150*time.Millisecond)
	since := float32((pos - word.Start).Seconds())
	e := easeOutCubic(clamp01(since / inkSet))

	// Before it is sung: a faint pencil guide, so the line's shape is
	// there to be written into.
	opacity, blur := float32(inkGhost), f*0.05
	if since > 0 {
		opacity = inkGhost + (1-inkGhost)*clamp01(since/0.16)
		blur = f * 0.20 * (1 - e) // the wet stroke spreading, then sharp
	}
	scale := lerp(1.09, 1, e)
	wet := clamp01(1 - since/inkDry)

	// The stroke just made glows a moment, as ink does while it is wet.
	glow := float32(0)
	if since > 0 && pos < end+700*time.Millisecond {
		glow = clamp01(1 - float32((pos-end).Seconds())/0.7)
	}

	box := ui.Box(c).MarginEnd(f*0.09).Scale(scale, scale).Opacity(clamp01(opacity))
	box.Children(func() {
		if since > 0 && wet > 0.01 {
			// The ink soaking into the paper around the stroke.
			ui.Text(c.Key("wet"), word.Text).Absolute().Top(0).Left(0).FontSize(f).FontWeight(800).NoWrap().
				TextColor(accent.Alpha(0.30 * wet)).Blur(f * 0.14).PassThrough()
		}
		if glow > 0.01 {
			ui.Text(c.Key("glow"), word.Text).Absolute().Top(0).Left(0).FontSize(f).FontWeight(800).NoWrap().
				TextColor(white.Alpha(0.32 * glow)).Blur(f * 0.18).PassThrough()
		}
		txt := ui.Text(c.Key("ink"), word.Text).FontSize(f).FontWeight(800).NoWrap().TextColor(white)
		if blur > 0.3 {
			txt.Blur(blur)
		}
	})
}
