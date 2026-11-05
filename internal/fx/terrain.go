package fx

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The terrain is Now Playing's background as a range of hills at night:
// three ridges, one behind another, their skyline the spectrum itself, so
// that the bass heaves the nearest one and the treble frets its far end.
// Each ridge is a hairline of the cover's color over the dark of it, and
// what lies below a ridge is filled in, so that the nearer hills stand in
// front of the further ones.
//
// Like the fog and the stars, it is a shader, so that the CPU may paint it
// small and once a song, and the GPU paint it half the window across,
// moving, thirty times a second while the music plays.

// terrainSource paints it: the sky of the cover's deep shade, lit along
// the horizon, and the three ridges of its glow, whose heights are the
// spectrum's, read left to right from the bass up to the treble.
const terrainSource = `package main

var Time float
var Bass float
var Deep vec4
var Glow vec4
var S0 float
var S1 float
var S2 float
var S3 float
var S4 float
var S5 float
var S6 float
var S7 float

// pick returns band i of the spectrum, of the eight the shader holds.
func pick(i float) float {
	if i < 0.5 {
		return S0
	}
	if i < 1.5 {
		return S1
	}
	if i < 2.5 {
		return S2
	}
	if i < 3.5 {
		return S3
	}
	if i < 4.5 {
		return S4
	}
	if i < 5.5 {
		return S5
	}
	if i < 6.5 {
		return S6
	}
	return S7
}

// band reads the spectrum at x, from 0 at the bass to 1 at the treble,
// smoothed between the bands either side of it.
func band(x float) float {
	v := clamp(x, 0.0, 1.0) * 7.0
	i := floor(v)
	f := v - i
	f = f * f * (3.0 - 2.0*f)
	return mix(pick(i), pick(i + 1.0), f)
}

// shape is how high the ridge stands at x: the spectrum's own height,
// loud where the music is and quiet where it is not, wandering a little
// and breathing between the notes, and never flat.
func shape(x float, t float, seed float) float {
	w := 0.03*sin(t*0.21+seed*3.1) + 0.018*sin(t*0.09+x*4.4+seed)
	h := band(x + w)
	h = h*h*(3.0-2.0*h)*(0.86+0.14*sin(t*0.37+seed*1.7))
	return 0.05 + 0.80*h + 0.15*band(0.5+0.5*sin(x*9.4+seed*2.3))
}

// ridge draws one hill over c: what stands below it filled in darker the
// nearer it comes, its skyline a hairline of tint, and a glow about it.
func ridge(c vec3, uv vec2, size vec2, base float, amp float, tint vec3, fill float, seed float, t float) vec3 {
	y := base - amp*shape(uv.x, t, seed)
	d := abs(uv.y - y)
	if uv.y > y {
		k := smoothstep(0.0, 0.55, uv.y-y)
		c = mix(c, tint*fill*(1.0-0.4*k), 0.9)
	}
	core := 1.0 - smoothstep(0.0, 1.4/size.y, d)
	glow := exp(-d*40.0)
	return c + tint*core*(0.75+0.5*Bass) + tint*glow*(0.10+0.16*Bass)
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	t := Time

	// The sky: the deep shade of the cover, its light gathered along the
	// horizon the hills stand on, and the dark of night above.
	c := Deep.rgb * (0.62 + 0.5*smoothstep(0.0, 0.6, uv.y))
	c = c + Glow.rgb*0.10*exp(-max(0.6-uv.y, 0.0)*5.0)

	// Three ridges, the far one first, each a little lower and a little
	// bolder than the one behind it.
	c = ridge(c, uv, size, 0.50, 0.13, mix(Deep.rgb, Glow.rgb, 0.55), 0.34, 1.7, t)
	c = ridge(c, uv, size, 0.67, 0.17, mix(Deep.rgb, Glow.rgb, 0.78), 0.19, 3.4, t)
	c = ridge(c, uv, size, 0.86, 0.22, Glow.rgb, 0.09, 5.9, t)

	// Darker toward the corners, as the night is.
	vg := length((uv - vec2(0.5, 0.46)) * vec2(1.0, 1.15))
	c = c * (1.06 - 0.34*smoothstep(0.25, 0.95, vg))
	return vec4(c, 1)
}
`

var terrainShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(terrainSource) })

// DrawTerrain paints the hills of tint in r at drift seconds, as high as
// bands say and as bright as bass does.
func DrawTerrain(p *ui.Painter, r ui.Rect, tint ui.Color, drift float32, bands [8]float32, bass float32) {
	deep, glow := NightColors(tint)
	terrainShader().Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time": drift, "Bass": min(bass, 1), "Deep": deep, "Glow": glow,
		"S0": bands[0], "S1": bands[1], "S2": bands[2], "S3": bands[3],
		"S4": bands[4], "S5": bands[5], "S6": bands[6], "S7": bands[7],
	}})
}

// NightColors returns the dark behind a night scene and its light, for
// a cover of tint: its hue, deeper than a night sky and brighter than a
// lamp, so that white reads over both. A gray cover stays gray.
func NightColors(tint ui.Color) (deep, glow ui.Color) {
	h, s, _ := HSV(float64(tint.R)/255, float64(tint.G)/255, float64(tint.B)/255)
	if s > 0.12 {
		s = min(max(s*1.35, 0.55), 0.9)
	}
	return HSVColor(h, s*0.75, 0.13), HSVColor(h, s*0.85, 0.78)
}
