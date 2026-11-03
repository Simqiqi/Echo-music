package fx

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The star field is Now Playing's background as a sky: stars in three
// depths drifting up at three speeds, dust of the cover's color between
// them, and a nebula of it billowing slowly behind, all of it brighter as
// the bass comes in. Time moves it, so a canvas of it is painted thirty
// times a second while the music plays, as the fog is; paused, it drifts
// to a stop.
//
// Like the fog, it is a shader, so that the CPU may paint it small and
// once a song, and the GPU paint it half the window across, moving.

// starSource paints the sky: a deep shade of the cover's hue, a nebula of
// it moving slowly through, and stars on three grids of cells, each cell
// at most one, jittered within it, some warm and some cold, each twinkling
// in a rhythm of its own. Bass lights them and swells the nebula.
const starSource = `package main

var Time float
var Bass float
var Deep vec4
var Glow vec4

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func noise(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * (3.0 - 2.0*f)
	a := hash(i)
	b := hash(i + vec2(1.0, 0.0))
	c := hash(i + vec2(0.0, 1.0))
	d := hash(i + vec2(1, 1))
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}

func fbm(p vec2) float {
	v := 0.0
	amp := 0.5
	for i := 0; i < 4; i++ {
		v = v + amp*noise(p)
		p = p*2.03 + vec2(1.7, 9.2)
		amp = amp * 0.5
	}
	return v
}

// stars paints the stars of one depth: at most one per cell of a grid
// cells across the shorter side's 3.2 units, drifting up at spd cells a
// second, of size sz of the screen's height, at brightness bright. seed
// moves the grid to another sky; tint warms the dust of the deepest one.
func stars(p vec2, cells float, seed float, spd float, sz float, bright float, tcol vec3, tmix float, t float) vec3 {
	q := p * cells
	q = q - vec2(0.0, t*spd)
	id := floor(q)
	f := fract(q) - 0.5
	a := hash(id + vec2(seed*1.7, seed*2.3))
	if a < 0.45 {
		return vec3(0.0, 0.0, 0.0)
	}
	b := hash(id + vec2(seed*3.7+1.1, seed*1.3+7.7))
	g := hash(id + vec2(seed*9.1+4.4, seed*5.5+2.2))
	at := (vec2(b, g) - 0.5) * 0.8
	d := length(f - at)
	// The size is of the screen, but the distance is of the cell.
	s := sz * cells * (0.5 + 0.9*g)
	// Each in a rhythm of its own, none in step with another.
	tw := 0.35 + 0.65*sin(t*(0.7+2.4*b)+a*43.0)
	core := 1.0 - smoothstep(s*0.35, s, d)
	halo := (1.0 - smoothstep(0, s*2.8, d)) * 0.13
	k := (core + halo) * tw * bright * (0.35 + 0.65*a*a) * (1.0 + 0.55*Bass)
	// Cold white, and now and then a warm one.
	col := mix(vec3(0.74, 0.83, 1), vec3(1, 0.86, 0.7), smoothstep(0.78, 0.96, g))
	return mix(col, tcol, tmix) * k
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	p := pos / size.y * 3.2
	t := Time
	// Night, darker toward the foot, under the controls.
	c := mix(Deep.rgb, Deep.rgb*0.42, uv.y)
	// The nebula of the cover's color, banked, and drifting.
	rise := vec2(t*0.008, -t*0.016)
	q := vec2(fbm(p*0.5+rise), fbm(p*0.5+rise+vec2(4.3, 1.7)-vec2(t*0.011, 0)))
	n := fbm(p*0.62 + rise + 0.7*q)
	neb := smoothstep(0.44, 0.95, n) * (0.55 + 0.45*smoothstep(0.2, 0.7, fbm(p*0.3+rise*0.6)))
	c = mix(c, Glow.rgb, min(neb*(0.42+0.3*Bass), 1.0))
	glow := Glow.rgb
	// Far, middle, near, and the dust between them.
	c = c + stars(p, 30, 1, 0.8, 0.0035, 0.62, glow, 0, t)
	c = c + stars(p, 12, 2, 1.1, 0.0048, 0.78, glow, 0, t)
	c = c + stars(p, 4.5, 3, 1.6, 0.012, 1.0, glow, 0, t)
	c = c + stars(p, 1.8, 5, 2.0, 0.06, 0.22, glow, 0.65, t)
	// Darker toward the corners, and at the foot.
	g := length((uv - vec2(0.5, 0.45)) * vec2(1.0, 1.3))
	c = c * (1.06 - 0.5*smoothstep(0.2, 0.9, g))
	c = c * (1.0 - 0.22*smoothstep(0.72, 1.0, uv.y))
	return vec4(c, 1)
}
`

var starShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(starSource) })

// DrawStars paints the star field of tint in r at drift seconds, lit by
// bass.
func DrawStars(p *ui.Painter, r ui.Rect, tint ui.Color, drift, bass float32) {
	deep, glow := StarColors(tint)
	starShader().Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time": drift, "Bass": min(bass, 1), "Deep": deep, "Glow": glow,
	}})
}

// StarColors returns the night behind the stars and the color of the
// nebula in it for a cover of tint: its hue, very dark, and the same,
// dim, whatever the cover, so that white reads over it. A gray cover
// stays gray.
func StarColors(tint ui.Color) (deep, glow ui.Color) {
	h, s, _ := HSV(float64(tint.R)/255, float64(tint.G)/255, float64(tint.B)/255)
	if s > 0.12 {
		s = min(max(s*1.2, 0.4), 0.75)
	}
	return HSVColor(h, s*0.55, 0.075), HSVColor(h, s*0.72, 0.36)
}
