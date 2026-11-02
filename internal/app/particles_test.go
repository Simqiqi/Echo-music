package app

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// A cover of stripes, so that the specks of the record show where on the
// cover each of them stands.
var stripeColors = []ui.Color{ui.RGB(220, 120, 160), ui.RGB(90, 140, 220), ui.RGB(240, 200, 110),
	ui.RGB(120, 200, 170), ui.RGB(170, 120, 230), ui.RGB(230, 140, 100), ui.RGB(120, 170, 210)}

func stripedCover() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		c := stripeColors[y*len(stripeColors)/64]
		for x := range 64 {
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
		}
	}
	return img
}

// particleApp stands Now Playing on the particle record, a cover of stripes
// as the song's, in a window of w×h, small enough to shade every frame of
// it soon: the record's shader shades every pixel of the pane afresh, and
// the software of the tests does it slowly.
func particleApp(t *testing.T, w, h int) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	a.art.art[netease.Sized("art1", 512)] = &artwork{bmp: ui.NewBitmap(stripedCover()), tint: stripeColors[2]}
	a.pl.Cue([]netease.Track{{ID: 1, Name: "歌", Duration: 200000, Album: netease.Album{PicURL: "art1"}}})
	a.pl.SetLyrics(nil)
	a.nowPlaying, a.npPane = true, paneParticles
	tt := ui.NewTester(a.view, w, h)
	tt.SetDark(true)
	return a, tt
}

// The record's own shader stands as it is written, before a frame of it is
// shaded.
func TestRecordShaderCompiles(t *testing.T) {
	recordShader()
	sparkShader()
}

// The specks make the cover out as the cover itself: the stripes of it show
// in the pane where they stand on the cover, the cover small in the middle
// of where the pane puts it and hung askew, its near side the wider, and
// its edge coming apart into the dark with no frame to mark where it stops.
func TestParticleRecordShowsTheCover(t *testing.T) {
	const w, h = 360, 240
	a, tt := particleApp(t, w, h)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	shot := tt.Image()
	b := shot.Bounds()
	// Where the pane stands the record: the pane leaves it the middle of
	// the room it has, and the test reads that from the record itself.
	rec := a.recAt
	s := rec.W
	cx, cy := int(rec.X+rec.W/2), int(rec.Y+rec.H/2)

	// The cover stands on its feet, not on its head: the stripes run down
	// the pane as they run down the cover, from its head to its foot. The
	// test asks nothing of how the tilt is reckoned, only that what is at
	// the head of the cover shows at the head of the pane.
	head := stripeIndex(averageAround(shot, cx, cy-int(s*0.28)))
	middle := stripeIndex(averageAround(shot, cx, cy))
	foot := stripeIndex(averageAround(shot, cx, cy+int(s*0.28)))
	if !(head < middle && middle < foot) {
		t.Errorf("the pane shows stripe %v at its head, %v at its middle and %v at its foot, want the cover's stripes down it from its head", head, middle, foot)
	}

	// The cover stands smaller than the pane, in the middle of it: its
	// specks take up the side the pane gave it across the middle row, and
	// leave the corners of the pane their own dark.
	left, right, ok := litSpan(shot, cy)
	if !ok {
		t.Fatal("no speck of the cover anywhere along the middle of the pane")
	}
	if span := float32(right - left + 1); span < s*0.60 || span > s*1.15 {
		t.Errorf("the cover spans %v across the pane, want about %v", span, s)
	}
	if mid := (left + right) / 2; absInt(mid-cx) > w/10 {
		t.Errorf("the cover stands at %v across the pane, want it in the middle at %v", mid, cx)
	}
	for _, at := range []image.Point{{b.Min.X + 4, b.Min.Y + 4}, {b.Max.X - 5, b.Min.Y + 4},
		{b.Min.X + 4, b.Max.Y - 5}, {b.Max.X - 5, b.Max.Y - 5}} {
		if c := averageAround(shot, at.X, at.Y); c[0]+c[1]+c[2] > 45 {
			t.Errorf("at %v the pane shows %v, want the dark round the cover and no frame of it", at, c)
		}
	}

	// It stands square to the eye, as the page opens with the pointer
	// asking nothing of it: as wide above its middle as below.
	upL, upR, upOK := litSpan(shot, cy-int(s*0.15))
	dnL, dnR, dnOK := litSpan(shot, cy+int(s*0.15))
	if !upOK || !dnOK {
		t.Fatal("no speck of the cover above or below its middle")
	}
	if up, dn := upR-upL, dnR-dnL; absInt(up-dn) > 2 {
		t.Errorf("the cover stands %v wide above its middle and %v below, want it square to the eye at the page's opening", up, dn)
	}
}

