package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/fx"
	"github.com/simqiqi/simqiqi-music/internal/netease"
)

func TestBackdropStyle(t *testing.T) {
	s := defaultSettings()
	if got := s.backdropStyle(); got != "" {
		t.Errorf("backdrop %q by default, want the cover's gradient", got)
	}
	// The fog, set before there was a choice, still answers for the fog.
	s.Fog = true
	if got := s.backdropStyle(); got != "fog" {
		t.Errorf("backdrop %q with the fog on, want fog", got)
	}
	s.setBackdrop("stars")
	if s.Fog || s.backdropStyle() != "stars" {
		t.Errorf("backdrop %q, fog %v, after picking the stars", s.backdropStyle(), s.Fog)
	}
	s.setBackdrop("")
	if s.Fog || s.backdropStyle() != "" {
		t.Errorf("backdrop %q, fog %v, after picking the gradient", s.backdropStyle(), s.Fog)
	}
	for _, b := range backdrops {
		s.setBackdrop(b.id)
		if got := s.backdropStyle(); got != b.id {
			t.Errorf("%s picked, backdrop %q", b.id, got)
		}
	}
}

// starSky paints the stars of tint at drift seconds into a window w by h,
// and returns it.
func starSky(t *testing.T, w, h int, tint ui.Color, drift, bass float32) *image.RGBA {
	t.Helper()
	view := func(c *ui.Context) {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
			fx.DrawStars(p, r, tint, drift, bass)
		})
	}
	tt := ui.NewTester(view, w, h)
	tt.Frame()
	return tt.Image()
}

// lights counts the specks of light in img, and its mean brightness: a
// speck is lighter than the mark.
func lights(img *image.RGBA) (mean float64, specks int) {
	return lightsAt(img, 110)
}

func lightsAt(img *image.RGBA, mark float64) (mean float64, specks int) {
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			l := float64(r+g+bl) / 3 / 0xffff * 255
			mean += l
			n++
			if l > mark {
				specks++
			}
		}
	}
	return mean / float64(n), specks
}

// TestStarFieldIsANightSky: the stars are specks of light over a dark
// sky, not a wash of color, and the bass lights them.
func TestStarFieldIsANightSky(t *testing.T) {
	tint := ui.RGB(52, 84, 176)
	img := starSky(t, 480, 320, tint, 12, 0)
	mean, specks := lights(img)
	n := float64(img.Bounds().Dx() * img.Bounds().Dy())
	if mean > 60 {
		t.Errorf("the sky is %0.0f bright on average, want a night sky, dark", mean)
	}
	if specks < 20 {
		t.Errorf("%d specks of light in the sky, want stars", specks)
	}
	if specks > int(n*0.06) {
		t.Errorf("%d specks of light, %0.0f%% of the sky: that is a wash, not stars", specks, 100*float64(specks)/n)
	}
	// The bass lights them, and swells the nebula.
	loud, _ := lights(starSky(t, 480, 320, tint, 12, 1))
	if loud <= mean {
		t.Errorf("the sky %0.1f bright with the bass, %0.1f without: it should light up", loud, mean)
	}
}

// TestStarFieldDrifts: the sky moves with the time it is painted at, so
// that it is worth painting again.
func TestStarFieldDrifts(t *testing.T) {
	tint := ui.RGB(180, 60, 120)
	one := starSky(t, 320, 200, tint, 0, 0.2)
	two := starSky(t, 320, 200, tint, 2.5, 0.2)
	same, changed := 0, 0
	b := one.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r1, g1, b1, _ := one.At(x, y).RGBA()
			r2, g2, b2, _ := two.At(x, y).RGBA()
			if r1 == r2 && g1 == g2 && b1 == b2 {
				same++
			} else {
				changed++
			}
		}
	}
	if changed < b.Dx()*b.Dy()/100 {
		t.Errorf("%d pixels changed in two and a half seconds, %d stayed: the sky is frozen", changed, same)
	}
}

// TestStarFieldInNowPlaying paints the sky behind a song, which is what
// the listener sees, and checks that it is the sky and not the gradient.
func TestStarFieldInNowPlaying(t *testing.T) {
	a := newTestApp()
	s := defaultSettings()
	s.setBackdrop("stars")
	a.settings = s
	url := "https://p1.music.126.net/star.jpg"
	a.art.art[netease.Sized(url, 600)+"#backdrop"] = &artwork{tint: ui.RGB(52, 84, 176)}
	a.pl.Cue([]netease.Track{{ID: 7, Name: "Starman", Duration: 240000,
		Album: netease.Album{ID: 1, Name: "Space", PicURL: url}, Artists: []netease.Artist{{Name: "Bowie"}}}})
	a.pl.SetLyrics(nil)
	a.nowPlaying, a.npPane = true, paneLyrics
	tt := ui.NewTester(a.view, 1600, 1080)
	tt.SetDark(true)
	tt.Frame()
	tt.Advance(1500 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Starman") && !tt.HasText("Bowie") {
		t.Fatal("now playing did not open on the song")
	}
	img := tt.Image()
	if a.backdropCache.style != "stars" {
		t.Errorf("the backdrop is %q, want stars", a.backdropCache.style)
	}
	// Dark, but with light in it: the sky, not the gradient.
	mean, bright := 0.0, 0
	b := img.Bounds()
	for y := 40; y < b.Dy()-140; y += 3 {
		for x := 0; x < b.Dx(); x += 3 {
			r, g, bl, _ := img.At(x, y).RGBA()
			l := float64(r+g+bl) / 3 / 0xffff * 255
			mean += l
			if l > 120 {
				bright++
			}
		}
	}
	n := float64((b.Dy() - 180) * b.Dx() / 9)
	mean /= n
	if mean > 80 {
		t.Errorf("the backdrop is %0.0f bright on average, want a night sky", mean)
	}
	if bright < 30 {
		t.Errorf("%0.0f bright samples of %0.0f in the backdrop: no stars in it", float64(bright), n)
	}
	if dir := os.Getenv("NM_SHOTS"); dir != "" {
		saveFrame(t, dir, "stars.png", img)
	}
}

// saveFrame writes img to dir/name, for a look at what was painted.
func saveFrame(t *testing.T, dir, name string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
