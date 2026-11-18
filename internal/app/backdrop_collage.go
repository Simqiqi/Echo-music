package app

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The collage backdrop is a wall of torn paper: sheets of near-black and
// grey laid over one another, short strips of masking tape across the
// tears, white scratches and the grain of a worn photocopy over it all.
// Nothing here is still. Each sheet drifts and turns on a clock of its
// own, the fibres along a tear keep working, a scratch writes itself out
// from left to right and starts again, the grain is re-cast twelve times
// a second, and the bass breathes a little through the whole wall.

const collageSource = glslCommon + `
var Time float
var Bass float
var Tint vec4

func turn(p vec2, a float) vec2 {
	c := cos(a)
	s := sin(a)
	return vec2(p.x*c - p.y*s, p.x*s + p.y*c)
}

func torn(q vec2, hx float, hy float, seed float, t float) float {
	rip := (noise(vec2(q.y*9.0 + seed*3.1, seed*1.7 + t*0.07)) - 0.5)*0.17
	jag := (hash(vec2(floor(q.y*130.0) + seed*17.0, seed*2.3)) - 0.5)*0.022
	fibre := (noise(vec2(q.y*62.0 + seed*9.3, seed*4.9 + t*0.32)) - 0.5)*0.030
	edge := rip + jag + fibre
	d := max(abs(q.x) - (hx + edge), abs(q.y) - (hy - edge*0.60))
	return 1.0 - smoothstep(-0.004, 0.006, d)
}

func printed(uv vec2, seed float, t float) float {
	n := noise(uv*vec2(29.0, 31.0) + vec2(seed*7.3, seed*3.1 + t*0.012))
	coarse := noise(uv*vec2(6.0, 7.0) + vec2(seed*2.7, seed*5.9))
	return 0.58 + 0.34*n + 0.34*coarse
}

func scribble(uv vec2, t float, seed float) float {
	y := 0.14 + 0.70*seed
	y = y + 0.060*sin(uv.x*(7.0 + seed*9.0) + seed*11.0)
	y = y + 0.022*sin(uv.x*(29.0 + seed*17.0) - seed*5.0)
	span := smoothstep(0.04 + 0.18*seed, 0.10 + 0.18*seed, uv.x)
	span = span * (1.0 - smoothstep(0.44 + 0.20*seed, 0.56 + 0.20*seed, uv.x))
	head := 0.12 + 0.56*fract(t*0.055 + seed)
	written := 1.0 - smoothstep(head - 0.03, head + 0.03, uv.x)
	d := abs(uv.y - y)
	return exp(-d*d/0.000012)*span*written
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	t := Time
	b := min(Bass, 1.0)

	col := vec3(0.22, 0.22, 0.215) * (0.74 + 0.46*noise(uv*vec2(5.0, 5.0) + vec2(0.0, t*0.006)))
	col = col * (0.98 + 0.02*sin(uv.y*size.y*0.45 + noise(uv*vec2(3.0, 3.0))*6.0))

	for i := 0; i < 10; i++ {
		fi := float(i)
		h := hash2(vec2(fi*13.1 + 2.7, fi*7.9 + 5.3))
		h2 := hash2(vec2(fi*3.7 + 9.1, fi*11.3 + 1.7))
		cx := 0.08 + 0.84*h.x + 0.014*sin(t*(0.09 + 0.06*h.y) + fi*2.1)
		cy := 0.06 + 0.88*h.y + 0.011*cos(t*(0.07 + 0.05*h.x) + fi*1.7)
		ang := (h2.x - 0.5)*1.6 + 0.035*sin(t*0.13 + fi*1.3)
		q := turn(uv - vec2(cx, cy), ang)
		m := torn(q, 0.14 + 0.30*h2.y, 0.13 + 0.30*h.x, fi*3.7 + 1.0, t)
		base := 0.025 + 0.060*h.y
		if fi > 6.5 {
			base = 0.30 + 0.16*h.x
		}
		shade := base*printed(uv, fi, t)
		col = mix(col, vec3(shade), m*(0.88 + 0.12*b))
	}

	for i := 0; i < 3; i++ {
		fi := float(i)
		h := hash2(vec2(fi*6.3 + 4.1, fi*9.7 + 2.9))
		cx := 0.74 - 0.28*fi + 0.03*(h.x - 0.5) + 0.008*sin(t*0.19 + fi*2.3)
		cy := 0.16 + 0.32*fi + 0.03*(h.y - 0.5) + 0.007*cos(t*0.23 + fi*1.1)
		ang := -0.62 + 0.42*fi + 0.05*(h.x - 0.5) + 0.030*sin(t*0.31 + fi)
		q := turn(uv - vec2(cx, cy), ang)
		m := torn(q, 0.060 + 0.025*h.y, 0.024, fi*7.1 + 3.0, t)
		col = mix(col, vec3(0.90, 0.87, 0.77)*printed(uv, fi + 20.0, t), m*0.90)
	}

	scr := scribble(uv, t, 0.13) + scribble(uv, t + 11.0, 0.37)*0.8
	scr = scr + scribble(uv, t + 23.0, 0.62)*0.7 + scribble(uv, t + 31.0, 0.86)*0.6
	col = col + vec3(0.88, 0.89, 0.90)*min(scr, 1.0)*(0.30 + 0.16*b)

	cell := size.y / 96.0
	id := floor(pos/cell)
	sp := hash2(id + vec2(3.1, 7.7))
	if sp.x > 0.84 {
		at := (id + sp)*cell
		d := length(pos - at)/max(cell*0.14, 0.8)
		col = col + vec3(1.0, 1.0, 0.98)*exp(-d*d)*0.34*(0.5 + 0.5*sp.y + 0.3*b)
	}

	flick := floor(t*12.0)
	gn := noise(pos*vec2(0.75, 0.75) + vec2(flick*7.3, flick*3.1)) - 0.5
	gn = gn + (noise(pos*vec2(2.3, 2.3) + vec2(flick*11.7, flick*5.9)) - 0.5)*0.5
	col = col + vec3(gn*0.13)

	col = mix(col, Tint.rgb*0.42, 0.055)

	g := length((uv - vec2(0.5, 0.46))*vec2(1.0, 1.18))
	col = col*(1.0 - 0.34*smoothstep(0.24, 0.86, g))
	mid := exp(-dot(uv - vec2(0.5, 0.48), uv - vec2(0.5, 0.48))*3.4)
	col = col*(1.0 - 0.18*mid)
	return vec4(col, 1)
}
`

var collageShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(collageSource) })

func drawCollage(p *ui.Painter, r ui.Rect, tint ui.Color, drift, bass float32) {
	collageShader().Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time": drift, "Bass": min(bass, 1), "Tint": tint,
	}})
}
