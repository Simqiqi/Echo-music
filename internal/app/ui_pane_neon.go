package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The neon pane hangs the song up as a sign in the dark: every character a
// tube of glass that catches with a stutter, burns white at its core and
// throws its colour onto the night around it. The line before hums above,
// still lit but low; the line to come waits below, glass not yet switched
// on. Where the ink pane is quiet paper, this one is a street at two in the
// morning.

const (
	neonStrike = 0.26 // seconds a tube takes to catch
	neonDim    = 0.11 // how much of a tube not yet lit shows
)

// neonPane shows the lyrics as a sign at night, over the backdrop dimmed to
// hold it: the tabs above, the foot's controls below, and between them the
// three lines that make the sign.
func (a *app) neonPane(c *ui.Context, tr *netease.Track, accent ui.Color, w float32) {
	lines, loaded := a.pl.Lyrics()
	pos := a.pl.Position()
	cur := lyrics.Index(lines, pos)
	if a.pl.Playing() {
		// A tube catches in a quarter of a second: the stage's own pacing —
		// a frame a moment — carries the catch and the hum after it.
		stageNextFrame(c, lines, cur, pos, 0)
	}
	f := min(62, max(34, w*0.040))
	first, second := neonTints(accent)

	// In the flow, grown, so the foot's controls stay at the foot: the sign
	// itself is absolute within, and covers just what is above them.
	ui.Box(c.Key("neon-page")).Grow(1).Children(func() {
		// The night the sign hangs in: the backdrop dimmed where the words
		// are, so a tube's colour has dark to burn against. It starts under
		// the tabs, which are drawn before this and would go under it.
		ui.Box(c.Key("neon-night")).Absolute().Top(topBar(c)+npTabsHeight+8).
			Left(0).Right(0).Bottom(0).Background(ui.RGBA(5, 5, 12, 0.62)).PassThrough()

		// The line just sung dies out where it stood, stuttering as a sign
		// does when it is switched, rather than fading like ink.
		if cur > 0 {
			if settle := float32((pos - lines[cur].At).Seconds()); settle < 0.5 {
				neonLeaving(c, lines[cur-1].Text, settle, f)
			}
		}

		ui.Column(c.Key("neon-sign")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
			Justify(ui.Center).AlignItems(ui.Center).Padding(0, w*0.09).PassThrough().Children(func() {
			if !loaded {
				return
			}
			if len(lines) == 0 {
				ui.Text(c, "纯音乐，请欣赏").FontSize(26).FontWeight(700).TextColor(first.Alpha(0.6))
				return
			}
			// The line before: lit still, but turned down, the sign next
			// door in the same street.
			if cur > 0 {
				lit := clamp01(float32((pos-lines[cur].At).Seconds()-0.34) / 0.4)
				ui.Row(c.Key("before")).Height(f * 0.86).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					neonSmall(c, lines[cur-1].Text, f*0.5, first, 0.36*lit, 0.20*lit)
				})
			} else {
				// Kept, so the big line does not jump from one line to the
				// next.
				ui.Row(c.Key("before-space")).Height(f * 0.86)
			}
			if cur >= 0 {
				a.neonLine(c, &lines[cur], pos, f, first, second)
			} else {
				ui.Row(c.Key("await")).Height(f * 1.5)
			}
			// The line to come: glass, not yet switched on.
			if cur+1 < len(lines) {
				ui.Row(c.Key("after")).Height(f * 0.86).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					neonSmall(c, lines[cur+1].Text, f*0.5, first, neonDim, 0)
				})
			} else {
				ui.Row(c.Key("after-space")).Height(f * 0.86)
			}
		})
	})
}

// neonLine is the line being sung: its words, each tube catching as it is
// sung, and the translation under it in smaller tubes.
func (a *app) neonLine(c *ui.Context, l *lyrics.Line, pos time.Duration, f float32, first, second ui.Color) {
	ui.Column(c.Key(l.At)).AlignItems(ui.Center).Gap(f * 0.30).Children(func() {
		if len(l.Words) == 0 {
			// No word times to catch to: the line lit whole, as it is sung.
			since := float32((pos - l.At).Seconds())
			lit := clamp01(since / neonStrike)
			if since <= 0 {
				lit = 0
			}
			ui.Row(c.Key("plain")).MinHeight(f * 1.3).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				neonSmall(c, l.Text, f, first, neonDim+(1-neonDim)*lit, 0.30*lit)
			})
			return
		}
		ui.Row(c.Key("tubes")).MinHeight(f * 1.3).Justify(ui.Center).AlignItems(ui.Center).
			Wrap().Children(func() {
			n := len(l.Words)
			for i, word := range l.Words {
				tint := first
				if n > 1 {
					// One sign, two colours: the tubes run from the cover's
					// own hue to one a step round the wheel.
					tint = first.Mix(second, float32(i)/float32(n-1))
				}
				neonTube(c.Key(i), i, word, pos, f, tint)
			}
		})
		if l.Translation != "" {
			ui.Text(c.Key("note"), l.Translation).FontSize(max(14, f*0.30)).FontWeight(600).
				TextColor(white.Alpha(0.40)).TextAlign(ui.Center)
		}
	})
}

