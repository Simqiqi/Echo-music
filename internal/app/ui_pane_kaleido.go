package app

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The kaleidoscope folds the cover into six mirrored petals that turn
// slowly, open a little on the beat, and breathe with the bass. The
// words of the song sit over its middle, where the light is quietest.

const kaleidoSource = glslCommon + `
var Time float
var Bass float
var Pulse float
var Swirl float
var Has float
var Tint vec4
var Cen vec2
var Ext float
var Pointer vec2

func Fragment(dst vec4, pos vec2) vec4 {
	q := (pos - Cen) / Ext
	t := Time

	// A slow turn, with a slight opening and closing on the beat.
	a := t*0.075 + 0.16*sin(t*0.45) + 0.10*Bass
	cs := cos(a)
	sn := sin(a)
	p := vec2(q.x*cs - q.y*sn, q.x*sn + q.y*cs)
	p = abs(p)
	// Fold once more at thirty degrees: six petals rather than four.
	c2 := cos(0.5236)
	s2 := sin(0.5236)
	p = vec2(p.x*c2 - p.y*s2, p.x*s2 + p.y*c2)
	p = abs(p)

	// A single octave of noise bends the fold, so the petals are cloth
	// rather than glass. We use noise, not fbm: the bend is low frequency
	// and fbm's three extra octaves only cost the frame its smoothness —
	// the particle record samples the cover too and runs fine without them.
	w := vec2(noise(p*2.0 + vec2(t*0.05, 0.0)),
		noise(p*2.0 + vec2(3.1, 7.7) - vec2(0.0, t*0.04)))
	p = p + (w - vec2(0.5, 0.5))*0.22
	cuv := clamp(p*1.15 + vec2(0.06, 0.06), 0.0, 1.0)
	cover := image0Sample(cuv * image0Size()).rgb
	cover = mix(Tint.rgb*0.9, cover, Has)

	r := length(q)
	petal := 0.5 + 0.5*sin(r*17.0 - t*1.3 + noise(p*3.0)*5.0)
	col := cover * (0.55 + 0.65*petal) * (0.82 + 0.35*Bass)

	// The seams where the mirrors meet, and a little light at the heart.
	seam := 1.0 - smoothstep(0.0, 0.035, abs(p.x - p.y))
	col = col * (1.0 - 0.30*seam)
	col = col + mix(vec3(1.0, 1.0, 1.0), Tint.rgb, 0.4) * exp(-r*r*9.0) * (0.18 + 0.22*Pulse)

	// A dark rim, and the pointer's light where it stirs the glass.
	col = col * (1.0 - 0.62*smoothstep(0.55, 1.35, r))
	pd := (pos - Pointer) / Ext
	col = col + Tint.rgb*exp(-dot(pd, pd)*9.0)*Swirl*0.35
	return vec4(col, 1)
}
`

var kaleidoShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(kaleidoSource) })

func drawKaleido(p *ui.Painter, r ui.Rect, f visualFrame) {
	has := float32(0)
	if f.cover != nil {
		has = 1
	}
	kaleidoShader().Draw(p, r, shader.Options{
		Uniforms: map[string]any{
			"Time": f.t, "Bass": f.bass, "Pulse": f.pulse, "Swirl": f.swirl,
			"Has": has, "Tint": f.tint, "Cen": f.cen, "Ext": f.ext,
			"Pointer": f.pointer,
		},
		Images: [4]*ui.Bitmap{f.cover},
	})
}
