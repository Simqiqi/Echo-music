package app

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"golang.org/x/image/draw"
)

// The pond is the one backdrop made of a photograph rather than of
// arithmetic. A plate of clear, shallow water — lotus leaves at its edges,
// sand and stems seen through it, the sun's caustics on the bottom — is
// baked once, graded and cropped, and kept as a resource. What the shader
// adds is only what a still photograph cannot have: the water bending what
// lies under it, a little skylight wandering over it, rings now and then,
// and a petal falling every few seconds. Nothing is drawn in outline; the
// picture stays a picture.
//
// The plate is 1024x640. It is the exact frame the web preview showed, so
// the app does not drift from what was approved.

//go:embed pond.jpg
var pondJPEG []byte

// pondAsset holds the plate three ways: as a bitmap for the shader to
// sample, as an image for the picker, and as a small square of it for the
// picker's swatch.
type pondAsset struct {
	bmp    *ui.Bitmap
	img    image.Image
	square *image.NRGBA
}

var pondPlate = sync.OnceValue(func() pondAsset {
	img, err := jpeg.Decode(bytes.NewReader(pondJPEG))
	if err != nil {
		// The plate is compiled in, so this cannot happen in a working
		// build; a dark sheet keeps the app drawing if a build is broken.
		img = image.NewNRGBA(image.Rect(0, 0, 2, 2))
	}
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(
		b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2,
		b.Min.X+(b.Dx()-side)/2+side, b.Min.Y+(b.Dy()-side)/2+side,
	)
	square := image.NewNRGBA(image.Rect(0, 0, swatchPx, swatchPx))
	draw.CatmullRom.Scale(square, square.Bounds(), img, crop, draw.Src, nil)
	return pondAsset{bmp: ui.NewBitmap(img), img: img, square: square}
})

