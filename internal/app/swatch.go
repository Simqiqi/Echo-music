package app

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/fx"
)

// The picker shows each backdrop as a little picture of it rather than a
// rectangle of its color, and that picture is painted here, not by the
// backdrops' own shaders: those are written for a window, where a hairline
// is one pixel of a thousand and a bead is a fraction of it, so at the
// width of a thumbnail they would be nothing to see. What is drawn here is
// the idea of each backdrop at the weight a thumbnail wants — the same
// shapes, the same colors, the ridges bolder, the ripples wider apart —
// painted four pixels to a DIP and shown at four to one, so that its edges
// stay smooth when it is small.

// swatchPx is how many pixels across a swatch is painted, for the
// twenty-six DIP it shows at.
const swatchPx = 104

// paintSwatch returns the little picture of style, in the colors a cover
// of tint gives its backdrop.
func paintSwatch(style string, tint ui.Color) image.Image {
	s := newSheet(swatchPx, swatchPx)
	switch style {
	case "fog":
		swatchFog(s, tint)
	case "stars":
		swatchStars(s, tint)
	case "nebula":
		swatchNebula(s, tint)
	case "terrain":
		swatchTerrain(s, tint)
	case "ripple":
		swatchRipple(s, tint)
	default:
		swatchGradient(s, tint)
	}
	return s.image()
}

// A sheet is what a swatch is painted on: light, not color, in three
// floats a pixel, so that a glow may be added to it and a shadow laid over
// it without the sums clipping before the last step.
type sheet struct {
	w, h int
	px   []float32
}

func newSheet(w, h int) *sheet {
	return &sheet{w: w, h: h, px: make([]float32, w*h*3)}
}

// set lays c over the pixel, opaque.
func (s *sheet) set(x, y int, c ui.Color) {
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return
	}
	i := (y*s.w + x) * 3
	s.px[i], s.px[i+1], s.px[i+2] = float32(c.R), float32(c.G), float32(c.B)
}

// over lays c over the pixel, as much of it as a says.
func (s *sheet) over(x, y int, c ui.Color, a float64) {
	a = clamp01f(a)
	if a <= 0 || x < 0 || y < 0 || x >= s.w || y >= s.h {
		return
	}
	i := (y*s.w + x) * 3
	k := float32(a)
	s.px[i] = s.px[i]*(1-k) + float32(c.R)*k
	s.px[i+1] = s.px[i+1]*(1-k) + float32(c.G)*k
	s.px[i+2] = s.px[i+2]*(1-k) + float32(c.B)*k
}

// light adds c to the pixel, as much of it as k says: a star, a crest of
// water, a glint of light.
func (s *sheet) light(x, y int, c ui.Color, k float64) {
	if k <= 0 || x < 0 || y < 0 || x >= s.w || y >= s.h {
		return
	}
	i := (y*s.w + x) * 3
	s.px[i] += float32(c.R) * float32(k)
	s.px[i+1] += float32(c.G) * float32(k)
	s.px[i+2] += float32(c.B) * float32(k)
}

// image returns the sheet as a picture, its light clipped back into what a
// pixel can hold.
func (s *sheet) image() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, s.w, s.h))
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			i, o := (y*s.w+x)*3, y*img.Stride+x*4
			img.Pix[o] = byte(clampF(float64(s.px[i]), 0, 255))
			img.Pix[o+1] = byte(clampF(float64(s.px[i+1]), 0, 255))
			img.Pix[o+2] = byte(clampF(float64(s.px[i+2]), 0, 255))
			img.Pix[o+3] = 255
		}
	}
	return img
}

// swatchGradient is the cover's own gradient: its color deep at the top and
// deeper at the foot, a breath of light near the top, the shade of the
// controls at the bottom.
func swatchGradient(s *sheet, tint ui.Color) {
	top, foot := backdropColors(tint)
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		c := blendColor(top, foot, sstep(0, 1, v))
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			g := math.Exp(-(sq(u-0.5)/0.20 + sq(v-0.0)/0.09))
			px := blendColor(c, white, 0.10*g)
			s.set(x, y, blendColor(px, midnight, 0.22*sstep(0.55, 1, v)))
		}
	}
}

