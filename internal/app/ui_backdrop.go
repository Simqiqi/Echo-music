package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// backdropPicker is Now Playing's own choice of backdrop: a quiet pill
// naming the one it wears, which opens onto all of them, each drawn small
// in the colors of the song playing. It sits at the top right of the view,
// out of the way of the song, where it is there to be found when the
// backdrop is what the listener is looking at.
func (a *app) backdropPicker(c *ui.Context, tint ui.Color, key string) ui.Element {
	style := a.settings.backdropStyle()
	chosen := style
	sel := ui.SelectBase(c.Key(key), &chosen)
	// As tall as the pane tabs, and their weight, so that the two read
	// as buttons of one kind. A little picture of the backdrop it wears
	// sits before its name, so the pill reads as the choice it is.
	tr := sel.Trigger.Height(npTabsHeight).Padding(5, 10, 5, 11).Gap(7).Radius(999).AlignItems(ui.Center).Label("播放页背景")
	fill := white.Alpha(0.10)
	if tr.Hovered() || sel.Open() {
		fill = white.Alpha(0.20)
	}
	tr.Background(tr.AnimateColor("bg", fill, ui.Animation{Duration: 140 * time.Millisecond})).
		Border(1, white.Alpha(0.14)).
		Shadow(0, 4, 16, -4, white.Alpha(0.10))
	chevron := ui.Pose{}
	if sel.Open() {
		chevron.Rotate = 180
	}
	pic := a.swatch(style, tint)
	tr.Children(func() {
		// The backdrop as it shows in the picker: a small, rounded cut of
		// it, with a hairline so a dark one still has an edge.
		ui.Box(c).Size(16, 16).Radius(5).Clip().Draw(func(p *ui.Painter, r ui.Rect) {
			p.Image(pic, r, ui.FillBox)
			p.Stroke(ui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1}, white.Alpha(0.35), 6.5, 1)
		})
		ui.Text(c, backdropLabel(style)).FontSize(13).FontWeight(600).TextColor(white.Alpha(0.9)).SingleLine()
		ui.Icon(c, iconDown).Size(11, 11).TextColor(white.Alpha(0.6)).AnimatePose(chevron, ui.Snappy)
	})
	sel.Popup(func(panel ui.Element) {
		panel.MinWidth(172).Padding(6).Radius(14).Background(ui.RGBA(24, 24, 28, 0.96)).
			Border(1, white.Alpha(0.12)).Shadow(0, 18, 44, -10, ui.RGBA(0, 0, 0, 0.45)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.97, Y: -4}, Exit: &ui.Motion{Opacity: 0}})
		for _, b := range backdrops {
			choice := b
			picked := choice.id == style
			it := sel.Item(choice.id).Padding(4, 7).Radius(9).Gap(10).AlignItems(ui.Center)
			if it.Highlighted() {
				it.Background(white.Alpha(0.09))
			} else if picked {
				it.Background(white.Alpha(0.05))
			}
			// The picture each backdrop makes, painted small: the same
			// colors the song gives it, at a weight the eye reads in
			// twenty-six DIPs.
			pic := a.swatch(choice.id, tint)
			it.Children(func() {
				ui.Box(c).Size(swatchSide, swatchSide).Radius(7).Clip().
					Draw(func(p *ui.Painter, r ui.Rect) {
						p.Image(pic, r, ui.FillBox)
						// A hairline about it, so that a dark swatch still
						// has an edge on the panel, and the one worn is
						// ringed the brighter.
						edge := float32(0.15)
						if picked {
							edge = 0.38
						}
						p.Stroke(ui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1}, white.Alpha(edge), 6.5, 1)
					})
				label := ui.Text(c, choice.label).FontSize(13).Grow(1).TextColor(white.Alpha(0.92)).SingleLine()
				if picked {
					label.FontWeight(600)
					ui.Icon(c, iconCheck).Size(11, 11).TextColor(white.Alpha(0.9))
				}
			})
		}
	})
	if chosen != style {
		// Only the backdrop changed: nothing to apply of the appearance,
		// so it is kept and painted again.
		a.settings.setBackdrop(chosen)
		a.settings.save()
		a.redraw()
	}
	return sel.Trigger
}

// backdropLabel returns what the settings call a backdrop by its id.
func backdropLabel(style string) string {
	for _, b := range backdrops {
		if b.id == style {
			return b.label
		}
	}
	return backdrops[0].label
}

// swatchSide is how big a swatch shows: the little picture of a backdrop
// beside its name, a shade larger than the icon of a list, so that the
// hills and the rings in it have room to read.
const swatchSide = 26

// swatchCache is a swatch as it was last painted, and the tint it was
// painted in.
type swatchCache struct {
	pic  *ui.Bitmap
	tint ui.Color
}

// swatch returns the little picture of style in the colors of a cover of
// tint, painted once and kept: the picker opens again on the same song and
// it is already there, and it is painted anew only when the song changes
// the colors it is made of.
func (a *app) swatch(style string, tint ui.Color) *ui.Bitmap {
	if a.swatches == nil {
		a.swatches = make(map[string]*swatchCache, len(backdrops))
	}
	sw := a.swatches[style]
	if sw == nil {
		sw = &swatchCache{}
		a.swatches[style] = sw
	}
	if sw.pic != nil && sw.tint.R == tint.R && sw.tint.G == tint.G && sw.tint.B == tint.B {
		return sw.pic
	}
	sw.pic, sw.tint = ui.NewBitmap(paintSwatch(style, tint)), tint
	return sw.pic
}