// The pointer is what gives the record its depth: at the head of the pane it
// leans the head to the eye and the cover shows the wider there, at the foot
// the other way about, and with the pointer gone it stands square again.
func TestParticleRecordLeansAfterThePointer(t *testing.T) {
	const w, h = 360, 240
	a, tt := particleApp(t, w, h)
	tt.Frame()
	rec := a.recAt
	s := rec.W
	cx, cy := int(rec.X+rec.W/2), int(rec.Y+rec.H/2)
	widths := func() (up, dn int) {
		tt.Frame()
		shot := tt.Image()
		upL, upR, upOK := litSpan(shot, cy-int(s*0.15))
		dnL, dnR, dnOK := litSpan(shot, cy+int(s*0.15))
		if !upOK || !dnOK {
			t.Fatalf("no speck of the cover above or below its middle at %v", tt.Image().Bounds())
		}
		return upR - upL, dnR - dnL
	}
	// The pointer high in the pane: the head of the cover comes to the eye.
	tt.Move(float32(cx), 4)
	tt.Advance(200 * time.Millisecond)
	if up, dn := widths(); up < dn+2 {
		t.Errorf("with the pointer high in the pane the cover stands %v wide above its middle and %v below, want it the wider above", up, dn)
	}
	// The pointer low in the pane: the foot comes to the eye instead.
	tt.Move(float32(cx), h-4)
	tt.Advance(200 * time.Millisecond)
	if up, dn := widths(); dn < up+2 {
		t.Errorf("with the pointer low in the pane the cover stands %v wide above its middle and %v below, want it the wider below", up, dn)
	}
	// And with the pointer gone from the window, it stands square again.
	a.recTilt.tx, a.recTilt.ty = 0, 0
	tt.Advance(600 * time.Millisecond)
	if up, dn := widths(); absInt(up-dn) > 2 {
		t.Errorf("with the pointer gone the cover stands %v wide above its middle and %v below, want it square again", up, dn)
	}
}

// The record keeps the middle of the room the pane has; the line being sung
// keeps a band under it, so that the two never stand in one another, or a
// band across the middle of the pane, over a record that keeps its room.
func TestParticleLayoutKeepsTheLineItsBand(t *testing.T) {
	const top = 64
	for _, size := range [][2]float32{{1200, 800}, {900, 620}, {720, 480}, {360, 240}} {
		w, h := size[0], size[1]
		for _, lyrics := range []bool{true, false} {
			for _, mid := range []bool{true, false} {
				if mid && !lyrics {
					continue // nothing stands across the middle of a page without a line
				}
				rec, line := particleLayout(top, w, h, lyrics, mid)
				if rec.W != rec.H || rec.W <= 0 {
					t.Fatalf("%v×%v lyrics %v mid %v: the record is %v wide and %v tall", w, h, lyrics, mid, rec.W, rec.H)
				}
				if rec.X+rec.W/2 != w/2 {
					t.Errorf("%v×%v lyrics %v mid %v: the record stands at %v across the pane, want it in the middle at %v", w, h, lyrics, mid, rec.X+rec.W/2, w/2)
				}
				if rec.Y < top-0.01 || rec.Y+rec.H > h+0.01 {
					t.Errorf("%v×%v lyrics %v mid %v: the record runs from %v to %v, want it in the pane from %v to %v", w, h, lyrics, mid, rec.Y, rec.Y+rec.H, top, h)
				}
				wantBand := float32(0)
				if lyrics {
					wantBand = particleLyric
				}
				if line.H != wantBand {
					t.Errorf("%v×%v lyrics %v mid %v: the band for the line is %v tall, want %v", w, h, lyrics, mid, line.H, wantBand)
				}
				if !lyrics {
					continue
				}
				if mid {
					// The band stands across the middle of the room the
					// pane has, and the record under it keeps the room it
					// has without a line.
					free, _ := particleLayout(top, w, h, false, false)
					if rec != free {
						t.Errorf("%v×%v mid: the record is %v, want the room it has without a line, %v", w, h, rec, free)
					}
					// A pane low enough that its room is held to a least
					// height has no middle to stand the line in; a pane
					// with room to spare stands it across the middle.
					if room := h - top - particleFoot; room >= 180 {
						middle := top + room/2
						if at := line.Y + line.H/2; at < middle-1 || at > middle+1 {
							t.Errorf("%v×%v mid: the line stands at %v down the pane, want it across the middle at %v", w, h, at, middle)
						}
					}
					continue
				}
				if rec.Y+rec.H > line.Y+0.01 {
					t.Errorf("%v×%v: the record runs to %v and the line begins at %v, want the record to keep above it", w, h, rec.Y+rec.H, line.Y)
				}
			}
		}
	}
}