// swatchFog is the fog: banks of the cover's color rising through its deep
// shade, thickest a little above the middle and thinning into the corners.
func swatchFog(s *sheet, tint ui.Color) {
	deep, fog := fogColors(tint)
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			px, pv := u*2.1+3.3, v*3.0
			qx := fbm(px+0.4, pv+1.7, 3) - 0.5
			qy := fbm(px+5.2, pv+1.3, 3) - 0.5
			f := fbm(px+1.15*qx, pv+1.15*qy, 3)
			bank := fbm(px*0.45+3.7, pv*0.45, 2)
			cloud := sstep(0.37, 0.72, f) * (0.45 + 0.85*sstep(0.26, 0.60, bank))
			m := sstep(0.0, 0.42, v) * (1 - 0.45*sstep(0.6, 1, v))
			c := blendColor(deep, fog, minf(cloud*m*1.05, 1))
			g := math.Hypot((u-0.5)*1.0, (v-0.42)*1.25)
			s.set(x, y, blendColor(c, midnight, 0.40*sstep(0.26, 0.92, g)))
		}
	}
}

// swatchStars is the night the stars make of the cover's shade: a nebula of
// its light low in the sky, and the stars over it, the near ones bright and
// broad, the far ones a grain of light.
func swatchStars(s *sheet, tint ui.Color) {
	deep, glow := fx.StarColors(tint)
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			c := blendColor(deep, blendColor(deep, midnight, 0.5), sstep(0, 0.9, 1-v))
			// The nebula: the light of the cover, curdled.
			n := fbm(u*2.4+0.7, v*2.4+3.1, 3)
			neb := sstep(0.44, 0.86, n) * 0.5 * (1 - 0.55*v)
			c = blendColor(c, glow, neb*0.8)
			// Light along the horizon the sky stands on.
			c = blendColor(c, glow, 0.20*sstep(0.6, 1, v))
			s.set(x, y, c)
		}
	}
	// The stars, each a core with a halo about it: the brighter ones warm.
	for _, st := range [][4]float64{
		{0.17, 0.24, 1.05, 1.0}, {0.33, 0.12, 0.75, 0.8}, {0.52, 0.31, 0.95, 0.9},
		{0.72, 0.17, 0.7, 0.7}, {0.88, 0.38, 0.85, 1.0}, {0.26, 0.54, 0.6, 0.6},
		{0.63, 0.58, 0.8, 0.9}, {0.11, 0.71, 0.65, 0.7}, {0.44, 0.79, 0.9, 1.0},
		{0.79, 0.83, 0.7, 0.6}, {0.94, 0.66, 0.6, 0.5}, {0.57, 0.05, 0.55, 0.6},
	} {
		cx, cy := st[0]*float64(s.w), st[1]*float64(s.h)
		warm := st[3] > 0.85
		core := white
		if warm {
			core = ui.RGB(255, 240, 214)
		}
		for y := int(cy) - 5; y <= int(cy)+5; y++ {
			for x := int(cx) - 5; x <= int(cx)+5; x++ {
				d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
				k := math.Exp(-sq(d/(0.75*st[2]))) * 1.1 * st[2]
				k += math.Exp(-d/1.9) * 0.20 * st[2]
				s.light(x, y, core, k*0.9)
			}
		}
	}
}

