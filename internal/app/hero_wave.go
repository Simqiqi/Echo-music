package app

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/player"
)

const (
	// heroWaveW is how much of the lead record the wave takes at its
	// right; the words keep clear of it.
	heroWaveW = 340
	heroWaveH = 148
	// The wave keeps fewer bars once it narrows past heroWaveMin, so none
	// of them becomes a sliver; below heroWaveBarsMin it stands down, so
	// that the words never lose more of their line than they can spare.
	heroWaveMin     = 240
	heroWaveBarsMin = 120
	heroWordsMin    = 180
	// Where the words begin, past the disc, how much air they keep from
	// the wave, and how far the wave stands from the card's right edge.
	heroWordsLeft = 206
	heroWaveGap   = 46
	heroWaveEdge  = 26
)

// heroWaveWidth is how much of the lead record the wave may take: all of
// heroWaveW where the card is wide enough, less where it is narrower, and
// none at all where the words would have to give up their room for it —
// the words come first, and the wave takes what is left.
func heroWaveWidth(cardW float32) float32 {
	left := cardW - heroWordsLeft - heroWordsMin - heroWaveGap - heroWaveEdge
	if left < heroWaveBarsMin {
		return 0
	}
	return min(heroWaveW, left)
}

// heroWaveRect is where the wave stands on the lead record, as wide as
// the card can spare for it.
func heroWaveRect(r ui.Rect, w float32) ui.Rect {
	return ui.Rect{X: r.X + r.W - w - heroWaveEdge, Y: r.Y + r.H - heroWaveH - 20, W: w, H: heroWaveH}
}

// drawHeroWave paints the sound of what plays on the lead record: bars of
// its spectrum standing along the foot.
func (a *app) drawHeroWave(c *ui.Context, p *ui.Painter, r ui.Rect, now time.Time) {
	width := heroWaveWidth(r.W)
	if width < heroWaveBarsMin {
		return
	}
	w := heroWaveRect(r, width)
	lv, live := a.pl.Levels()
	a.heroBars.step(now, lv, live)

	// The bars: a wave of the music, low ones tall, along the foot. A
	// narrower wave keeps fewer of them, so that none is a sliver.
	count := 26
	if w.W < heroWaveMin {
		count = max(8, int(w.W/13))
	}
	bars := ui.Rect{X: w.X, Y: w.Y + w.H - 46, W: w.W, H: 46}
	drawBars(p, bars, &a.heroBars, count, a.pal.accent.Alpha(0.5), 3)

	if live || a.heroBars.group(0, player.Bands) > 0.01 {
		p.AnimationFrame()
	}
}
