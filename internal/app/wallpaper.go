package app

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/egoist/mygo/ui"
)

// wallpaper is the color the app opens onto: two hues drawn at random
// when it starts, one at each end of a slow diagonal, so that no two
// mornings look alike. It is a wash over the ground rather than the
// ground itself: the light or dark appearance still sets the depth, and
// the cover of what plays still tints it.
type wallpaper struct {
	hue, next float32 // its colors, in degrees around the wheel
	angle     float32 // the way it runs, in degrees clockwise from upwards
	lift      float32 // how far along it the light gathers
}

// newWallpaper draws a wallpaper: a hue, another one near enough to it to
// belong, at an angle that is more often than not a diagonal.
func newWallpaper() wallpaper {
	h := rand.Float32() * 360
	// Around the wheel, either way, by a step wide enough to be seen and
	// narrow enough not to argue.
	step := 26 + rand.Float32()*48
	if rand.IntN(2) == 0 {
		step = -step
	}
	// Down to the right, or down to the left.
	angle := 105 + rand.Float32()*60
	if rand.IntN(2) == 0 {
		angle = 195 + rand.Float32()*60
	}
	return wallpaper{
		hue:   h,
		next:  wrapHue(h + step),
		angle: angle,
		lift:  0.38 + rand.Float32()*0.24,
	}
}

// The wash breathes. Its hues go on around the wheel, slowly enough that
// no one watches them go, and its angle sways either way of the one it
// opened at, so that an evening's color is never quite the one it was an
// hour ago, and no two glances at it are quite alike.
const (
	wallpaperDrift = 0.07                   // degrees a second around the wheel: a turn in an hour and a half
	wallpaperSway  = 8                      // degrees either side of the angle it opened at
	wallpaperTide  = 78                     // seconds the sway takes to go and to come back
	wallpaperGap   = 110                    // seconds the two hues take to draw apart and together
	wallpaperFrame = 220 * time.Millisecond // how often it is painted as it breathes
)

// posed is the wallpaper as it stands t seconds after the app opened: its
// hues gone a little way round the wheel, a little further apart or nearer
// than they opened, and its angle swayed.
func (w wallpaper) posed(t float32) wallpaper {
	d := w
	d.hue = wrapHue(w.hue + t*wallpaperDrift)
	gap := shortestArc(w.hue, w.next) * (1 + 0.18*sinTide(t, wallpaperGap))
	d.next = wrapHue(d.hue + gap)
	d.angle = w.angle + wallpaperSway*sinTide(t, wallpaperTide)
	return d
}

// sinTide is a sine of t with a period of period seconds, from -1 to 1,
// starting at 0.
func sinTide(t, period float32) float32 {
	return float32(math.Sin(2 * math.Pi * float64(t) / float64(period)))
}

// paintWallpaper washes r in the wallpaper, over the ground the page has
// already laid down, as it stands t seconds after the app opened. Where
// the renderer can paint shaders, the wash is a field of the colors rather
// than a line of them, pushed about by a slow current (flow.go); where it
// cannot, the gradient stands in for it. Its colors are held at one
// opacity, so that what shows through is the ground's, animated and tinted
// as it is.
func (w wallpaper) paint(p *ui.Painter, r ui.Rect, dark bool, t float32) {
	if p.Accelerated() {
		drawFlow(p, r, w, dark, t)
		return
	}
	d := w.posed(t)
	p.FillGradient(r, ui.LinearGradient{Angle: d.angle, Oklab: true, Stops: d.stops(dark)}, 0)
}

// stops are the wallpaper's colors along its line: deep at both ends, and
// the light gathering in between, where the two hues meet.
func (w wallpaper) stops(dark bool) []ui.GradientStop {
	mid := wrapHue(w.hue + shortestArc(w.hue, w.next)/2)
	if dark {
		// Deep, and deeper toward the far end, as a night room lit from
		// one side: the colors are the room's, not the lamp's.
		return []ui.GradientStop{
			{Color: ui.Oklch(0.34, 0.11, w.hue).Alpha(0.75), At: 0},
			{Color: ui.Oklch(0.52, 0.12, mid).Alpha(0.75), At: w.lift},
			{Color: ui.Oklch(0.26, 0.09, w.next).Alpha(0.75), At: 1},
		}
	}
	// The same wash in daylight, plainly there to see: a pastel that runs
	// from one hue to the other through the light.
	return []ui.GradientStop{
		{Color: ui.Oklch(0.92, 0.065, w.hue).Alpha(0.88), At: 0},
		{Color: ui.Oklch(0.86, 0.090, mid).Alpha(0.88), At: w.lift},
		{Color: ui.Oklch(0.90, 0.055, w.next).Alpha(0.88), At: 1},
	}
}

// shortestArc is the angle from a to b the short way around the wheel,
// negative the other way.
func shortestArc(a, b float32) float32 {
	d := float32(math.Mod(float64(b-a), 360))
	if d > 180 {
		d -= 360
	}
	if d < -180 {
		d += 360
	}
	return d
}

// ground lays the page's ground: the color of the moment, fading from one
// song's tint to the next, under the wallpaper drawn at the start. The
// wallpaper is painted again and again, a handful of times a second, for
// it goes on drifting all the while the app is open.
func (a *app) ground(el ui.Element, bg ui.Color) {
	el.Background(el.AnimateColor("bg", bg, ui.Animation{Duration: 900 * time.Millisecond}))
	el.Draw(func(p *ui.Painter, r ui.Rect) {
		a.wash.paint(p, r, a.pal.dark, a.wallTime(p.Now()))
		p.After(wallpaperFrame)
	})
}

// wallTime is how many seconds the app has been open at now: the time the
// wallpaper is drawn at, from the moment it first was.
func (a *app) wallTime(now time.Time) float32 {
	if a.opened.IsZero() {
		a.opened = now
	}
	return float32(now.Sub(a.opened).Seconds())
}