// swatchNebula is the nebula: the cover's color blown into a slow, turning
// cloud of light with dust caught in it, the way the field shows it.
func swatchNebula(s *sheet, tint ui.Color) {
	deep := blendColor(midnight, tint, 0.22)
	glow := blendColor(tint, white, 0.12)
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			dx, dy := u-0.5, v-0.5
			r := math.Hypot(dx, dy) * 1.7
			// The whole field turns: the angle winds with the radius, so the
			// cloud reads as spinning rather than as flat rings.
			ang := math.Atan2(dy, dx) + r*2.6
			band := 0.5 + 0.5*math.Sin(r*9-ang*1.4)
			n := fbm(u*2.4+0.7, v*2.4+3.1, 3)
			cloud := sstep(0.34, 0.82, n) * (0.4 + 0.7*band) * math.Exp(-r*r*1.15)
			c := blendColor(deep, glow, cloud*0.95)
			s.set(x, y, c)
		}
	}
	// Dust, each a grain of light on its own.
	for _, d := range [][3]float64{{0.42, 0.44, 1}, {0.6, 0.58, 0.8}, {0.5, 0.32, 0.7}, {0.7, 0.4, 0.6}} {
		cx, cy, br := d[0]*float64(s.w), d[1]*float64(s.h), d[2]
		for y := int(cy) - 3; y <= int(cy)+3; y++ {
			for x := int(cx) - 3; x <= int(cx)+3; x++ {
				dd := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
				s.light(x, y, white, math.Exp(-dd*dd)*0.5*br)
			}
		}
	}
}

// swatchTerrain is the three ridges: each skyline the cover's color, the
// hills below it filled into the night, the nearest one boldest.
func swatchTerrain(s *sheet, tint ui.Color) {
	deep, glow := fx.NightColors(tint)
	ridges := [3]struct {
		base, amp, fill, seed float64
		line                  ui.Color
	}{
		{0.44, 0.10, 0.34, 1.7, blendColor(deep, glow, 0.55)},
		{0.65, 0.16, 0.19, 3.4, blendColor(deep, glow, 0.78)},
		{0.87, 0.22, 0.09, 5.9, glow},
	}
	// The sky, its light gathered along the horizon.
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		sky := blendColor(blendColor(deep, midnight, 0.38), deep, sstep(0, 0.7, v))
		sky = blendColor(sky, glow, 0.12*math.Exp(-math.Max(0.66-v, 0)*5))
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			c := sky
			for _, r := range ridges {
				d := v - ridgeAt(u, r.base, r.amp, r.seed)
				if d <= 0 {
					continue
				}
				// What stands below the skyline, darker toward the foot.
				body := blendColor(deep, midnight, 0.38*(1-sstep(0, 0.5, d)))
				body = blendColor(body, r.line, 0.30*r.fill)
				c = blendColor(c, body, 0.92*sstep(0, 1.1/float64(s.h), d))
			}
			g := math.Hypot((u-0.5)*1.0, (v-0.46)*1.15)
			s.set(x, y, blendColor(c, midnight, 0.34*sstep(0.3, 0.95, g)))
		}
	}
	// The skylines themselves: a hairline, and a glow about it, drawn
	// pixel by pixel, so that no column steps where a slope runs steep.
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			for _, r := range ridges {
				d := math.Abs(v-float64(ridgeAt(u, r.base, r.amp, r.seed))) * float64(s.h)
				k := math.Exp(-sq(d/0.78))*0.42 + math.Exp(-d/3.4)*0.12
				s.light(x, y, r.line, k)
			}
		}
	}
}

// ridgeAt is how high the skyline stands at u: the spectrum's own height,
// as the shader reads it, wandering and never flat.
func ridgeAt(u, base, amp, seed float64) float64 {
	w := 0.03*math.Sin(seed*3.1) + 0.018*math.Sin(u*4.4+seed)
	h := spectrumAt(u + w + seed*0.13)
	h = h * h * (3 - 2*h) * (0.86 + 0.14*math.Sin(u*13.7+seed*1.3))
	return base - amp*(0.05+0.68*h+0.15*spectrumAt(0.5+0.5*math.Sin(u*9.4+seed*2.3)))
}

// spectrumAt reads a still spectrum, from the bass at 0 to the treble at 1,
// the shape of a song playing quietly: a swell in the low middle, a long
// tail of it toward the treble.
func spectrumAt(x float64) float64 {
	bands := [8]float64{0.34, 0.62, 0.88, 0.58, 0.38, 0.55, 0.26, 0.18}
	v := clampF(x, 0, 1) * 7
	i := math.Floor(v)
	f := v - i
	f = f * f * (3 - 2*f)
	return mixf(bands[int(i)], bands[min(int(i)+1, 7)], f)
}

