package fx

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The ripples are Now Playing's background as black water at night, with
// the cover darkly under it: the colors of the cover, blurred, seen
// through a slow swell that never stops, and now and then a ring
// widening from where a drop fell, its crests catching the light as it
// passes. The water bends what is under it, so the cover moves as the
// rings go over it, rather than the rings lying on the cover.
//
// Like the fog and the stars, it is a shader, so that the CPU may paint
// it small and once a song, and the GPU paint it half the window across,
// moving, thirty times a second while the music plays.

// rippleSource paints it: the cover, blurred, read through the water's
// bend, darkened by the depth of the water and lit along the crests of
// the rings.
const rippleSource = `package main

var Time float
var Bass float
var Deep vec4
var Glow vec4

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	asp := size.x / size.y
	// The cover is square: it fills the window, cropped to its shape.
	u0 := uv
	if asp > 1.0 {
		u0.y = 0.5 + (uv.y-0.5)/asp
	} else {
		u0.x = 0.5 + (uv.x-0.5)*asp
	}
	p := vec2(uv.x*asp, uv.y)
	t := Time

	// The swell of the water, running slowly across it, and a shorter
	// chop within that.
	w := vec2(0.0, 0.0)
	w.y = 0.0038*sin(p.y*6.5-t*0.62) + 0.0018*sin(p.y*13.0+p.x*1.9-t*0.43)
	w.x = 0.0024*sin(p.y*4.6+p.x*0.7+t*0.30) + 0.0011*sin(p.y*10.0-t*0.55)

	// Rings, each from a place of its own, widening and fading as they
	// go, the bass striking them a little harder.
	lit := 0.0
	for i := 0; i < 3; i++ {
		fi := float(i)
		src := vec2(0.10+0.80*hash(vec2(fi*3.1, 1.7)), 0.10+0.62*hash(vec2(fi*5.3, 8.2)))
		T := 6.0 + 4.5*hash(vec2(fi*2.7, 4.4))
		ph := fract(t/T + hash(vec2(fi, 9.1)))
		dv := p - src
		d := length(dv)
		e := exp(-abs(d-ph*1.55)*26.0) * (1.0-ph)*(1.0-ph)*(1.0-ph)
		w = w + (dv/max(d, 0.001))*e*(0.009+0.006*Bass)
		lit = lit + e
	}

	// The cover read four times, a pixel apart, so that the water's
	// bend runs smooth rather than in steps.
	q := (u0 + w) * image0Size()
	col := image0Sample(q + vec2(0.7, 0.7)).rgb*0.25 + image0Sample(q + vec2(-0.7, 0.7)).rgb*0.25 +
		image0Sample(q + vec2(0.7, -0.7)).rgb*0.25 + image0Sample(q + vec2(-0.7, -0.7)).rgb*0.25
	// Water has depth: the cover is darker under it, and the shade of the
	// deep is in the shadows.
	col = col*0.72 + Deep.rgb*0.08
	// The light of the rings, lying on the water, and the sheen where the
	// swell turns over.
	lg := mix(vec3(0.82, 0.92, 1.0), Glow.rgb, 0.30)
	col = col + lg*min(lit, 1.2)*(0.22+0.12*Bass)
	sh := max(0.0, sin(p.y*6.5-t*0.62+1.6))
	col = col + lg*sh*sh*sh*0.05

	// Darker toward the corners, so that the song reads over it.
	vg := length((uv - vec2(0.5, 0.5)) * vec2(1.05, 1.2))
	col = col * (1.10 - 0.42*smoothstep(0.22, 1.0, vg))
	return vec4(col, 1)
}
`

var rippleShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(rippleSource) })

// DrawRipple paints the water of r at drift seconds over art, the cover
// blurred, and lit by bass.
func DrawRipple(p *ui.Painter, r ui.Rect, art *ui.Bitmap, tint ui.Color, drift, bass float32) {
	deep, glow := NightColors(tint)
	rippleShader().Draw(p, r, shader.Options{
		Uniforms: map[string]any{"Time": drift, "Bass": min(bass, 1), "Deep": deep, "Glow": glow},
		Images:   [4]*ui.Bitmap{art},
	})
}
