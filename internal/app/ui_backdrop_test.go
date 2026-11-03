package app

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// nowPlayingScene opens Now Playing on a song of a cover that is not
// loaded, so that the backdrop is painted of its tint alone, and returns
// the app and the tester showing it.
func nowPlayingScene(t *testing.T, style string, w, h int) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	s := defaultSettings()
	s.setBackdrop(style)
	a.settings = s
	url := "https://p1.music.126.net/star.jpg"
	a.art.art[netease.Sized(url, 600)+"#backdrop"] = &artwork{tint: ui.RGB(52, 84, 176)}
	a.pl.Cue([]netease.Track{{ID: 7, Name: "Starman", Duration: 240000,
		Album: netease.Album{ID: 1, Name: "Space", PicURL: url}, Artists: []netease.Artist{{Name: "Bowie"}}}})
	a.pl.SetLyrics(nil)
	a.nowPlaying, a.npPane = true, paneLyrics
	tt := ui.NewTester(a.view, w, h)
	tt.SetDark(true)
	tt.Frame()
	tt.Advance(1500 * time.Millisecond)
	tt.Frame()
	return a, tt
}

// TestBackdropPickerPicksOne: the pill at the top right of Now Playing
// names the backdrop it wears, opens onto the three, and the one picked
// is the one the settings keep.
func TestBackdropPickerPicksOne(t *testing.T) {
	a, tt := nowPlayingScene(t, "", 1400, 900)
	if !tt.HasText("渐变") {
		t.Fatal("the picker does not name the backdrop it wears")
	}
	if dir := os.Getenv("NM_SHOTS"); dir != "" {
		saveFrame(t, dir, "picker-closed.png", tt.Image())
	}
	if err := tt.Click("播放页背景"); err != nil {
		t.Fatalf("the picker does not open: %v", err)
	}
	tt.Frame()
	if dir := os.Getenv("NM_SHOTS"); dir != "" {
		saveFrame(t, dir, "picker-open.png", tt.Image())
	}
	for _, b := range backdrops {
		if !tt.HasText(b.label) {
			t.Errorf("the picker does not offer %s", b.label)
		}
	}
	if err := tt.Click("星野"); err != nil {
		t.Fatalf("the picker does not take a pick: %v", err)
	}
	tt.Frame()
	if got := a.settings.backdropStyle(); got != "stars" {
		t.Errorf("backdrop %q after picking the stars, want stars", got)
	}
	if a.settings.Fog {
		t.Error("the fog is on after picking the stars")
	}
	if !tt.HasText("星野") {
		t.Error("the picker still names the backdrop it wore before")
	}
	if a.backdropCache.style != "stars" {
		t.Errorf("the backdrop is %q, want stars", a.backdropCache.style)
	}
}

// TestBackdropSwatches paints each of the backdrops small, the way the
// picker shows them: every one of them is painted, none is flat, and no two
// look alike.
func TestBackdropSwatches(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	seen := map[string]string{}
	for _, b := range backdrops {
		img := paintSwatch(b.id, ui.RGB(52, 84, 176))
		if got := img.Bounds().Dx(); got != swatchPx {
			t.Errorf("the %s swatch is %d across, want %d", b.label, got, swatchPx)
		}
		if dir != "" {
			saveFrame(t, dir, "swatch-"+b.id+".png", img)
		}
		// Its picture, four by four: flat paper would give one colour per
		// cell, and a backdrop that shows nothing at all would give one
		// colour everywhere.
		var cells []uint32
		all := map[uint32]bool{}
		for cy := range 4 {
			for cx := range 4 {
				var r, g, bl, n uint32
				for y := cy * swatchPx / 4; y < (cy+1)*swatchPx/4; y++ {
					for x := cx * swatchPx / 4; x < (cx+1)*swatchPx/4; x++ {
						cr, cg, cb, _ := img.At(x, y).RGBA()
						r, g, bl, n = r+cr>>8, g+cg>>8, bl+cb>>8, n+1
					}
				}
				c := (r/n)<<16 | (g/n)<<8 | bl/n
				cells = append(cells, c)
				all[c] = true
			}
		}
		if len(all) < 6 {
			t.Errorf("the %s swatch is %d colours over sixteen cells: it shows nothing", b.label, len(all))
		}
		key := fmt.Sprint(cells)
		if other, ok := seen[key]; ok {
			t.Errorf("the %s and %s swatches look alike", b.label, other)
		}
		seen[key] = b.label
	}
}
