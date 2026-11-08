package app

import (
	"math"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
)

const (
	// heroWaveW is how much of the lead record the wave and the discs
	// take at its right; the words keep clear of it.
	heroWaveW = 340
	heroWaveH = 148
)

// albumFace is an album as the wave wears it: its cover, and its name.
type albumFace struct {
	name  string
	cover string
}

// heroWaveRect is where the wave stands on the lead record.
func heroWaveRect(r ui.Rect) ui.Rect {
	return ui.Rect{X: r.X + r.W - heroWaveW - 26, Y: r.Y + r.H - heroWaveH - 20, W: heroWaveW, H: heroWaveH}
}

// heroAlbums returns the albums the listener plays most, for the discs
// over the wave: asked for once, and empty until they answer.
func (a *app) heroAlbums() []albumFace {
	if !a.heroFacesAsked {
		a.heroFacesAsked = true
		go func() {
			plays, err := recentPlays(100)
			a.post(func() {
				if err != nil {
					return
				}
				type tally struct {
					face albumFace
					n    int
				}
				counts := map[string]*tally{}
				for _, play := range plays {
					album := play.Track.Album
					if album.PicURL == "" {
						continue
					}
					t := counts[album.PicURL]
					if t == nil {
						t = &tally{face: albumFace{name: album.Name, cover: album.PicURL}}
						counts[album.PicURL] = t
					}
					t.n++
				}
				list := make([]*tally, 0, len(counts))
				for _, t := range counts {
					list = append(list, t)
				}
				// Most played first; the covers keep their order as a tie.
				sort.SliceStable(list, func(i, j int) bool { return list[i].n > list[j].n })
				faces := make([]albumFace, 0, 3)
				for i := 0; i < len(list) && i < 3; i++ {
					faces = append(faces, list[i].face)
				}
				a.heroFaces = faces
			})
		}()
	}
	if len(a.heroFaces) >= 3 {
		return a.heroFaces
	}
	// Until the plays answer, the charts' covers stand in, so the wave is
	// never bare.
	faces := make([]albumFace, 0, 3)
	faces = append(faces, a.heroFaces...)
	for i := range a.charts {
		if len(faces) >= 3 {
			break
		}
		if cover := a.charts[i].Cover(); cover != "" {
			faces = append(faces, albumFace{name: a.charts[i].Name, cover: cover})
		}
	}
	return faces
}

// drawHeroWave paints the sound of what plays on the lead record: bars of
// its spectrum standing along the foot, and, over them, the three albums
// the listener plays most, bobbing as the music comes.
func (a *app) drawHeroWave(c *ui.Context, p *ui.Painter, r ui.Rect, now time.Time) {
	w := heroWaveRect(r)
	lv, live := a.pl.Levels()
	a.heroBars.step(now, lv, live)
	still := c.Preferences().ReduceMotion

	// The bars: a wave of the music, low ones tall, along the foot.
	bars := ui.Rect{X: w.X, Y: w.Y + w.H - 46, W: w.W, H: 46}
	drawBars(p, bars, &a.heroBars, 26, a.pal.accent.Alpha(0.5), 3)

	// The discs over it, the nearest the largest, each on its own beat.
	faces := a.heroAlbums()
	t := float64(now.UnixNano()) / 1e9
	low := a.heroBars.group(0, 6)
	for i := 0; i < min(3, len(faces)); i++ {
		d := [...]float32{56, 46, 36}[i]
		fromRight := [...]float32{54, 132, 204}[i]
		cy := w.Y + 55
		if !still {
			amp := float32(1.5)
			if live {
				amp += 2 + 14*low
			}
			cy += amp * float32(math.Sin(t*(1.9+0.35*float64(i))+float64(i)*2.1))
		}
		cx := w.X + w.W - fromRight
		rect := ui.Rect{X: cx - d/2, Y: cy - d/2, W: d, H: d}
		// The glow it throws around it, and the pool of light under it.
		p.Fill(ui.Rect{X: rect.X - 7, Y: rect.Y - 7, W: d + 14, H: d + 14}, a.pal.accent.Alpha(0.1), d/2+7)
		p.Fill(ui.Rect{X: rect.X + d*0.14, Y: rect.Y + d*0.84, W: d * 0.72, H: d * 0.24}, a.pal.accent.Alpha(0.16), d*0.12)
		p.Clip(rect, d/2, func() {
			if bmp := bitmap(a.art.get(netease.Sized(faces[i].cover, 220), false)); bmp != nil {
				p.Image(bmp, rect, ui.Cover)
			} else {
				p.Fill(rect, a.pal.hover, d/2)
			}
			// Light on it from above, and its own shade at the foot.
			p.FillGradient(rect, ui.LinearGradient{Angle: 160, Stops: []ui.GradientStop{
				{Color: ui.RGBA(255, 255, 255, 0.22), At: 0},
				{Color: ui.RGBA(255, 255, 255, 0), At: 0.55},
			}}, d/2)
			p.FillGradient(rect, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
				{Color: ui.RGBA(0, 0, 0, 0), At: 0.6},
				{Color: ui.RGBA(0, 0, 0, 0.26), At: 1},
			}}, d/2)
		})
		p.Stroke(rect, ui.RGBA(255, 255, 255, 0.3), d/2, 1.2)
	}
	if live || a.heroBars.group(0, player.Bands) > 0.01 {
		p.AnimationFrame()
	}
}