// pondSource reads the plate through a moving water surface. The bend is
// two slow horizontal swells and a finer chop, small enough that a leaf
// never loses its shape. Over that go the falling petals, a breath of
// light from the shallows, three wandering patches of skylight, and the
// rings; then the middle is darkened so white lyrics keep their contrast.
const pondSource = glslCommon + `
var Time float
var Bass float
var Tint vec4

func turnPond(p vec2, a float) vec2 {
	c := cos(a)
	s := sin(a)
	return vec2(p.x*c - p.y*s, p.x*s + p.y*c)
}

func pondRnd(n float) float {
	return hash(vec2(n, 1.0))
}

// pondAspect maps a point on the window to the plate as a cover crop:
// the window keeps its own shape and the photograph is never stretched.
func pondAspect(uv vec2, size vec2, isz vec2) vec2 {
	asp := size.x / size.y
	ia := isz.x / isz.y
	if asp > ia {
		return vec2(uv.x, 0.5 + (uv.y - 0.5)*(ia/asp))
	}
	return vec2(0.5 + (uv.x - 0.5)*(asp/ia), uv.y)
}

// pondLight is one patch of reflected sky, gone at its rim.
func pondLight(uv vec2, size vec2, t float, cx float, cy float, r float, phase float) float {
	bx := cx + 0.020*sin(t*0.18 + phase)
	by := cy + 0.014*cos(t*0.14 + phase)
	d := length((uv - vec2(bx, by))*size)/(r*size.x)
	k := 0.045 + 0.025*sin(t*0.36 + phase)
	return k*(1.0 - clamp(d, 0.0, 1.0))
}

// pondRing is a ripple widening from where a drop fell, thin and faint.
func pondRing(pos vec2, size vec2, t float, sx float, sy float, idx float) float {
	age := fract(t*0.22 + idx*0.33)
	rad := age*0.20*size.x
	d := length(vec2(pos.x - sx*size.x, (pos.y - sy*size.y)/0.62))
	dd := abs(d - rad)
	return exp(-dd*dd/2.6)*(1.0 - age)*0.11
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	isz := image0Size()
	uv := pos / size
	t := Time
	b := min(Bass, 1.0)

	// The water bends what lies under it. These are the same swells as the
	// preview's horizontal bands, written continuously so no seam shows
	// when the canvas is smaller than the window.
	w := vec2(0.0, 0.0)
	w.x = 0.0016*sin(uv.y*23.5 + t*1.05) + 0.0009*sin(uv.y*8.4 - t*0.62)
	w.x = w.x + 0.00055*sin(uv.x*31.0 + uv.y*13.0 - t*0.85)
	w.y = 0.0007*cos(uv.y*14.4 + t*0.78)
	q := pondAspect(uv + w, size, isz)
	q = vec2(clamp(q.x, 0.001, 0.999), clamp(q.y, 0.001, 0.999))
	col := image0Sample(q*isz).rgb

	// Five petals, each with a clock of its own. Only one or two are in
	// the air at any moment; a landed one floats a while and fades.
	for i := 0; i < 5; i++ {
		fi := float(i)
		birth := fi*3.1 + 1.2
		cyc := 10.5
		k := mod(t - birth, cyc)
		if k > 7.6 {
			continue
		}
		x0 := 0.12 + 0.76*pondRnd(fi + 1.7)
		landY := 0.56 + 0.22*pondRnd(fi + 8.3)
		if x0 > 0.32 && x0 < 0.68 && landY > 0.40 && landY < 0.58 {
			landY = landY + 0.12
		}
		fall := 3.1 + 1.1*pondRnd(fi + 4.9)
		sz := (0.010 + 0.006*pondRnd(fi + 2.2))*size.x
		baseRot := pondRnd(fi + 6.1)*6.2831853

		// The little ring where it touches the water.
		if k >= fall && k < fall + 0.9 {
			ra := (k - fall)/0.9
			rd := length(vec2(pos.x - x0*size.x, (pos.y - landY*size.y)/0.62))
			dd := abs(rd - ra*size.x*0.030)
			col = mix(col, vec3(0.902, 0.980, 0.956), exp(-dd*dd/1.6)*0.13*(1.0 - ra))
		}

		x := x0
		y := landY
		rot := baseRot
		flip := 1.0
		alpha := 0.78
		if k < fall {
			p := k/fall
			y = -0.06 + (landY + 0.06)*p*p
			x = x0 + sin(k*1.05 + fi)*0.018 + sin(k*0.42 + fi*2.0)*0.011
			rot = baseRot + sin(k*1.5 + fi)*0.48 + k*0.22
			flip = 0.58 + 0.42*abs(cos(k*2.0 + fi))
			alpha = 0.78
		} else {
			f := k - fall
			y = landY + sin(f*0.85 + fi)*0.004
			x = x0 + sin(f*0.48 + fi)*0.012 + f*0.0016
			rot = baseRot + sin(f*0.6 + fi)*0.22
			flip = 1.0
			alpha = 0.68*exp(-f*0.42)
		}
		if alpha < 0.02 {
			continue
		}

		// Most of the window is nowhere near a petal; skip it early.
		d := pos - vec2(x*size.x, y*size.y)
		bound := sz*1.5
		if abs(d.x) > bound || abs(d.y) > bound*1.7 {
			continue
		}
		lp := turnPond(d, -rot)
		lx := lp.x/flip
		yn := (lp.y + sz)/(1.5*sz)
		if yn > 0.0 && yn < 1.0 {
			// A pointed petal: narrow at both tips, widest a little
			// above its foot, the way the preview's bezier falls.
			w := 0.66*sz*pow(max(sin(3.14159265*yn), 0.0), 1.25)*(1.0 - 0.18*yn)
			pd := abs(lx) - w
			m := 1.0 - smoothstep(-0.45, 0.45, pd)
			// Pale at the tip, pink at the foot, with a fine vein.
			pc := mix(vec3(1.0, 0.943, 0.943), vec3(0.945, 0.624, 0.700), yn)
			if abs(lx) < 0.7 && yn > 0.10 && yn < 0.88 {
				pc = mix(pc, vec3(0.839, 0.573, 0.659), 0.32)
			}
			col = mix(col, pc, m*alpha)
		}
	}

	// The shallows let a little light up through the water.
	d := length((uv - vec2(0.5, 0.55))*size)/(size.x*0.42)
	col = col + vec3(0.588, 0.882, 0.843)*0.032*exp(-d*d*1.8)

	// Three patches of reflected sky, drifting as the surface turns.
	sky := pondLight(uv, size, t, 0.24, 0.30, 0.32, 0.4)
	sky = sky + pondLight(uv, size, t, 0.74, 0.64, 0.28, 2.2)
	sky = sky + pondLight(uv, size, t, 0.50, 0.84, 0.24, 4.1)
	col = col + vec3(0.620, 0.847, 0.808)*sky*(0.90 + 0.20*b)

	// Rings from three places, each fading as it widens.
	rings := pondRing(pos, size, t, 0.33, 0.41, 0.0)
	rings = rings + pondRing(pos, size, t, 0.68, 0.34, 1.0)
	rings = rings + pondRing(pos, size, t, 0.54, 0.71, 2.0)
	col = mix(col, vec3(0.871, 0.965, 0.933), min(rings, 0.30))

	// The middle is darkened for the lyrics, and the corners again, as
	// the preview did. The photograph keeps its edges and its detail.
	dm := length(pos - vec2(size.x*0.5, size.y*0.48))/(size.x*0.48)
	shade := 0.50 - 0.20*smoothstep(0.0, 0.55, dm) - 0.30*smoothstep(0.55, 1.0, dm)
	col = mix(col, vec3(0.012, 0.063, 0.071), clamp(shade, 0.0, 1.0))
	dv := length(pos - vec2(size.x*0.5, size.y*0.48))/(size.x*0.78)
	col = mix(col, vec3(0.0, 0.039, 0.047), 0.42*smoothstep(0.385, 1.0, dv))

	// Only a breath of the cover in it: this wall is a photograph.
	col = mix(col, Tint.rgb*0.42, 0.015)
	return vec4(col, 1)
}
`

var pondShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(pondSource) })

// drawPond paints the water of r at drift seconds, lit by bass.
func drawPond(p *ui.Painter, r ui.Rect, tint ui.Color, drift, bass float32) {
	pondShader().Draw(p, r, shader.Options{
		Uniforms: map[string]any{"Time": drift, "Bass": min(bass, 1), "Tint": tint},
		Images:   [4]*ui.Bitmap{pondPlate().bmp},
	})
}

// swatchPond shows the picker the photograph itself, with the same middle
// darkening: the one backdrop whose little picture is not drawn by hand.
func swatchPond(s *sheet, tint ui.Color) {
	img := pondPlate().square
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			c := colorAtPond(img.At(x, y))
			dm := math.Hypot(float64(x)-float64(s.w)*0.5, float64(y)-float64(s.h)*0.48) / (float64(s.w) * 0.48)
			shade := 0.50 - 0.20*sstep(0, 0.55, dm) - 0.30*sstep(0.55, 1, dm)
			c = blendColor(c, ui.RGB(3, 16, 18), clamp01f(shade))
			dv := math.Hypot(float64(x)-float64(s.w)*0.5, float64(y)-float64(s.h)*0.48) / (float64(s.w) * 0.78)
			c = blendColor(c, ui.RGB(0, 10, 12), 0.42*sstep(0.385, 1, dv))
			s.set(x, y, c)
		}
	}
}

func colorAtPond(c color.Color) ui.Color {
	r, g, b, _ := c.RGBA()
	return ui.RGB(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}
