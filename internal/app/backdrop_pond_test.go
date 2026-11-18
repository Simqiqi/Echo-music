package app

import (
	"image"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// pondFrame paints the pond into a small window and returns it. MyGo's
// tester runs the shader on the CPU, which is the path a machine without
// GPU acceleration uses; if the shader cannot compile, this panics.
func pondFrame(t *testing.T, w, h int, drift, bass float32) *image.RGBA {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
			drawPond(p, r, ui.RGB(52, 84, 176), drift, bass)
		})
	}, w, h)
	tt.SetDark(true)
	tt.Frame()
	return tt.Image()
}

// TestPondBackdropCompiles is the shader's syntax check, and proves the
// embedded photograph is really sampled rather than a flat color.
func TestPondBackdropCompiles(t *testing.T) {
	img := pondFrame(t, 320, 200, 0, 0.3)
	seen := map[uint32]bool{}
	for y := 0; y < img.Bounds().Dy(); y += 7 {
		for x := 0; x < img.Bounds().Dx(); x += 7 {
			r, g, b, _ := img.At(x, y).RGBA()
			seen[(r>>8)<<16|(g>>8)<<8|b>>8] = true
		}
	}
	if len(seen) < 40 {
		t.Fatalf("the pond is only %d colours: the plate was not drawn", len(seen))
	}
}

// TestPondBackdropMoves: the water and the petals differ from one moment
// to the next, so the wall is not a still photograph.
func TestPondBackdropMoves(t *testing.T) {
	first, later := pondFrame(t, 320, 200, 0, 0.3), pondFrame(t, 320, 200, 6, 0.3)
	changed, n := 0, 0
	for y := 0; y < first.Bounds().Dy(); y += 3 {
		for x := 0; x < first.Bounds().Dx(); x += 3 {
			ar, ag, ab, _ := first.At(x, y).RGBA()
			br, bg, bb, _ := later.At(x, y).RGBA()
			d := absU32(ar>>8, br>>8) + absU32(ag>>8, bg>>8) + absU32(ab>>8, bb>>8)
			if d > 8 {
				changed++
			}
			n++
		}
	}
	if changed < n/20 {
		t.Errorf("only %d of %d sampled pixels moved; the water looks still", changed, n)
	}
}

func absU32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestPondBackdropIsReadable: the middle is dark enough for white lyrics,
// even though the plate is a bright photograph of shallow water.
func TestPondBackdropIsReadable(t *testing.T) {
	img := pondFrame(t, 320, 200, 2, 0.3)
	var sum, n float64
	for y := img.Bounds().Dy() / 4; y < img.Bounds().Dy()*3/4; y++ {
		for x := img.Bounds().Dx() / 4; x < img.Bounds().Dx()*3/4; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			sum += (float64(r>>8)*0.299 + float64(g>>8)*0.587 + float64(b>>8)*0.114) / 255
			n++
		}
	}
	if mean := sum / n; mean > 0.52 {
		t.Errorf("the middle of the pond is %.2f bright; white lyrics would be lost", mean)
	}
}

// TestPondSwatch: the picker shows the photograph, not a flat rectangle.
func TestPondSwatch(t *testing.T) {
	img := paintSwatch("pond", ui.RGB(52, 84, 176))
	if img.Bounds().Dx() != swatchPx || img.Bounds().Dy() != swatchPx {
		t.Fatalf("the pond swatch is %v, want %dx%d", img.Bounds(), swatchPx, swatchPx)
	}
	seen := map[uint32]bool{}
	for y := 0; y < swatchPx; y += 4 {
		for x := 0; x < swatchPx; x += 4 {
			r, g, b, _ := img.At(x, y).RGBA()
			seen[(r>>8)<<16|(g>>8)<<8|b>>8] = true
		}
	}
	if len(seen) < 20 {
		t.Errorf("the pond swatch has only %d colours", len(seen))
	}
}

// TestPondBackdropPreview writes two frames for a human to judge. It is
// off unless NM_POND_SHOT names a directory.
func TestPondBackdropPreview(t *testing.T) {
	dir := os.Getenv("NM_POND_SHOT")
	if dir == "" {
		t.Skip("set NM_POND_SHOT to write preview frames")
	}
	saveFrame(t, dir, "pond-app-t0.png", pondFrame(t, 640, 400, 0, 0.3))
	saveFrame(t, dir, "pond-app-t6.png", pondFrame(t, 640, 400, 6, 0.3))
	saveFrame(t, dir, "pond-app-t14.png", pondFrame(t, 640, 400, 14, 0.3))
}
