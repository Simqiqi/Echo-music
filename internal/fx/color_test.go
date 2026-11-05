package fx

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// HSV reads a primary's hue, and stays without hue for black, white and gray.
func TestHSV(t *testing.T) {
	for _, tc := range []struct {
		name    string
		r, g, b float64
		h, s, v float64
	}{
		{"black", 0, 0, 0, 0, 0, 0},
		{"white", 1, 1, 1, 0, 0, 1},
		{"gray", 0.5, 0.5, 0.5, 0, 0, 0.5},
		{"red", 1, 0, 0, 0, 1, 1},
		{"yellow", 1, 1, 0, 60, 1, 1},
		{"green", 0, 1, 0, 120, 1, 1},
		{"cyan", 0, 1, 1, 180, 1, 1},
		{"blue", 0, 0, 1, 240, 1, 1},
		{"magenta", 1, 0, 1, 300, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, s, v := HSV(tc.r, tc.g, tc.b)
			if math.Abs(h-tc.h) > 0.001 || math.Abs(s-tc.s) > 0.001 || math.Abs(v-tc.v) > 0.001 {
				t.Errorf("HSV(%v, %v, %v) = (%v, %v, %v), want (%v, %v, %v)",
					tc.r, tc.g, tc.b, h, s, v, tc.h, tc.s, tc.v)
			}
		})
	}
}

// HSVColor turns a hue back into the same color HSV would read from it.
func TestHSVColorRoundTrip(t *testing.T) {
	for _, h := range []float64{0, 30, 60, 120, 200, 300, 350} {
		for _, s := range []float64{0, 0.4, 1} {
			c := HSVColor(h, s, 0.8)
			gotH, gotS, gotV := HSV(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
			if math.Abs(gotV-0.8) > 0.01 || math.Abs(gotS-s) > 0.01 ||
				(s > 0 && math.Abs(gotH-h) > 1) {
				t.Errorf("HSVColor(%v, %v, 0.8) = %+v, read back as (%v, %v, %v)",
					h, s, c, gotH, gotS, gotV)
			}
		}
	}
}

// WrapHue keeps hues on the wheel past either end of it.
func TestWrapHue(t *testing.T) {
	for _, tc := range []struct{ in, want float32 }{
		{0, 0}, {360, 0}, {725, 5}, {-30, 330}, {-725, 355},
	} {
		if got := WrapHue(tc.in); got != tc.want {
			t.Errorf("WrapHue(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// A neutral cover used to turn magenta in the terrain: the green channel was
// divided as uint8 before conversion, so NightColors saw almost no green and
// gave a gray or mixed tint a hue it never had.
func TestNightColorsKeepsCoverTint(t *testing.T) {
	for _, tc := range []struct {
		name       string
		tint       ui.Color
		deep, glow [3]uint8
	}{
		{"gray", ui.RGB(128, 128, 128), [3]uint8{33, 33, 33}, [3]uint8{199, 199, 199}},
		{"warm", ui.RGB(200, 180, 160), [3]uint8{33, 26, 19}, [3]uint8{199, 152, 106}},
		{"teal", ui.RGB(30, 140, 110), [3]uint8{11, 33, 27}, [3]uint8{47, 199, 157}},
		{"green", ui.RGB(0, 255, 0), [3]uint8{11, 33, 11}, [3]uint8{47, 199, 47}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deep, glow := NightColors(tc.tint)
			for _, c := range []struct {
				name string
				got  ui.Color
				want [3]uint8
			}{{"deep", deep, tc.deep}, {"glow", glow, tc.glow}} {
				rgb := [3]uint8{c.got.R, c.got.G, c.got.B}
				if rgb != c.want {
					t.Errorf("%s = %v, want %v", c.name, rgb, c.want)
				}
			}
		})
	}
}
