package app

import (
	"image"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/fx"
	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// scenePaint paints a window w by h with draw, and returns what it holds.
func scenePaint(t *testing.T, w, h int, draw func(p *ui.Painter, r ui.Rect)) image.Image {
	t.Helper()
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) { draw(p, r) })
	}
	tt := ui.NewTester(view, w, h)
	tt.Frame()
	return tt.Image()
}

// TestTerrainPaints: the hills run across the window, they follow the
// spectrum, and the foot of the window is darker than the sky.
func TestTerrainPaints(t *testing.T) {
	tint := ui.RGB(52, 84, 176)
	bass := [8]float32{0.95, 0.65, 0.8, 0.4, 0.55, 0.3, 0.6, 0.25}
	treble := [8]float32{0.15, 0.3, 0.35, 0.45, 0.6, 0.75, 0.9, 0.95}
	sky := scenePaint(t, 600, 400, func(p *ui.Painter, r ui.Rect) {
		fx.DrawTerrain(p, r, tint, 3.2, bass, 0.6)
	})
	if dir := os.Getenv("NM_SHOTS"); dir != "" {
		saveFrame(t, dir, "terrain.png", sky)
	}
	lum := func(img image.Image, x, y int) float64 {
		r, g, b, _ := img.At(x, y).RGBA()
		return (float64(r) + float64(g) + float64(b)) / 3 / 0xffff
	}
	// The sky is lit, the foot of the window filled in.
	if lum(sky, 300, 40) <= lum(sky, 300, 380) {
		t.Errorf("the sky is %0.2f as bright as the foot, %0.2f: the hills are not painted", lum(sky, 300, 40), lum(sky, 300, 380))
	}
	// Bass at the left, treble at the right: the two spectra stand apart.
	other := scenePaint(t, 600, 400, func(p *ui.Painter, r ui.Rect) {
		fx.DrawTerrain(p, r, tint, 3.2, treble, 0.6)
	})
	same := 0
	for x := 0; x < 600; x += 7 {
		if lum(sky, x, 300) == lum(other, x, 300) {
			same++
		}
	}
	if same > 80 {
		t.Errorf("%d of 86 columns are the same with the spectrum turned around", same)
	}
}

// TestRipplePaints: the water takes the cover's colors, bends them as the
// rings go over it, and keeps the window dark enough to read over.
func TestRipplePaints(t *testing.T) {
	cover := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for y := range 48 {
		for x := range 48 {
			// A cover of two halves, so that the water has an edge to bend.
			c := ui.RGB(230, 190, 120)
			if x+y > 48 {
				c = ui.RGB(30, 60, 140)
			}
			i := cover.PixOffset(x, y)
			cover.Pix[i], cover.Pix[i+1], cover.Pix[i+2], cover.Pix[i+3] = c.R, c.G, c.B, 255
		}
	}
	bmp := ui.NewBitmap(cover)
	defer bmp.ReleasePixels()
	img := scenePaint(t, 600, 400, func(p *ui.Painter, r ui.Rect) {
		fx.DrawRipple(p, r, bmp, ui.RGB(52, 84, 176), 4.4, 0.5)
	})
	if dir := os.Getenv("NM_SHOTS"); dir != "" {
		saveFrame(t, dir, "ripple.png", img)
	}
	// Its colors are the cover's, and not the cover itself: the water
	// darkens them, and bends them out of place.
	warm, _, _, _ := img.At(80, 60).RGBA()
	if warm>>8 < 60 {
		t.Errorf("the lit half of the cover is %d bright through the water, want its color", warm>>8)
	}
	if _, _, blue, _ := img.At(420, 340).RGBA(); blue>>8 < 40 {
		t.Errorf("the dark half of the cover is %d blue through the water, want its color", blue>>8)
	}
	same := 0
	for y := 0; y < 400; y += 3 {
		r0, _, _, _ := img.At(300, y).RGBA()
		r1, _, _, _ := img.At(300, y+1).RGBA()
		if r0>>8 == r1>>8 {
			same++
		}
	}
	if same > 120 {
		t.Errorf("%d of 133 rows are flat: the water does not move", same)
	}
}

// TestNowPlayingBackdrops paints Now Playing wearing each of the new
// backdrops, whole, for looking at.
func TestNowPlayingBackdrops(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip("no NM_SHOTS")
	}
	// A cover of its own, blurred as the backdrop takes it, so that the
	// water has one to reflect.
	img := image.NewRGBA(image.Rect(0, 0, 300, 300))
	for y := range 300 {
		for x := range 300 {
			c := ui.RGB(226, 176, 96)
			if x > y {
				c = ui.RGB(48, 92, 200)
			}
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
		}
	}
	for _, style := range []string{"terrain", "ripple", "vinyl", "water"} {
		a, tt := nowPlayingScene(t, style, 1600, 1080)
		a.art.art[netease.Sized("https://p1.music.126.net/star.jpg", 600)+"#backdrop"] =
			&artwork{tint: ui.RGB(52, 84, 176), backdrop: ui.NewBitmap(blurImage(img))}
		tt.SetDark(true)
		tt.Advance(3 * time.Second)
		tt.Frame()
		saveFrame(t, dir, "np-"+style+".png", tt.Image())
	}
}

