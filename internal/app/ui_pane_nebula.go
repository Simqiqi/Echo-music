package app

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The nebula is the song's own color blown into a slow, turning cloud of
// light, with dust caught in it. The bass swells the cloud, the treble
// wakes the dust, and the pointer stirs both: they close again behind it.

const nebulaSource = glslCommon + `
var Time float
var Bass float
var Treble float
var Pulse float
var Swirl float
var Tint vec4
var Cen vec2
var Ext float
var Pointer vec2

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	q := (pos - Cen) / Ext
	t := Time

	// The whole field turns, slowly, so no two moments are quite alike.
	cs := cos(t*0.09)
	sn := sin(t*0.09)
	p := vec2(q.x*cs - q.y*sn, q.x*sn + q.y*cs)

	// The pointer pushes the cloud aside; it closes again as Swirl dies.
	pd := pos - Pointer
	pl := length(pd) / Ext
	push := Swirl * exp(-pl*pl*7.0)
	p = p + pd/Ext * push * 1.1

	w := vec2(fbm(p*1.6 + vec2(t*0.06, -t*0.05)),
		fbm(p*1.6 + vec2(5.2, 1.3) - vec2(t*0.04, t*0.03)))
	f := fbm(p*2.2 + 1.7*w)
	r := length(p)
	band := 0.5 + 0.5*sin(r*12.0 - t*1.6 + f*7.0 + w.x*4.0)
	cloud := smoothstep(0.26, 0.82, f) * (0.32 + 0.8*band)
	cloud = cloud * exp(-r*r*1.45) * (1.0 + 0.35*Bass + 0.2*Pulse)

	cold := vec3(0.72, 0.84, 1.0)
	col := vec3(0.012, 0.013, 0.022)
	col = col + mix(Tint.rgb, cold, 0.3 + 0.4*f) * cloud * (0.9 + 0.5*Bass)
	col = col + Tint.rgb * exp(-r*r*3.2) * (0.16 + 0.24*Bass)

	// Dust, each speck on a slow orbit of its own and its own twinkle.
	cell := size.y / 44.0
	id := floor((pos - Cen) / cell)
	h := hash2(id + vec2(1.3, 7.7))
	if h.x > 0.42 {
		at := Cen + (id + 0.5 + (h - vec2(0.5, 0.5))*0.8) * cell
		from := at - Cen
		d0 := max(length(from), 1.0)
		drift := vec2(-from.y, from.x) / d0
		at = at + drift * (2.0 + 7.0*sin(t*0.3 + h.y*6.28)) * (0.45 + Bass)
		away := at - Pointer
		at = at + away/max(length(away), 1.0) * push * 36.0
		d := length(pos - at) / max(cell*0.085, 0.8)
		tw := 0.45 + 0.55*sin(t*(1.1 + 2.6*h.y) + h.x*30.0)
		col = col + mix(vec3(1.0, 1.0, 1.0), cold, h.y) * exp(-d*d) * tw * (0.25 + 0.8*Treble + 0.3*Pulse)
	}
	// A dark rim keeps the light from running into the window's edges.
	col = col * (1.0 - 0.42*smoothstep(0.75, 1.6, length(q)))
	return vec4(col, 1)
}
`

var nebulaShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(nebulaSource) })

func drawNebula(p *ui.Painter, r ui.Rect, f visualFrame) {
	nebulaShader().Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time": f.t, "Bass": f.bass, "Treble": f.treble, "Pulse": f.pulse,
		"Swirl": f.swirl, "Tint": f.tint, "Cen": f.cen, "Ext": f.ext,
		"Pointer": f.pointer,
	}})
}
