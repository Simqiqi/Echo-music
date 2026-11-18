package app

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// The picker's little picture of the collage is the same wall the shader
// paints at Time zero, drawn in Go so its hairlines still read at 26 DIP.

func collageHash(x, y float64) float64 {
	v := math.Sin(x*127.1+y*311.7) * 43758.5453
	return v - math.Floor(v)
}

func collageHash2(x, y float64) (float64, float64) {
	return collageHash(x, y), collageHash(x+17.3, y+9.1)
}

func collageNoise(x, y float64) float64 {
	ix, iy := math.Floor(x), math.Floor(y)
	fx, fy := x-ix, y-iy
	ux, uy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a := collageHash(ix, iy)
	b := collageHash(ix+1, iy)
	c := collageHash(ix, iy+1)
	d := collageHash(ix+1, iy+1)
	return mixf(mixf(a, b, ux), mixf(c, d, ux), uy)
}

func collageTorn(qx, qy, hx, hy, seed, t float64) float64 {
	rip := (collageNoise(qy*9.0+seed*3.1, seed*1.7+t*0.07) - 0.5) * 0.17
	jag := (collageHash(math.Floor(qy*130.0)+seed*17.0, seed*2.3) - 0.5) * 0.022
	fibre := (collageNoise(qy*62.0+seed*9.3, seed*4.9+t*0.32) - 0.5) * 0.030
	edge := rip + jag + fibre
	d := math.Max(math.Abs(qx)-(hx+edge), math.Abs(qy)-(hy-edge*0.60))
	return 1 - sstep(-0.004, 0.006, d)
}

func collagePrinted(u, v, seed, t float64) float64 {
	n := collageNoise(u*29.0+seed*7.3, v*31.0+seed*3.1+t*0.012)
	coarse := collageNoise(u*6.0+seed*2.7, v*7.0+seed*5.9)
	return 0.58 + 0.34*n + 0.34*coarse
}

func collageGrey(shade float64) ui.Color {
	v := uint8(clampF(shade*255, 0, 255))
	return ui.RGB(v, uint8(clampF(shade*250, 0, 255)), uint8(clampF(shade*251, 0, 255)))
}

func swatchCollage(s *sheet, tint ui.Color) {
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			n := collageNoise(u*5.0, v*5.0)
			bend := collageNoise(u*3.0, v*3.0)
			shade := 0.22 * (0.74 + 0.46*n) * (0.98 + 0.02*math.Sin(v*float64(s.h)*0.45+bend*6.0))
			s.set(x, y, collageGrey(shade))
		}
	}

	for i := 0; i < 10; i++ {
		fi := float64(i)
		hx1, hy1 := collageHash2(fi*13.1+2.7, fi*7.9+5.3)
		hx2, hy2 := collageHash2(fi*3.7+9.1, fi*11.3+1.7)
		cx := 0.08 + 0.84*hx1 + 0.014*math.Sin(fi*2.1)
		cy := 0.06 + 0.88*hy1 + 0.011*math.Cos(fi*1.7)
		ang := (hx2-0.5)*1.6 + 0.035*math.Sin(fi*1.3)
		ca, sa := math.Cos(ang), math.Sin(ang)
		base := 0.025 + 0.060*hy1
		if fi > 6.5 {
			base = 0.30 + 0.16*hx1
		}
		for y := 0; y < s.h; y++ {
			v := (float64(y) + 0.5) / float64(s.h)
			for x := 0; x < s.w; x++ {
				u := (float64(x) + 0.5) / float64(s.w)
				dx, dy := u-cx, v-cy
				qx, qy := dx*ca-dy*sa, dx*sa+dy*ca
				m := collageTorn(qx, qy, 0.14+0.30*hy2, 0.13+0.30*hx1, fi*3.7+1.0, 0)
				if m <= 0 {
					continue
				}
				s.over(x, y, collageGrey(base*collagePrinted(u, v, fi, 0)), m*0.88)
			}
		}
	}

	tape := ui.RGB(230, 222, 196)
	for i := 0; i < 3; i++ {
		fi := float64(i)
		hx1, hy1 := collageHash2(fi*6.3+4.1, fi*9.7+2.9)
		cx := 0.74 - 0.28*fi + 0.03*(hx1-0.5) + 0.008*math.Sin(fi*2.3)
		cy := 0.16 + 0.32*fi + 0.03*(hy1-0.5) + 0.007*math.Cos(fi*1.1)
		ang := -0.62 + 0.42*fi + 0.05*(hx1-0.5) + 0.030*math.Sin(fi)
		ca, sa := math.Cos(ang), math.Sin(ang)
		for y := 0; y < s.h; y++ {
			v := (float64(y) + 0.5) / float64(s.h)
			for x := 0; x < s.w; x++ {
				u := (float64(x) + 0.5) / float64(s.w)
				dx, dy := u-cx, v-cy
				qx, qy := dx*ca-dy*sa, dx*sa+dy*ca
				m := collageTorn(qx, qy, 0.060+0.025*hy1, 0.024, fi*7.1+3.0, 0)
				if m <= 0 {
					continue
				}
				k := collagePrinted(u, v, fi+20.0, 0)
				s.over(x, y, ui.RGB(uint8(clampF(float64(tape.R)*k, 0, 255)),
					uint8(clampF(float64(tape.G)*k, 0, 255)),
					uint8(clampF(float64(tape.B)*k, 0, 255))), m*0.90)
			}
		}
	}

	for _, seed := range []float64{0.13, 0.37, 0.62, 0.86} {
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			span := sstep(0.04+0.18*seed, 0.10+0.18*seed, u) *
				(1 - sstep(0.44+0.20*seed, 0.56+0.20*seed, u))
			ly := 0.14 + 0.70*seed +
				0.060*math.Sin(u*(7.0+seed*9.0)+seed*11.0) +
				0.022*math.Sin(u*(29.0+seed*17.0)-seed*5.0)
			for y := 0; y < s.h; y++ {
				v := (float64(y) + 0.5) / float64(s.h)
				d := math.Abs(v - ly)
				s.light(x, y, white, math.Exp(-d*d/0.00009)*span*0.24)
			}
		}
	}
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			h, _ := collageHash2(float64(x)*0.31, float64(y)*0.31)
			if h > 0.86 {
				s.light(x, y, white, 0.28)
			}
			gn := collageNoise(float64(x)*0.75, float64(y)*0.75) - 0.5
			gn += (collageNoise(float64(x)*2.3, float64(y)*2.3) - 0.5) * 0.5
			s.light(x, y, white, gn*0.13)
		}
	}

	sung := ui.RGB(uint8(float64(tint.R)*0.42), uint8(float64(tint.G)*0.42), uint8(float64(tint.B)*0.42))
	for y := 0; y < s.h; y++ {
		v := (float64(y) + 0.5) / float64(s.h)
		for x := 0; x < s.w; x++ {
			u := (float64(x) + 0.5) / float64(s.w)
			s.over(x, y, sung, 0.055)
			g := math.Hypot((u-0.5)*1.0, (v-0.46)*1.18)
			k := 1 - 0.34*sstep(0.24, 0.86, g)
			mid := math.Exp(-(sq(u-0.5)+sq(v-0.48))*3.4)
			k *= 1 - 0.18*mid
			i := (y*s.w + x) * 3
			s.px[i], s.px[i+1], s.px[i+2] = s.px[i]*float32(k), s.px[i+1]*float32(k), s.px[i+2]*float32(k)
		}
	}
}
