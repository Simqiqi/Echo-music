package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// newAlbumsState is the new-albums page: the albums most recently released.
type newAlbumsState struct {
	paged
	albums []netease.NewAlbum
}

func (a *app) loadNewAlbums() {
	if a.newAlbums.loaded || a.newAlbums.loading {
		return
	}
	a.newAlbums.loading = true
	a.newAlbums.err = ""
	go func() {
		al, err := netease.NewAlbums(60)
		a.post(func() {
			a.newAlbums.loading = false
			if err != nil {
				a.newAlbums.err = "新碟加载失败，请检查网络"
				return
			}
			a.newAlbums.albums = al
			a.newAlbums.loaded = true
		})
	}()
}

// newAlbumsPage is what just came out: a grid of covers, the newest first,
// with how many landed today as the music calendar's touch.
func (a *app) newAlbumsPage(c *ui.Context) {
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("newalbums")).TrackScroll(&a.pageScroll).Grow(1).
		Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).
		Children(func() {
			a.startMark(c)
			defer a.endMark(c)
			now := time.Now().In(beijing)
			today := 0
			for _, al := range a.newAlbums.albums {
				if al.PublishTime > 0 {
					pt := al.Published().In(beijing)
					if pt.Year() == now.Year() && pt.Month() == now.Month() && pt.Day() == now.Day() {
						today++
					}
				}
			}
			sub := "每日更新"
			if today > 0 {
				sub = fmt.Sprintf("今日新碟 %d 张", today)
			}
			a.heading(c, "新碟速递", sub)

			if !a.newAlbums.loaded {
				if a.newAlbums.err != "" {
					a.centered(c, a.newAlbums.err, a.loadNewAlbums)
					return
				}
				a.centered(c, "正在加载…", nil)
				return
			}
			if len(a.newAlbums.albums) == 0 {
				a.empty(c, "暂无可展示的新碟")
				return
			}
			ui.Grid(c).Columns(a.columns(c, 168)).GapX(20).GapY(28).Children(func() {
				for i := range a.newAlbums.albums {
					a.albumTile(c.Key(a.newAlbums.albums[i].ID), a.newAlbums.albums[i])
				}
			})
		})
}

// albumTile is a new album: its cover, name, artist and the day it came out,
// opening the album when clicked.
func (a *app) albumTile(c *ui.Context, al netease.NewAlbum) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(8).AlignItems(ui.Stretch).Justify(ui.Start).FocusRing(false).Label(al.Name)
	hovered := card.Hovered()
	card.Children(func() {
		art := ui.Box(c).AspectRatio(1).Radius(10)
		lift := ui.Pose{}
		if hovered {
			lift = ui.Pose{Y: -3}
			art.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			art.Shadow(0, 2, 8, -2, p.cardShade)
		}
		art.AnimatePose(lift, ui.Snappy)
		art.Children(func() {
			a.coverFill(c, al.PicURL, 400, 10)
		})
		ui.Text(c, al.Name).FontSize(13).FontWeight(500).MaxLines(2).LineHeight(1.35)
		sub := al.ArtistName
		if sub == "" {
			sub = "未知歌手"
		}
		if al.PublishTime > 0 {
			sub += " · " + al.Published().Format("2006/01/02")
		}
		ui.Text(c, sub).FontSize(12).TextColor(p.muted).SingleLine()
	})
	if card.Clicked() {
		a.open(route{page: pageAlbum, id: al.ID})
	}
}