// swatchRipple is the water: the cover sunk in the deep of its own color,
// the light of the cover across it, and rings running out from where
// something fell in.
func swatchRipple(s *sheet, tint ui.Color) {
	deep, glow := fx.NightColors(tint)
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			// The cover, under the water, darker the deeper it lies.
			c := blendColor(blendColor(tint, white, 0.10), deep, sstep(0.0, 1.0, v))
			c = blendColor(c, midnight, 0.34)
			// The swell, and the crests of it.
			sw := math.Sin(v*17+1.5*math.Sin(u*5.4)) * 0.5
			c = blendColor(c, glow, 0.07*math.Max(sw, 0))
			s.set(x, y, c)
		}
	}
	// The rings, each a thin bright line with a soft one behind it.
	for _, r := range [][3]float64{
		{0.40, 0.86, 0.34}, {0.74, 0.12, 0.15}, {0.18, 0.28, 0.20},
	} {
		cx, cy, rad := r[0]*float64(s.w), r[1]*float64(s.h), r[2]*float64(s.w)
		for y := 0; y < s.h; y++ {
			for x := 0; x < s.w; x++ {
				d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
				dd := math.Abs(d - rad)
				if dd > 6 {
					continue
				}
				k := math.Exp(-sq(dd/0.70))*0.26 + math.Exp(-dd/2.6)*0.06
				// Light falls from above: the crest of a ring carries it,
				// its foot barely.
				up := 0.5 - 0.5*clampF((float64(y)+0.5-cy)/rad, -1, 1)
				k *= 0.12 + 0.88*up
				s.light(x, y, blendColor(white, glow, 0.35), k)
			}
		}
	}
	// A broad light across the water, where the room's falls.
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			b := math.Exp(-sq((u*0.7 + v*0.6 - 0.66) / 0.16))
			s.light(x, y, glow, 0.10*b)
		}
	}
}

// noiseAt is value noise: the hash at the corners of a cell, smoothed
// between them.
func noiseAt(x, y float64) float64 {
	i, j := math.Floor(x), math.Floor(y)
	fx, fy := x-i, y-j
	u, v := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a := hashAt(i, j)
	b := hashAt(i+1, j)
	c := hashAt(i, j+1)
	d := hashAt(i+1, j+1)
	return mixf(mixf(a, b, u), mixf(c, d, u), v)
}

// hashAt is the noise at a corner of a cell, in zero to one, off the sine
// of where it is.
func hashAt(x, y float64) float64 {
	return math.Abs(math.Mod(math.Sin(x*127.1+y*311.7)*43758.5453, 1))
}

// fbm is noise of octaves, each half as loud and twice as fine as the one
// before it.
func fbm(x, y float64, oct int) float64 {
	v, amp, sum := 0.0, 0.5, 0.0
	for range oct {
		v += amp * noiseAt(x, y)
		sum += amp
		x, y = x*2.03+1.7, y*2.03+9.2
		amp *= 0.5
	}
	return v / sum
}

// blendColor returns a towards b, as much of b as k says.
func blendColor(a, b ui.Color, k float64) ui.Color {
	k = clamp01f(k)
	f := func(x, y uint8) uint8 { return uint8(clampF(float64(x)+(float64(y)-float64(x))*k, 0, 255)) }
	return ui.RGB(f(a.R, b.R), f(a.G, b.G), f(a.B, b.B))
}

func sstep(e0, e1, x float64) float64 {
	if e0 == e1 {
		return 0
	}
	t := clamp01f((x - e0) / (e1 - e0))
	return t * t * (3 - 2*t)
}

func clamp01f(x float64) float64 { return clampF(x, 0, 1) }

// midnight is what a swatch darkens toward.
var midnight = ui.RGB(0, 0, 0)

func clampF(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }

func mixf(a, b, k float64) float64 { return a + (b-a)*k }

func minf(a, b float64) float64 { return math.Min(a, b) }

func sq(x float64) float64 { return x * x }
