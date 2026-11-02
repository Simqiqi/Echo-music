package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The vinyl view of Now Playing's cover, after the record players of old:
// the cover becomes the label of a record spinning on a platter, at
// thirty-three and a third, while the music plays, and freezes, mid-spin,
// when it stops, as a paused record does. The drag and the fling of the
// cover stay, for changing songs, and a click on the vinyl button turns
// the record back into the square cover it was.

// vinylRev is how long the record takes to go around once: far slower
// than a true thirty-three, whose spin reads frantic at a label this
// small, and stately instead.
const vinylRev = 8000 * time.Millisecond

// vinylDisc fills what would be the cover with the record on its platter.
func (a *app) vinylDisc(c *ui.Context, art *artwork, size float32) {
	pl := a.pl
	// The record breathes as the cover does: smaller while paused.
	scale := float32(1)
	if !pl.Playing() {
		scale = 0.88
	}
	root := ui.Box(c.Key("vinyl")).Fill()
	root.AnimatePose(ui.Pose{Scale: scale}, ui.Animation{Spring: ui.SpringOptions{Duration: 600 * time.Millisecond, Bounce: 0.25}})
	root.Children(func() {
		d := size
		platter := ui.Box(c.Key("platter")).Size(d, d).Radius(d/2).Background(ui.Hex("#101013")).
			Shadow(0, 30, 60, -12, ui.RGBA(0, 0, 0, 0.55))
		platter.Children(func() {
			// A rim of light on the platter's edge; it stays put while the
			// record turns under it.
			rim := d - 3
			ui.Box(c.Key("rim")).Size(rim, rim).Radius(rim/2).Border(1, white.Alpha(0.08)).
				Absolute().Top(1.5).Left(1.5)

			// The record spins about its middle, its grooves and its label
			// with it.
			disc := ui.Box(c.Key("record")).Absolute().Top(0).Left(0).Size(d, d)
			if pl.Playing() && !c.Preferences().ReduceMotion {
				disc.TransformOrigin(0.5, 0.5).Rotate(disc.Loop("spin", vinylRev, ui.Linear) * 360)
			}
			disc.Children(func() {
				// Grooves: faint rings from the rim in to the label.
				for i, k := range []float32{0.475, 0.45, 0.425, 0.40, 0.375, 0.35, 0.325, 0.30, 0.275} {
					gd := d * k * 2
					ui.Box(c.Key(i)).Size(gd, gd).Radius(gd/2).Border(1, white.Alpha(0.045)).
						Absolute().Top((d - gd) / 2).Left((d - gd) / 2)
				}
				// The label: the cover itself, clipped round, as a record's
				// label is printed on — wider than a label truly runs, so
				// that the art shows.
				ld := d * 0.52
				label := ui.Box(c.Key("label")).Size(ld, ld).Radius(ld / 2).Background(white.Alpha(0.08)).
					Absolute().Top((d - ld) / 2).Left((d - ld) / 2)
				bmp := bitmap(art)
				in := artIn(label, bmp)
				if in < 1 {
					label.Background(ui.RGB(90, 90, 96))
				}
				label.Children(func() { artImage(c, bmp, ld/2, in) })
				// The spindle's hole, over the label's middle.
				hd := max(d*0.032, 6)
				ui.Box(c.Key("spindle")).Size(hd, hd).Radius(hd / 2).Background(ui.Hex("#0b0b0d")).
					Absolute().Top((d - hd) / 2).Left((d - hd) / 2)
			})
		})

		// The tonearm, hung from a pivot past the platter's upper right:
		// while the music plays its head rests on the record, and while
		// the record waits it swings up and out of the way. It moves on a
		// spring, as a real arm does when a hand sets it down.
		arm := ui.Box(c.Key("tonearm")).Size(d*0.62, d*0.62).
			Absolute().Top(d * 0.01).Left(d * 0.44)
		rest, set := float32(-26), float32(2)
		pose := ui.Pose{Rotate: rest}
		if pl.Playing() {
			pose.Rotate = set
		}
		arm.AnimatePose(pose, ui.Animation{Spring: ui.SpringOptions{Duration: 700 * time.Millisecond, Bounce: 0.12}})
		arm.Children(func() {
			// The arm itself, a bar turning about the pivot at its right
			// end, and the head that holds the needle at its left.
			bar := ui.Box(c.Key("arm-bar")).Absolute().Top(d*0.075).Right(d*0.045).
				Size(d*0.5, d*0.024).TransformOrigin(1, 0.5).Rotate(-33)
			bar.Children(func() {
				ui.Box(c).Fill().Radius(d*0.012).Background(ui.RGB(196, 198, 206)).
					Shadow(0, 2, 6, -1, ui.RGBA(0, 0, 0, 0.5))
				// The head, at the arm's far end, tipped toward the record.
				head := d * 0.055
				ui.Box(c.Key("head")).Size(head, head*0.8).Radius(head * 0.2).
					Background(ui.RGB(72, 72, 80)).Absolute().Left(-head * 0.7).Top(-head * 0.24).Rotate(8)
			})
			// The pivot it all turns about.
			hub := d * 0.085
			ui.Box(c.Key("pivot")).Size(hub, hub).Radius(hub/2).
				Background(ui.RGB(96, 96, 104)).Border(1.5, ui.RGB(150, 150, 158)).
				Absolute().Top(d*0.062 - hub/2 + d*0.012).Right(d * 0.03)
			hubDot := hub * 0.38
			ui.Box(c.Key("pivot-dot")).Size(hubDot, hubDot).Radius(hubDot / 2).
				Background(ui.RGB(40, 40, 46)).
				Absolute().Top(d*0.062 - hubDot/2 + d*0.012).Right(d*0.03 + (hub-hubDot)/2)
		})
	})
}

// vinylButton turns the vinyl view on and off; on, its icon lights in the
// song's accent.
func (a *app) vinylButton(c *ui.Context, accent ui.Color) {
	color := white.Alpha(0.7)
	if a.settings.Vinyl {
		color = accent
	}
	b := iconButton(c, iconVinyl, 22, color, "黑胶模式")
	if b.Clicked() {
		a.settings.Vinyl = !a.settings.Vinyl
		a.settings.save()
	}
}
