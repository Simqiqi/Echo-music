package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// downloadsState is the downloads page: the songs kept on disk and the
// tracks they name, looked up once from NetEase.
type downloadsState struct {
	loaded  bool
	entries []player.DownloadEntry
	tracks  map[int64]netease.Track
	err     string
}

func (a *app) loadDownloads() {
	if a.downloads.loaded {
		return
	}
	go func() {
		entries := []player.DownloadEntry{}
		if a.pl.Cache != nil {
			entries = a.pl.Cache.Entries()
		}
		tracks := map[int64]netease.Track{}
		ids := make([]int64, 0, len(entries))
		for _, e := range entries {
			ids = append(ids, e.ID)
		}
		if len(ids) > 0 {
			if ts, err := netease.SongDetails(ids); err == nil {
				for _, t := range ts {
					tracks[t.ID] = t
				}
			}
		}
		a.post(func() {
			a.downloads.entries = entries
			a.downloads.tracks = tracks
			a.downloads.loaded = true
		})
	}()
}

// downloadsPage lists what the listener kept on disk, by the song it is,
// with a way to take each one off.
func (a *app) downloadsPage(c *ui.Context) {
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("downloads")).TrackScroll(&a.pageScroll).Grow(1).
		Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).
		Children(func() {
			a.startMark(c)
			defer a.endMark(c)
			a.heading(c, "下载管理", "已缓存到本地的歌曲，可离线收听")

			if !a.downloads.loaded {
				return
			}
			var total int64
			for _, e := range a.downloads.entries {
				total += e.Size
			}
			ui.Row(c).Gap(10).Margin(0, 0, 16, 0).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("%d 首 · %s", len(a.downloads.entries), mb(total))).
					FontSize(13).TextColor(a.pal.muted)
				if len(a.downloads.entries) > 0 {
					if pillButton(c.Key("clear"), iconTrash, "清空", a.pal.hover.Over(a.pal.bg), a.pal.text).Clicked() {
						a.clearDownloads()
					}
				}
			})
			if len(a.downloads.entries) == 0 {
				a.empty(c, "还没有下载的歌曲")
				return
			}
			ui.Column(c).Gap(2).Children(func() {
				for i := range a.downloads.entries {
					a.downloadRow(c.Key(a.downloads.entries[i].ID), &a.downloads.entries[i])
				}
			})
		})
}

// downloadRow is a kept song: its cover and name, the level it is, and a
// way to drop it. Clicking the song plays it; the trash takes it off disk.
func (a *app) downloadRow(c *ui.Context, e *player.DownloadEntry) {
	p := a.pal
	tr := a.downloads.tracks[e.ID]
	pic := ""
	if tr.ID != 0 {
		pic = tr.Album.PicURL
	}
	ui.Row(c).Height(56).PaddingX(10).Gap(12).Radius(10).AlignItems(ui.Center).Children(func() {
		play := ui.ButtonBase(c.Key("play")).Gap(12).AlignItems(ui.Center).Grow(1).Shrink(1).MinWidth(0).
			FocusRing(false).Label("播放")
		if h := play.Hovered(); h {
			play.Background(p.hover)
		}
		play.Children(func() {
			a.cover(c, pic, 96, 38, 6)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				name := tr.Name
				if name == "" {
					name = fmt.Sprintf("歌曲 #%d", e.ID)
				}
				ui.Text(c, name).FontSize(14).FontWeight(500).SingleLine()
				sub := levelName(e.Level)
				if tr.ID != 0 {
					sub = tr.ArtistNames() + " · " + sub
				}
				ui.Text(c, sub).FontSize(12).TextColor(p.muted).SingleLine()
			})
		})
		if play.Clicked() && tr.ID != 0 {
			a.pl.PlayQueue([]netease.Track{tr}, 0)
		}
		ui.Text(c, mb(e.Size)).FontSize(12).TextColor(p.faint).Width(72).TextAlign(ui.End)
		if iconButton(c.Key("del"), iconTrash, 17, p.muted, "删除").Clicked() {
			a.deleteDownload(e.Path)
		}
	})
}

func (a *app) deleteDownload(path string) {
	if a.pl.Cache != nil {
		a.pl.Cache.Remove(path)
	}
	a.downloads.loaded = false
	a.loadDownloads()
}

func (a *app) clearDownloads() {
	if a.pl.Cache != nil {
		a.pl.Cache.Clear()
	}
	a.downloads.loaded = false
	a.loadDownloads()
}

// levelName is the Chinese name of a stream level.
func levelName(level string) string {
	switch level {
	case "standard":
		return "标准"
	case "higher":
		return "较高"
	case "exhigh":
		return "极高"
	case "lossless":
		return "无损"
	case "hires":
		return "Hi-Res"
	}
	return level
}

// mb formats bytes as a short size, as the downloads page shows it.
func mb(n int64) string {
	const (
		kb = 1024
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/gb)
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%d KB", n/kb)
	}
	return fmt.Sprintf("%d B", n)
}
