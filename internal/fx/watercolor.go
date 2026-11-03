package fx

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The watercolor is Now Playing's background as a sheet of dark paper with
// the cover's colors washed over it: three inks, each its own hue near the
// cover's, bleeding into the paper and into each other, their edges
// gathering where they dry, the paper's grain showing through.
//
// It is wet, so it moves: the pigment is carried along by currents of its
// own, each wash's wet front creeps outward and draws back again as the
// paper takes water and dries, and now and then a drop lands in the wet
// paper and pushes the ink out in a ring that widens and thins away. The
// bass opens the washes a little wider and quickens the rings.
//
// Like the fog and the stars, it is a shader, so that the CPU may paint it
// small and once a song, and the GPU paint it half the window across,
// moving, thirty times a second while the music plays.

// waterSource paints it: noise of four octaves, warped by noise of its own
// so that the washes bleed rather than slide, and at each wash's rim the
// ink gathers darker, as watercolor does when it dries.
const waterSource = `package main

var Time float
var Bass float
var Paper vec4
var W1 vec4
var W2 vec4
var W3 vec4

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
	d := hash(i + vec2(1.0, 1.0))
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}

func fbm(p vec2) float {
	v := 0.0
	amp := 0.5
	for i := 0; i < 4; i++ {
		v = v + amp*noise(p)
		p = p*2.13 + vec2(1.7, 9.2)
		amp = amp * 0.5
	}
	return v
}

// wash is one ink over the paper: where its noise is thick it lies on the
// paper, and at its rim it gathers darker, as watercolor does when it
// dries. Time works on it two ways: two drifts of noise carry the pigment
// along, each going its own way so that the wash spreads instead of
// sliding, and the wet front creeps out and back as the paper wets and
// dries, taking the rim with it.
func wash(c vec3, ink vec3, q vec2, t float, thick float, at vec2, seed float) vec3 {
	wx := fbm(q*0.62 + at + vec2(seed, 0.0) + vec2(t*0.072, -t*0.020))
	wy := fbm(q*0.62 + at + vec2(4.1, 7.3) + vec2(0.0, seed) + vec2(-t*0.026, t*0.061))
	f := fbm(q + 1.20*vec2(wx, wy))
	// The front: the wash runs a shade wider, draws back, and runs again.
	edge := 0.42 + 0.030*sin(t*0.13+seed*2.7) + 0.014*sin(t*0.31+seed)
	s := smoothstep(edge, edge+0.46, f) * thick
	// The rim: where the water stood, the ink is a little darker, and
	// just inside it a little lighter, where it ran.
	rim := exp(-pow((f-edge)/0.055, 2.0))
	inside := smoothstep(edge, edge+0.14, f) * (1.0 - smoothstep(edge+0.14, edge+0.36, f))
	out := mix(c, ink, s)
	out = out * (1.0 - rim*0.24) + ink*inside*0.07
	// The paper keeps its grain through it all.
	g := hash(floor(q*38.0))*0.5 + hash(floor(q*11.0))*0.5
	return out + vec3((g-0.5)*0.018)
}

// blot is a drop landing in the wet paper: the water pushes the pigment
// outward in a ring, its front catching the light and the ink gathered
// just past it, the paper inside lifted a little paler where the pigment
// went; the ring widens and thins away, and another comes.
func blot(c vec3, ink vec3, q vec2, t float, at vec2, period float, seed float, strength float) vec3 {
	ph := fract(t/period + seed)
	rad := 0.09 + 0.70*ph
	// It arrives quickly and is gone slowly.
	env := smoothstep(0.0, 0.08, ph) * (1.0 - ph) * (1.0 - ph)
	d := length(q - at)
	front := exp(-pow((d-rad)/0.030, 2.0))
	edge := exp(-pow((d-rad-0.065)/0.070, 2.0))
	c = c + vec3(0.17, 0.18, 0.19)*front*env*strength
	c = mix(c, ink, 0.26*edge*env*strength)
	inside := 1.0 - smoothstep(rad-0.26, rad-0.03, d)
	c = c * (1.0 + 0.16*inside*env*strength)
	return c
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	p := pos / size.y * 1.35
	t := Time

	c := Paper.rgb
	c = wash(c, W1.rgb, p, t, 0.85+0.10*Bass, vec2(0.0, 0.0), 0.0)
	c = wash(c, W2.rgb, p*1.15+vec2(2.7, 1.9), t*0.9, 0.62+0.08*Bass, vec2(3.3, 6.1), 2.1)
	c = wash(c, W3.rgb, p*1.45+vec2(7.1, 4.4), t*0.8, 0.42, vec2(9.2, 2.7), 4.4)

	// Drops, each its own spot and its own pace, so that one is landing
	// every few seconds, never two at once.
	c = blot(c, W1.rgb, p, t, vec2(0.62, 0.78), 13.0, 0.0, 0.9+0.3*Bass)
	c = blot(c, W3.rgb, p, t, vec2(1.85, 0.52), 17.0, 0.41, 0.7+0.3*Bass)
	c = blot(c, W2.rgb, p, t, vec2(1.24, 1.22), 23.0, 0.73, 0.8)

	// A little light coming through the paper from above.
	c = c + W3.rgb*0.04*smoothstep(0.0, 0.9, 1.0-uv.y)
	// Darker toward the corners, so that the song reads over it.
	vg := length((uv - vec2(0.5, 0.46)) * vec2(1.0, 1.18))
	c = c * (1.06 - 0.40*smoothstep(0.24, 0.95, vg))
	return vec4(c, 1)
}
`

var waterShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(waterSource) })

// WaterInks returns the paper and the three inks of a cover of tint: the
// paper near black, and each ink its own hue a step from the cover's, so
// that they argue a little where they meet.
func WaterInks(tint ui.Color) (paper, w1, w2, w3 ui.Color) {
	h, s, _ := HSV(float64(tint.R)/255, float64(tint.G)/255, float64(tint.B)/255)
	if s > 0.12 {
		s = min(max(s*1.3, 0.55), 0.9)
	}
	return HSVColor(h, s*0.3, 0.052),
		HSVColor(h, s*0.8, 0.46),
		HSVColor(float64(WrapHue(float32(h)+36)), s*0.72, 0.36),
		HSVColor(float64(WrapHue(float32(h)-28)), s*0.85, 0.62)
}

// DrawWatercolor paints the washes of tint in r at drift seconds: wet, and
// running, opening wider and dropping faster as bass does.
func DrawWatercolor(p *ui.Painter, r ui.Rect, tint ui.Color, drift, bass float32) {
	paper, w1, w2, w3 := WaterInks(tint)
	waterShader().Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time": drift, "Bass": min(bass, 1),
		"Paper": paper, "W1": w1, "W2": w2, "W3": w3,
	}})
}