// The record beats to the song: a strike of the drum (the bass leaping)
// brings it a breath forward, the breath dies away while the bass holds,
// and in silence it stands as it stands.
func TestPulseStateSteps(t *testing.T) {
	var p pulseState
	now := time.Now()
	quiet, loud := [player.Bands]float32{}, [player.Bands]float32{}
	for i := range loud {
		loud[i] = 0.9
	}
	if v := p.step(now, quiet, false); v != 0 {
		t.Errorf("in silence the record comes forward %v, want it still", v)
	}
	p.step(now.Add(16*time.Millisecond), quiet, true)
	if v := p.step(now.Add(32*time.Millisecond), loud, true); v < 0.4 {
		t.Errorf("the bass leapt and the record came forward %v, want it a good way forward", v)
	}
	// It settles back while the bass holds, the strike dying away.
	var peak, last float32
	for i := range 60 {
		v := p.step(now.Add(time.Duration(48+i*16)*time.Millisecond), loud, true)
		peak, last = max(peak, v), v
	}
	if last > peak*0.6 {
		t.Errorf("the strike has died and the record still stands %v forward of the %v it leapt to, want it settling back", last, peak)
	}
	// And in silence it comes no further.
	for i := range 40 {
		p.step(now.Add(time.Duration(2+i)*time.Second), quiet, false)
	}
	if v := p.step(now.Add(10*time.Second), quiet, false); v != 0 {
		t.Errorf("in silence the record comes forward %v, want it still", v)
	}
}

// The record comes forward at a strike: at the full push its specks stand a
// breath wider than at rest, and no more than a breath, for the cover must
// go on reading as the cover.
func TestParticleRecordComesForward(t *testing.T) {
	const w, h = 360, 360
	const ext = 200
	spanAt := func(push float32) int {
		view := func(c *ui.Context) {
			box := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0)
			box.Draw(func(p *ui.Painter, r ui.Rect) {
				drawParticles(p, r, ui.NewBitmap(stripedCover()), stripeColors[2],
					ui.Point{X: w / 2.0, Y: h / 2.0}, ext, 0, 0, push, 0)
			})
		}
		tt := ui.NewTester(view, w, h)
		tt.SetDark(true)
		tt.Frame()
		left, right, ok := litSpan(tt.Image(), h/2)
		if !ok {
			t.Fatalf("push %v: no speck of the cover anywhere along the middle", push)
		}
		return right - left + 1
	}
	rest, pushed := spanAt(0), spanAt(1)
	if pushed <= rest {
		t.Errorf("the record stands %v across at rest and %v at the strike, want it a breath wider as it comes forward", rest, pushed)
	}
	if pushed > rest+rest/20 {
		t.Errorf("the record stands %v across at rest and %v at the strike, want no more than a twentieth wider", rest, pushed)
	}
}

// averageAround is the color of the pane about x, y, over a few pixels, for
// the specks stand in a dark between them.
func averageAround(shot image.Image, x, y int) [3]float32 {
	var sum [3]float32
	var n float32
	for dy := -3; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			r, g, bl, _ := shot.At(x+dx, y+dy).RGBA()
			sum[0] += float32(r >> 8)
			sum[1] += float32(g >> 8)
			sum[2] += float32(bl >> 8)
			n++
		}
	}
	return [3]float32{sum[0] / n, sum[1] / n, sum[2] / n}
}

