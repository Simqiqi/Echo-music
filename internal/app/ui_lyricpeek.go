package app

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
)

// lyricPeek shows the line being sung and the one after it, at the foot of
// the sidebar over what plays: the words at a glance, without opening the
// lyrics. A click opens them.
//
// It takes no room of its own when there is nothing to show — no track,
// lyrics still loading, an instrumental — so the sidebar is what it was.
func (a *app) lyricPeek(c *ui.Context) {
	pl := a.pl
	lines, loaded := pl.Lyrics()
	if !loaded || len(lines) == 0 || pl.Current() == nil {
		return
	}
	pos := pl.Position()
	cur := lyrics.Index(lines, pos)
	// Redraw as the next line begins, so the two keep the song's time.
	if pl.Playing() && cur+1 < len(lines) {
		c.After(lines[cur+1].At - pos + 20*time.Millisecond)
	}

	sung := ""
	if cur >= 0 {
		sung = strings.TrimSpace(lines[cur].Text)
	}
	// The line after is the next one with words in it: a blank line
	// between verses would leave the second row empty.
	next := ""
	for i := cur + 1; i < len(lines) && next == ""; i++ {
		next = strings.TrimSpace(lines[i].Text)
	}
	if sung == "" && next == "" {
		return
	}

	p := a.pal
	row := ui.ButtonBase(c.Key("lyric-peek")).Column().AlignItems(ui.Start).
		Padding(8, 10).Radius(10).Gap(3).FocusRing(false).Label("歌词速览")
	bg := ui.Transparent
	if row.Hovered() {
		bg = p.hover
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
	row.Children(func() {
		// What is being sung, a step stronger than what comes; before the
		// first line, only what comes shows, quiet as the rest.
		if sung != "" {
			ui.Text(c, sung).FontSize(12.5).FontWeight(600).TextColor(p.text).SingleLine()
		}
		if next != "" {
			ui.Text(c, next).FontSize(11.5).TextColor(p.muted).SingleLine()
		}
	})
	if row.Clicked() {
		a.nowPlaying, a.npPane = true, paneLyrics
	}
}
