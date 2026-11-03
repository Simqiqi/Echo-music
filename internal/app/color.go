package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/fx"
)

// The colors the pages mix their shades from, in the vocabulary the app
// has always used: the effects themselves live in internal/fx, where the
// shaders that paint them are.

// hsv is the hue (degrees), saturation and value of r, g and b, 0 to 1.
func hsv(r, g, b float64) (h, s, v float64) { return fx.HSV(r, g, b) }

// hsvColor is the color of hue h (degrees), saturation s and value v.
func hsvColor(h, s, v float64) ui.Color { return fx.HSVColor(h, s, v) }

// wrapHue brings a hue back into the wheel, 0 to 360.
func wrapHue(h float32) float32 { return fx.WrapHue(h) }