// TestVinylPaints: the record stands out from behind the cover, its
// grooves turning, and the room is darker than the disc.
func TestVinylPaints(t *testing.T) {
	cover := ui.Rect{X: 160, Y: 200, W: 320, H: 320}
	for _, at := range []float32{0, 2.6} {
		img := scenePaint(t, 1200, 800, func(p *ui.Painter, r ui.Rect) {
			fx.DrawVinyl(p, r, r.W, cover, ui.RGB(52, 84, 176), at)
		})
		if dir := os.Getenv("NM_SHOTS"); dir != "" {
			saveFrame(t, dir, "vinyl-"+strconv.Itoa(int(at*10))+".png", img)
		}
	}
	a := scenePaint(t, 1200, 800, func(p *ui.Painter, r ui.Rect) {
		fx.DrawVinyl(p, r, r.W, cover, ui.RGB(52, 84, 176), 0)
	})
	b := scenePaint(t, 1200, 800, func(p *ui.Painter, r ui.Rect) {
		fx.DrawVinyl(p, r, r.W, cover, ui.RGB(52, 84, 176), 2.6)
	})
	// The disc turns: the grooves of one frame are not the other's.
	same := 0
	for y := 150; y < 620; y += 5 {
		r0, _, _, _ := a.At(600, y).RGBA()
		r1, _, _, _ := b.At(600, y).RGBA()
		if r0 == r1 {
			same++
		}
	}
	if same > 90 {
		t.Errorf("%d of 94 rows are the same two and a half seconds on: the disc does not turn", same)
	}
}

// TestWaterPaints: the washes carry the cover's color, and they are not
// flat: the paper has light and shade of its own.
func TestWaterPaints(t *testing.T) {
	img := scenePaint(t, 1200, 800, func(p *ui.Painter, r ui.Rect) {
		fx.DrawWatercolor(p, r, ui.RGB(52, 84, 176), 5, 0.4)
	})
	if dir := os.Getenv("NM_SHOTS"); dir != "" {
		saveFrame(t, dir, "water.png", img)
	}
	seen := map[uint32]bool{}
	for y := 40; y < 760; y += 23 {
		for x := 20; x < 1180; x += 31 {
			r, g, b, _ := img.At(x, y).RGBA()
			seen[r<<16|g<<8|b] = true
		}
	}
	if len(seen) < 200 {
		t.Errorf("the paper is %d colors, want a wash of many", len(seen))
	}
}

// TestFlowPaints: the flow holds the wallpaper's colors as a field of
// them, over the ground, at the wash's opacity, moving with time.
func TestFlowPaints(t *testing.T) {
	w := wallpaper{hue: 214, next: 262, angle: 132, lift: 0.5}
	bg := ui.Hex("#141416")
	a := scenePaint(t, 600, 400, func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, bg, 0)
		drawFlow(p, r, w, true, 0)
	})
	if dir := os.Getenv("NM_SHOTS"); dir != "" {
		saveFrame(t, dir, "flow.png", a)
	}
	at := func(img image.Image, x, y int) (int, int, int) {
		r, g, b, _ := img.At(x, y).RGBA()
		return int(r >> 8), int(g >> 8), int(b >> 8)
	}
	r0, g0, b0 := at(a, 300, 200)
	if r0 == 20 && g0 == 20 && b0 == 22 {
		t.Error("the middle is the bare ground: the flow is not painted")
	}
	// The wash holds the ground beneath: nothing is painted full over it.
	lit := false
	for x := 10; x < 590; x += 13 {
		for y := 10; y < 390; y += 11 {
			r, g, b := at(a, x, y)
			if r > 60 || g > 70 || b > 120 {
				lit = true
			}
		}
	}
	if !lit {
		t.Error("the flow holds none of its colors' light")
	}
	// And it moves: later, the field is not where it was.
	b := scenePaint(t, 600, 400, func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, bg, 0)
		drawFlow(p, r, w, true, 240)
	})
	same := 0
	for x := 10; x < 590; x += 17 {
		for y := 10; y < 390; y += 13 {
			if a.At(x, y) == b.At(x, y) {
				same++
			}
		}
	}
	if same > 900 {
		t.Errorf("%d of 1272 points are the same four minutes on: the flow does not move", same)
	}
}

// TestWaterMoves: the wash is wet, so it runs: frames seconds apart are
// not the same sheet, and it is still dark enough for the song to read.
func TestWaterMoves(t *testing.T) {
	tint := ui.RGB(52, 84, 176)
	paint := func(at float32) image.Image {
		return scenePaint(t, 480, 300, func(p *ui.Painter, r ui.Rect) {
			fx.DrawWatercolor(p, r, tint, at, 0.3)
		})
	}
	a, b := paint(0), paint(4)
	dif, most := 0, 0
	for y := 8; y < 292; y += 4 {
		for x := 8; x < 472; x += 4 {
			r0, g0, b0, _ := a.At(x, y).RGBA()
			r1, g1, b1, _ := b.At(x, y).RGBA()
			d := absI(int(r0>>8)-int(r1>>8)) + absI(int(g0>>8)-int(g1>>8)) + absI(int(b0>>8)-int(b1>>8))
			if d > 3 {
				dif++
			}
			most = max(most, d)
		}
	}
	if dif < 400 {
		t.Errorf("four seconds on only %d of 8500 points have moved: the wash is dry", dif)
	}
	if most < 12 {
		t.Errorf("the biggest change over four seconds is %d of 255: the wash barely stirs", most)
	}
	// Never so bright that the song would not read over it.
	for y := 8; y < 292; y += 9 {
		for x := 8; x < 472; x += 9 {
			r, g, b, _ := a.At(x, y).RGBA()
			if l := (int(r>>8) + int(g>>8) + int(b>>8)) / 3; l > 150 {
				t.Fatalf("the paper is %d bright at %d,%d: it would drown the song", l, x, y)
			}
		}
	}
}

func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
