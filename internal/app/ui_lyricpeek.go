package app

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
)

// sungLine is the line being sung, the one after it, and how long until
// that one begins. It answers nothing when there is nothing to show — no
// track, lyrics still loading, an instrumental.
func (a *app) sungLine() (sung, next string, until time.Duration, ok bool) {
	pl := a.pl
	lines, loaded := pl.Lyrics()
	if !loaded || len(lines) == 0 || pl.Current() == nil {
		return "", "", 0, false
	}
	pos := pl.Position()
	cur := lyrics.Index(lines, pos)
	if pl.Playing() && cur+1 < len(lines) {
		until = lines[cur+1].At - pos + 20*time.Millisecond
	}
	if cur >= 0 {
		sung = strings.TrimSpace(lines[cur].Text)
	}
	// The line after is the next one with words in it: a blank line
	// between verses would leave the second row empty.
	for i := cur + 1; i < len(lines) && next == ""; i++ {
		next = strings.TrimSpace(lines[i].Text)
	}
	return sung, next, until, sung != "" || next != ""
}

// lyricPeek shows the line being sung and the one after it, in the room
// the player bar's buttons leave at its right: the words at a glance,
// without opening the lyrics. A click opens them.
//
// It takes no room of its own when there is nothing to show — no track,
// lyrics still loading, an instrumental — so the bar is what it was.
func (a *app) lyricPeek(c *ui.Context) {
	sung, next, until, ok := a.sungLine()
	if !ok {
		return
	}
	// Redraw as the next line begins, so the two keep the song's time.
	if until > 0 {
		c.After(until)
	}

	p := a.pal
	row := ui.ButtonBase(c.Key("lyric-peek")).Column().AlignItems(ui.Start).
		Padding(4, 10).Radius(9).Gap(2).FocusRing(false).Label("歌词速览").
		MaxWidth(380).Shrink(1).MinWidth(0)
	bg := ui.Transparent
	if row.Hovered() {
		bg = p.hover
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
	row.Children(func() {
		// What is being sung, a step stronger than what comes; before the
		// first line, only what comes shows, quiet as the rest.
		if sung != "" {
			ui.Text(c, sung).FontSize(11.5).FontWeight(600).TextColor(p.text).SingleLine()
		}
		if next != "" {
			ui.Text(c, next).FontSize(11).TextColor(p.muted).SingleLine()
		}
	})
	if row.Clicked() {
		a.nowPlaying, a.npPane = true, paneLyrics
	}
}
