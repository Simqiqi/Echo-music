package fx

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The record is Now Playing's background as a turntable in a dark room: a
// disc of black vinyl standing out from behind the cover, its grooves
// turning slowly, the light of the room lying across it in two soft
// bands, and a little dust caught in the grooves, going round with it.
//
// Like the fog and the stars, it is a shader, so that the CPU may paint it
// small and once a song, and the GPU paint it half the window across,
// moving, thirty times a second while the music plays.

// vinylSource paints it: the dark of the room, the disc, and the light
// over both. It turns with Time; its grooves are near enough rings, and
// the dust in them and the wobble of their spacing are what turn. Scale is
// how many of the canvas's pixels a window's DIP is, for the grooves to
// stay the same width whatever the canvas is painted at — and where the
// canvas is too coarse to hold them, the grooves and the dust give way to
// a plain disc.
const vinylSource = `package main

var Time float
var Deep vec4
var Glow vec4
var Center vec2
var Radius float
var Scale float

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	c := Deep.rgb*0.10 + vec3(0.008, 0.008, 0.010)
	// The room's light gathers behind the record.
	q := pos - Center
	d := length(q)
	c = c + Glow.rgb*0.05*exp(-d*d/(Radius*Radius*2.2))
	c = c + Deep.rgb*0.30*exp(-length((uv-vec2(0.5, 0.45))*vec2(1.0, 1.3))*3.0)

	if Radius > 1.0 && d < Radius {
		// The disc, turned: what the grooves carry goes round with it.
		rot := Time * 0.70
		cs := cos(rot)
		sn := sin(rot)
		qr := vec2(q.x*cs+q.y*sn, -q.x*sn+q.y*cs)
		// The grooves, a shade over two DIPs apart, their spacing nudged
		// a little, cell by cell, so that the disc is seen to turn.
		per := 2.3*Scale
		fine := smoothstep(0.7, 1.1, per)
		wob := 2.2*hash(floor(qr/max(16.0*Scale, 2.0)))
		g := sin(d*6.2831/max(per, 0.4)+wob)
		// The bands of the record: a little louder here and there, and
		// the grooves fading where the label sits.
		band := 0.55 + 0.45*sin(d*Scale*0.157+wob*0.3)
		label := smoothstep(Radius*0.16, Radius*0.30, d)
		vinyl := vec3(0.055, 0.055, 0.060) + Deep.rgb*0.12
		vinyl = vinyl * (1.0-0.30*band) + vec3(0.42, 0.44, 0.50)*max(g, 0.0)*0.14*band*label*fine
		// Where the record ends: a hairline of light.
		rim := exp(-pow((d-Radius)/max(1.3*Scale, 1.2), 2.0))
		vinyl = vinyl + vec3(0.62, 0.64, 0.70)*rim*0.30
		// Dust in the grooves, sparse, going round with them.
		cp := 5.0*Scale
		dust := smoothstep(1.8, 2.6, cp)
		if dust > 0.0 {
			cell := floor(qr/max(cp, 1.0))
			h := hash(cell)
			if h > 0.988+0.02*(1.0-dust) {
				at := (vec2(hash(cell+vec2(1.7, 4.3)), hash(cell+vec2(8.2, 2.1)))-0.5)*cp
				dd := length(fract(qr/max(cp, 1.0))*cp-cp/2.0-at)
				vinyl = vinyl + vec3(0.62, 0.60, 0.55)*exp(-dd*dd*0.35)*0.35*dust
			}
		}
		c = mix(c, vinyl, 1.0-smoothstep(Radius-max(1.5*Scale, 0.8), Radius, d))
		// Two soft bands of the room's light across the disc, which stay
		// where they are as it turns.
		l1 := exp(-pow((q.x-Radius*0.34)/(Radius*0.22), 2.0))
		l2 := exp(-pow((q.x+Radius*0.52)/(Radius*0.09), 2.0))
		shade := exp(-pow(q.y/(Radius*1.05), 2.0))
		c = c + vec3(0.62, 0.66, 0.74)*(l1*0.10+l2*0.07)*shade*(1.0-smoothstep(Radius*0.9, Radius, d))
	}

	// Darker toward the corners, as the room is.
	vg := length((uv-vec2(0.5, 0.46))*vec2(1.0, 1.15))
	c = c * (1.04 - 0.40*smoothstep(0.25, 0.95, vg))
	return vec4(c, 1)
}
`

var vinylShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(vinylSource) })

// DrawVinyl paints the record of r at drift seconds, its disc standing
// out from behind cover, a box of the window W across.
func DrawVinyl(p *ui.Painter, r ui.Rect, W float32, cover ui.Rect, tint ui.Color, drift float32) {
	deep, glow := NightColors(tint)
	// How many of the canvas's pixels a DIP of the window is, and the
	// cover's box in them: the disc a little larger than the cover, set
	// off to the right of it, as a record stands behind its sleeve.
	k := r.W / max(W, 1)
	size := cover.W
	cx, cy := (cover.X+size*0.78)*k, (cover.Y+size/2)*k
	radius := size * 0.62 * k
	if cover.W <= 0 {
		// No cover to stand behind: the disc in the middle of the room.
		cx, cy, radius = r.W/2, r.H*0.46, min(r.W, r.H)*0.30
	}
	vinylShader().Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time": drift, "Deep": deep, "Glow": glow,
		"Center": []float32{cx, cy}, "Radius": radius, "Scale": k,
	}})
}