// neonTube draws one word as it is at pos: dark glass before it is sung,
// white hot with its colour around it as it catches, and humming after.
func neonTube(c *ui.Context, i int, word lyrics.Word, pos time.Duration, f float32, tint ui.Color) {
	since := float32((pos - word.Start).Seconds())
	lit, level := float32(0), float32(1)
	if since > 0 {
		lit = clamp01(since / neonStrike)
		if since < 0.55 {
			// Catching: it overshoots white and stutters, as a starter does.
			q := since / 0.55
			level = 1 + 0.6*(1-q)*(0.55+0.45*float32(math.Sin(float64(since*90))))
		} else {
			// Lit, it hums: never quite steady.
			level = 1 - 0.035*float32(math.Abs(math.Sin(float64(pos.Seconds()*7.7)+float64(i)*2.3)))
		}
	}

	box := ui.Box(c).MarginEnd(f * 0.14).Scale(lerp(1.04, 1, lit), lerp(1.04, 1, lit))
	box.Children(func() {
		if lit > 0.02 {
			// The colour thrown onto the night, then the rim of the tube.
			ui.Text(c.Key("halo"), word.Text).Absolute().Top(0).Left(0).FontSize(f).FontWeight(700).NoWrap().
				TextColor(tint.Alpha(min(1, 0.40*lit*level))).Blur(f * 0.30).PassThrough()
			ui.Text(c.Key("rim"), word.Text).Absolute().Top(0).Left(0).FontSize(f).FontWeight(700).NoWrap().
				TextColor(tint.Alpha(min(1, 0.60*lit))).Blur(f * 0.07).PassThrough()
		}
		coreAlpha := neonDim + (1-neonDim)*lit
		if level > 1 {
			// White hot at the strike, settling to the colour it holds.
			coreAlpha = min(1, 0.45+0.55*lit)
		}
		ui.Text(c.Key("core"), word.Text).FontSize(f).FontWeight(700).NoWrap().TextColor(white.Alpha(coreAlpha))
	})
}

// neonSmall is a whole line as one tube: glass at alpha, with a halo at
// halo of the sign's own colour. Used for the lines either side of the one
// being sung, where no word has to catch.
func neonSmall(c *ui.Context, text string, size float32, tint ui.Color, alpha, halo float32) {
	box := ui.Box(c)
	box.Children(func() {
		if halo > 0.01 {
			ui.Text(c.Key("halo"), text).Absolute().Top(0).Left(0).FontSize(size).FontWeight(700).NoWrap().
				TextColor(tint.Alpha(min(1, halo))).Blur(size * 0.28).PassThrough()
		}
		ui.Text(c.Key("glass"), text).FontSize(size).FontWeight(700).NoWrap().TextColor(white.Alpha(alpha))
	})
}

// neonLeaving is the line just sung, dying where it stood: switched off in
// stutters rather than faded, and drawing up into the small line above.
func neonLeaving(c *ui.Context, text string, settle, f float32) {
	q := clamp01(settle / 0.5)
	stutter := 1 - 0.55*float32(math.Abs(math.Sin(float64(settle*46))))
	s := lerp(1, 0.56, easeOutCubic(q))
	ui.Row(c.Key("neon-leaving")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		Justify(ui.Center).AlignItems(ui.Center).PassThrough().Children(func() {
		ui.Text(c, text).FontSize(f).FontWeight(700).SingleLine().
			TextColor(white.Alpha(0.88 * (1 - q) * stutter)).
			Blur(f*0.04 + f*0.18*q).
			Scale(s, s).
			Translate(0, -f*(0.2+0.8*q))
	})
}

// neonTints are the two colours a sign is made of: the cover's own hue with
// enough colour in it to burn, and one a step round the wheel. A cover with
// no colour to speak of still lights its sign — in the blue of a street at
// night.
func neonTints(accent ui.Color) (ui.Color, ui.Color) {
	h, s, v := hsv(float64(accent.R)/255, float64(accent.G)/255, float64(accent.B)/255)
	if s < 0.22 || v < 0.20 {
		h, s = 214, 0.72
	}
	first := hsvColor(h, max(0.66, min(0.95, s)), 1)
	second := hsvColor(float64(wrapHue(float32(h)+18)), max(0.58, min(0.92, s*0.9)), 1)
	return first, second
}
