package app

import (
	"bytes"
	"image/png"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The accent the settings pick is the palette's accent; nothing chosen, or
// something unknown, is the red the app has always been.
func TestAccentPaletteFollowsChoice(t *testing.T) {
	red := paletteFor(&ui.Theme{}, "red")
	blue := paletteFor(&ui.Theme{}, "blue")
	if red.accent == blue.accent {
		t.Fatal("the blue accent is the red one")
	}
	if def := paletteFor(&ui.Theme{}, ""); def.accent != red.accent {
		t.Fatal("no accent chosen is not the red one")
	}
	if odd := paletteFor(&ui.Theme{}, "chartreuse"); odd.accent != red.accent {
		t.Fatal("an unknown accent is not the red one")
	}
	if ui.Oklch(0.61, 0.2, accentHue("blue")) != blue.accent {
		t.Fatal("the palette does not take the accent's hue")
	}
}

// The settings' row of swatches chooses the accent.
func TestAccentSwatches(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	// No app runs to apply an appearance to.
	old := setThemeSource
	setThemeSource = func(mygo.ThemeSource) {}
	defer func() { setThemeSource = old }()

	tt := ui.NewTester(a.settingsTestView, 520, 940)
	tt.Frame()
	if err := tt.Click("强调色蓝"); err != nil {
		t.Fatal(err)
	}
	if a.settings.Accent != "blue" {
		t.Fatal("the blue swatch did not choose blue:", a.settings.Accent)
	}
	if err := tt.Click("强调色红"); err != nil {
		t.Fatal(err)
	}
	if a.settings.Accent != "red" {
		t.Fatal("the red swatch did not choose red:", a.settings.Accent)
	}
}

// A card quotes the line being sung, the ones around it, and skips a blank
// line; while the song is still in its intro, it quotes the first to come.
func TestLyricCardQuoting(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tr := netease.Track{ID: 9, Name: "晴天", Artists: []netease.Artist{{Name: "周杰伦"}}, Duration: 120000}
	a.pl.Cue([]netease.Track{tr})
	a.pl.SetLyrics([]lyrics.Line{
		{At: 0, End: 2 * time.Second, Text: "作词：周杰伦"},
		{At: 20 * time.Second, End: 25 * time.Second, Text: "故事的小黄花"},
		{At: 25 * time.Second, End: 30 * time.Second, Text: "  "},
		{At: 30 * time.Second, End: 35 * time.Second, Text: "从出生那年就飘着", Translation: "story"},
	})

	a.pl.Hold(22 * time.Second)
	card, ok := a.lyricCardOf(&tr)
	if !ok || card.line != "故事的小黄花" || card.before != "作词：周杰伦" || card.after != "从出生那年就飘着" {
		t.Fatal("the card quotes the wrong lines:", card)
	}
	if card.title != "晴天" || card.artist != "周杰伦" {
		t.Fatal("the card says the wrong song:", card.title, card.artist)
	}

	a.pl.Hold(5 * time.Second)
	if card, ok = a.lyricCardOf(&tr); !ok || card.line != "作词：周杰伦" {
		t.Fatal("the intro does not quote the first line:", card)
	}

	a.pl.Hold(26 * time.Second)
	if card, ok = a.lyricCardOf(&tr); !ok || card.line != "从出生那年就飘着" || card.trans != "story" {
		t.Fatal("a blank line did not skip ahead:", card)
	}

	// No lyrics, no card.
	a.pl.SetLyrics(nil)
	if _, ok = a.lyricCardOf(&tr); ok {
		t.Fatal("a song without lyrics quotes something")
	}
}

// Sharing writes the card's image where it goes, as a PNG of the card's
// size, and says so.
func TestLyricCardCopies(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tr := netease.Track{ID: 9, Name: "晴天", Artists: []netease.Artist{{Name: "周杰伦"}}}
	a.pl.Cue([]netease.Track{tr})
	a.pl.SetLyrics([]lyrics.Line{{At: 0, End: time.Second, Text: "故事的小黄花"}})
	a.pl.Hold(0)

	var got []byte
	a.saveImage = func(b []byte) error { got = b; return nil }
	a.shareLyrics()
	if len(got) == 0 {
		t.Fatal("no card was written")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(got))
	if err != nil {
		t.Fatal("the card is not an image:", err)
	}
	if cfg.Width != cardW*2 || cfg.Height != cardH*2 {
		t.Fatal("the card is not the card's size:", cfg.Width, cfg.Height)
	}
	if a.toast != "歌词卡片已复制，粘贴即可分享" {
		t.Fatal("no toast:", a.toast)
	}

	// Without lyrics, the share says so instead.
	a.saveImage = nil
	a.pl.SetLyrics(nil)
	a.shareLyrics()
	if a.toast != "还没有歌词，分享不了" {
		t.Fatal("no word about the missing lyrics:", a.toast)
	}
}