// litSpan returns the leftmost and rightmost speck of the cover along the
// row y, and whether there is one at all.
func litSpan(shot image.Image, y int) (left, right int, ok bool) {
	b := shot.Bounds()
	left, right = b.Max.X, b.Min.X
	for x := b.Min.X; x < b.Max.X; x++ {
		if c := averageAround(shot, x, y); c[0]*0.3+c[1]*0.5+c[2]*0.2 > 30 {
			left, right = min(left, x), max(right, x)
		}
	}
	return left, right, right >= left
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// With NM_SHOTS set, saves a picture of the record the size of a window,
// for looking at.
func TestParticleRecordFrame(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	_, tt := particleApp(t, 720, 480)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "particles.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// stripeIndex returns the index of the stripe of the cover whose hue c is
// nearest, and -1 where c shows nothing of the cover at all: c is taken to
// its full brightness first, for the specks show a stripe dimmed by the
// dark between them.
func stripeIndex(c [3]float32) int {
	hi := max(c[0], c[1], c[2])
	if hi < 12 {
		return -1
	}
	hue := [3]float64{float64(c[0]) / float64(hi) * 255, float64(c[1]) / float64(hi) * 255, float64(c[2]) / float64(hi) * 255}
	best, at := math.MaxFloat64, -1
	for i, s := range stripeColors {
		top := max(float64(s.R), float64(s.G), float64(s.B))
		shade := [3]float64{float64(s.R) / top * 255, float64(s.G) / top * 255, float64(s.B) / top * 255}
		d := area(hue[0]-shade[0]) + area(hue[1]-shade[1]) + area(hue[2]-shade[2])
		if d < best {
			best, at = d, i
		}
	}
	return at
}

func area(f float64) float64 { return f * f }

// particleLyricApp stands Now Playing on the particles page with words to
// sing, the song a little way into a line.
func particleLyricApp(t *testing.T, w, h int) (*app, *ui.Tester) {
	t.Helper()
	a, tt := particleApp(t, w, h)
	raw := []lyrics.Line{
		{At: 0, End: 20 * time.Second, Text: "作词：周杰伦"},
		{At: 20 * time.Second, End: 26 * time.Second, Text: "故事的小黄花", Translation: "the little yellow flower"},
		{At: 26 * time.Second, End: 32 * time.Second, Text: "从出生那年就飘着"},
	}
	for i := range raw {
		raw[i].Words = lyrics.SpreadWords(raw[i].Text, raw[i].At, raw[i].End)
	}
	a.pl.SetLyrics(raw)
	a.pl.Hold(22 * time.Second)
	return a, tt
}

// The line being sung shows along the foot of the record, below it, where
// the pane keeps a band for it.
func TestParticleRecordShowsTheLine(t *testing.T) {
	const w, h = 480, 400
	a, tt := particleLyricApp(t, w, h)
	tt.Frame()
	band := a.recLyric
	if band.H <= 0 {
		t.Fatal("the pane keeps no band for the line being sung")
	}
	if a.recAt.Y+a.recAt.H > band.Y+0.01 {
		t.Errorf("the record runs to %v and the line begins at %v, want the record to keep above it", a.recAt.Y+a.recAt.H, band.Y)
	}
	shot := tt.Image()
	bright := 0
	for y := int(band.Y) + 1; y < int(band.Y+band.H)-1; y++ {
		for x := 2; x < w-2; x++ {
			r, g, b, _ := shot.At(x, y).RGBA()
			if (r>>8)+(g>>8)+(b>>8) > 3*150 {
				bright++
			}
		}
	}
	if bright < 30 {
		t.Errorf("the band shows %v bright pixels, want the line being sung in it", bright)
	}
}

// The switch takes the line away, and gives the record the band back.
func TestParticleLyricSwitchTakesTheLineAway(t *testing.T) {
	const w, h = 480, 400
	a, tt := particleLyricApp(t, w, h)
	tt.Frame()
	shown := a.recAt.W
	if a.recLyric.H == 0 {
		t.Fatal("the band for the line is there before the switch is touched")
	}
	a.settings.ParticleLyricsOff = true
	tt.Frame()
	if a.recLyric.H != 0 {
		t.Errorf("the line is off, and the pane keeps a band %v tall for it", a.recLyric.H)
	}
	if a.recAt.W <= shown {
		t.Errorf("the line is off, and the record is %v across, want it the room the band gave back", a.recAt.W)
	}
}

// Without the line the record leans after the pointer: at one edge of the
// pane it turns one way, at the other the other.
func TestParticleLeansWithThePointer(t *testing.T) {
	const w, h = 260, 200
	a, tt := particleApp(t, w, h)
	a.settings.ParticleLyricsOff = true
	tt.Frame()
	tt.Move(w-4, h/2)
	tt.Frame()
	if a.recTilt.tx < 0.8 {
		t.Fatalf("the pointer at the right edge asks the record to lean %v across, want it near the edge", a.recTilt.tx)
	}
	tt.Advance(120 * time.Millisecond)
	if a.recTilt.x < 0.25 {
		t.Errorf("the record leans %v across after the pointer at the right edge, want it leaning that way", a.recTilt.x)
	}
	tt.Move(4, h/2)
	tt.Frame()
	if a.recTilt.tx > -0.8 {
		t.Fatalf("the pointer at the left edge asks the record to lean %v across, want it near the other edge", a.recTilt.tx)
	}
	tt.Advance(120 * time.Millisecond)
	if a.recTilt.x > -0.25 {
		t.Errorf("the record leans %v across after the pointer at the left edge, want it leaning back the other way", a.recTilt.x)
	}
}

// The page opens with the record square to the eye: no pointer has asked
// anything of it yet, so it leans not at all, line or no line.
func TestParticleOpensSquare(t *testing.T) {
	for _, off := range []bool{true, false} {
		a, tt := particleLyricApp(t, 260, 200)
		a.settings.ParticleLyricsOff = off
		tt.Frame()
		if a.recTilt.x != 0 || a.recTilt.y != 0 || a.recTilt.tx != 0 || a.recTilt.ty != 0 {
			t.Errorf("lyrics off %v: the page opens with the record leaning %v, %v, asked %v, %v, want it square", off, a.recTilt.x, a.recTilt.y, a.recTilt.tx, a.recTilt.ty)
		}
	}
}

// The line over the record does not hold it: the pointer leans the record
// with the line shown as it does without it.
func TestParticleLeansWithTheLine(t *testing.T) {
	const w, h = 260, 200
	a, tt := particleLyricApp(t, w, h)
	tt.Frame()
	tt.Move(w-4, h/2)
	tt.Advance(160 * time.Millisecond)
	if a.recTilt.x < 0.2 {
		t.Errorf("the line is shown, and the record leans %v across after the pointer at the edge, want it leaning", a.recTilt.x)
	}
}

// Choosing the middle of the pane stands the line there, over a record that
// keeps the whole of the room it has.
func TestParticleLineStandsInTheMiddle(t *testing.T) {
	const w, h = 480, 400
	a, tt := particleLyricApp(t, w, h)
	tt.Frame()
	alone := a.recAt // the record of the page with the line under it
	a.settings.ParticleLyricMid = true
	tt.Frame()
	if a.recAt.W <= alone.W {
		t.Errorf("the record is %v across with the line across the middle, want it the room it has without a line, %v", a.recAt.W, alone.W)
	}
	band := a.recLyric
	middle := float32(h) / 2
	if at := band.Y + band.H/2; absFloat(at-middle) > 60 {
		t.Errorf("the line stands at %v down the pane, want it near the middle at %v", at, middle)
	}
	if band.Y+band.H < a.recAt.Y || band.Y > a.recAt.Y+a.recAt.H {
		t.Errorf("the line stands at %v and the record from %v to %v, want the line over the record", band.Y, a.recAt.Y, a.recAt.Y+a.recAt.H)
	}
}

// With the pointer gone the record comes back to where it stands.
func TestParticleSettlesWhenThePointerLeaves(t *testing.T) {
	const w, h = 260, 200
	a, tt := particleApp(t, w, h)
	a.settings.ParticleLyricsOff = true
	tt.Frame()
	tt.Move(w-4, h/2)
	tt.Advance(160 * time.Millisecond)
	if a.recTilt.x < 0.2 {
		t.Fatalf("the record leans %v across with the pointer at the edge, want it leaning", a.recTilt.x)
	}
	a.recTilt.tx, a.recTilt.ty = 0, 0
	tt.Advance(400 * time.Millisecond)
	if a.recTilt.x > 0.1 {
		t.Errorf("the record leans %v across with nothing asked of it, want it back where it stands", a.recTilt.x)
	}
}

// With NM_SHOTS set, saves pictures of the record with the line being sung
// under it and across its middle, for looking at.
func TestParticleLyricsFrame(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, shot := range []struct {
		name string
		mid  bool
	}{{"particles-line.png", false}, {"particles-line-mid.png", true}} {
		a, tt := particleLyricApp(t, 720, 480)
		a.settings.ParticleLyricMid = shot.mid
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, shot.name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}
