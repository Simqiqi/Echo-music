package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The week's listening, as the app itself hears it: every quarter minute
// that a song plays, a quarter minute goes to that song on that day, and
// the week's card on the home page gathers the days up. It keeps a fortnight
// of them, no more.

// weeklyTrack is what the card says of a song: its name, its artists, and
// where its cover hangs.
type weeklyTrack struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
	Pic    string `json:"pic"`
}

// weeklyData is the record itself: a song's seconds on a day, the hour of
// the day they ran at, and what the songs are called.
type weeklyData struct {
	Days   map[string]map[int64]float64 `json:"days"`
	Hours  map[string]map[int]float64   `json:"hours,omitempty"`
	Tracks map[int64]weeklyTrack        `json:"tracks"`
}

// weeklyState is the record, its place on the disk, and the lock over it.
type weeklyState struct {
	mu   sync.Mutex
	data weeklyData
	path string
}

// weeklyPath is where the record is kept, beside the settings.
func weeklyPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "weekly.json"), nil
}

// newWeekly loads the record from the disk, or starts one empty where
// none is.
func newWeekly() *weeklyState {
	if path, err := weeklyPath(); err == nil {
		return newWeeklyAt(path)
	}
	return newWeeklyAt("")
}

// newWeeklyAt loads the record kept at path, or starts one empty where
// none is. A path of "" keeps nothing on the disk.
func newWeeklyAt(path string) *weeklyState {
	w := &weeklyState{path: path, data: weeklyData{
		Days:   map[string]map[int64]float64{},
		Hours:  map[string]map[int]float64{},
		Tracks: map[int64]weeklyTrack{},
	}}
	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(b, &w.data)
		}
	}
	if w.data.Days == nil {
		w.data.Days = map[string]map[int64]float64{}
	}
	if w.data.Hours == nil {
		w.data.Hours = map[string]map[int]float64{}
	}
	if w.data.Tracks == nil {
		w.data.Tracks = map[int64]weeklyTrack{}
	}
	return w
}

// weeklyKey names a day as the record keeps it.
func weeklyKey(t time.Time) string {
	return t.Format("2006-01-02")
}

// add hears that tr played for secs from now, and keeps it. It lets the
// record run to a fortnight, and saves it.
func (w *weeklyState) add(now time.Time, tr netease.Track, secs float64) {
	if tr.ID == 0 || secs <= 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	day := weeklyKey(now)
	if w.data.Days[day] == nil {
		w.data.Days[day] = map[int64]float64{}
	}
	w.data.Days[day][tr.ID] += secs
	if w.data.Hours[day] == nil {
		w.data.Hours[day] = map[int]float64{}
	}
	w.data.Hours[day][now.Hour()] += secs
	if _, ok := w.data.Tracks[tr.ID]; !ok {
		w.data.Tracks[tr.ID] = weeklyTrack{Name: tr.Name, Artist: tr.ArtistNames(), Pic: tr.Album.PicURL}
	} else {
		t := w.data.Tracks[tr.ID]
		if t.Name == "" {
			t.Name = tr.Name
		}
		if t.Artist == "" {
			t.Artist = tr.ArtistNames()
		}
		if t.Pic == "" {
			t.Pic = tr.Album.PicURL
		}
		w.data.Tracks[tr.ID] = t
	}
	// A fortnight of days, no more.
	cut := weeklyKey(now.AddDate(0, 0, -14))
	for day := range w.data.Days {
		if day < cut {
			delete(w.data.Days, day)
		}
	}
	for day := range w.data.Hours {
		if day < cut {
			delete(w.data.Hours, day)
		}
	}
	w.save()
}

// save writes the record to the disk.
func (w *weeklyState) save() {
	if w.path == "" {
		return
	}
	b, err := json.MarshalIndent(w.data, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(w.path), 0o700)
	_ = os.WriteFile(w.path, b, 0o600)
}

// weeklyReport is a week's listening gathered up: how long it ran, how
// many songs it heard, how that stands against the week before, the hour
// of the day it ran at, and the songs and artists it heard the most.
type weeklyReport struct {
	Minutes         int
	Songs           int
	LastWeekMinutes int
	Story           string
	Tops            []weeklyTop
	Artists         []weeklyArtist
}

// weeklyTop is a song of the week, and the time it had.
type weeklyTop struct {
	Track   weeklyTrack
	Minutes int
}

// weeklyArtist is an artist of the week, and the time their songs had.
type weeklyArtist struct {
	Name    string
	Minutes int
}

// recent is what the app heard over the last few days, the songs with the
// most time first: what the home page offers to go on with. It keeps to
// songs it knows the name of, since a nameless row says nothing.
func (w *weeklyState) recent(now time.Time, n int) []netease.Track {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	secs := map[int64]float64{}
	for d := range 3 {
		for id, s := range w.data.Days[weeklyKey(now.AddDate(0, 0, -d))] {
			secs[id] += s
		}
	}
	type heard struct {
		id   int64
		secs float64
	}
	var list []heard
	for id, s := range secs {
		if t, ok := w.data.Tracks[id]; ok && t.Name != "" {
			list = append(list, heard{id, s})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].secs != list[j].secs {
			return list[i].secs > list[j].secs
		}
		return list[i].id < list[j].id
	})
	if len(list) > n {
		list = list[:n]
	}
	out := make([]netease.Track, 0, len(list))
	for _, h := range list {
		t := w.data.Tracks[h.id]
		tr := netease.Track{ID: h.id, Name: t.Name, Album: netease.Album{PicURL: t.Pic}}
		if t.Artist != "" {
			tr.Artists = []netease.Artist{{Name: t.Artist}}
		}
		out = append(out, tr)
	}
	return out
}

// report gathers the week up: from the Monday of now's week to now.
func (w *weeklyState) report(now time.Time) weeklyReport {
	if w == nil {
		return weeklyReport{}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	r := weeklyReport{}
	// The Monday at the head of now's week, and the Monday of the week
	// before it, for the weeks to stand against each other.
	off := (int(now.Weekday()) + 6) % 7
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	key := weeklyKey(monday)
	last := weeklyKey(monday.AddDate(0, 0, -7))
	perSong := map[int64]float64{}
	perArtist := map[string]float64{}
	perHour := map[int]float64{}
	for day, songs := range w.data.Days {
		if day < key {
			// The week before: it counts, seven days of it, no more.
			if day >= last {
				for _, secs := range songs {
					r.LastWeekMinutes += int(secs/60 + 0.5)
				}
			}
			continue
		}
		for id, secs := range songs {
			perSong[id] += secs
		}
		for hour, secs := range w.data.Hours[day] {
			perHour[hour] += secs
		}
	}
	if len(perSong) == 0 {
		return r
	}
	r.Story = weeklyStory(perHour)
	ids := make([]int64, 0, len(perSong))
	for id, secs := range perSong {
		var total float64
		r.Songs++
		total += secs
		_ = total
		r.Minutes += int(secs/60 + 0.5)
		t, ok := w.data.Tracks[id]
		if ok && t.Artist != "" {
			for _, name := range strings.Split(t.Artist, "/") {
				perArtist[strings.TrimSpace(name)] += secs
			}
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if perSong[ids[i]] != perSong[ids[j]] {
			return perSong[ids[i]] > perSong[ids[j]]
		}
		return ids[i] < ids[j]
	})
	for n, id := range ids {
		if n == 5 {
			break
		}
		t := w.data.Tracks[id]
		if t.Name == "" {
			continue
		}
		r.Tops = append(r.Tops, weeklyTop{Track: t, Minutes: int(perSong[id]/60 + 0.5)})
	}
	names := make([]string, 0, len(perArtist))
	for name := range perArtist {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if perArtist[names[i]] != perArtist[names[j]] {
			return perArtist[names[i]] > perArtist[names[j]]
		}
		return names[i] < names[j]
	})
	for n, name := range names {
		if n == 3 {
			break
		}
		r.Artists = append(r.Artists, weeklyArtist{Name: name, Minutes: int(perArtist[name]/60 + 0.5)})
	}
	return r
}

// weeklyPage is the week's listening, a page of its own from the sidebar:
// a dark stage washed in the color of the week's own cover, the minutes
// it ran at in figures too big to miss, the days as a chart, the songs
// and artists it heard the most — and the song it heard most of all,
// turning on a record of its own.
func (a *app) weeklyPage(c *ui.Context) {
	top, bottom := a.insets(c)
	now := c.Now()
	var r weeklyReport
	if a.weekly != nil {
		r = a.weekly.report(now)
	}
	// The color of the page: the same ground as every other page, lit
	// only where the week's best-loved cover lends it a little of its
	// own color — the accent where the week is still empty.
	tint := a.pal.accent
	if len(r.Tops) > 0 && r.Tops[0].Track.Pic != "" {
		if aw := a.art.get(netease.Sized(r.Tops[0].Track.Pic, 160), false); aw != nil {
			tint = aw.tint
		}
	}
	base := a.pal.bg.Mix(tint, 0.05)
	hi := tint.Mix(white, 0.62)
	if !a.pal.dark {
		hi = tint.Mix(ui.RGB(0, 0, 0), 0.28)
	}
	lo := a.pal.muted
	faint := a.pal.faint

	ui.Scroll(c.Key("weekly")).TrackScroll(&a.pageScroll).Grow(1).
		Padding(top+pageGap, pagePad, bottom+56, pagePad).
		ScrollbarInsets(top, 0, bottom, 0).Background(base).TextColor(a.pal.text).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		if r.Songs == 0 {
			ui.Column(c).Fill().MinHeight(300).Center().Gap(10).Children(func() {
				ui.Icon(c, iconWeekly).Size(38, 38).TextColor(faint)
				ui.Text(c, "这一周还没有听歌").FontSize(15).FontWeight(600).TextColor(lo)
				ui.Text(c, "放一首歌，周报会自己长出来").FontSize(12.5).TextColor(faint)
			})
			return
		}

		// Two columns: the week's story at the left, and its side
		// rooms — the cover wall, the clock, the small numbers — at
		// the right.
		var st weeklyStats
		if a.weekly != nil {
			st = a.weekly.stats(now)
		}
		ui.Row(c).AlignItems(ui.Start).Gap(40).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).MaxWidth(560).Children(func() {

				// The week at its head: what days it ran over, the minutes it
				// ran for, and how that stands against the week before.
				ui.Textf(c, "%s – %s · 本周听歌报告", weeklyKey(mondayOf(now))[5:], weeklyKey(now)[5:]).
					FontSize(12.5).FontWeight(600).FontFeatures("tnum").TextColor(hi)
				ui.Row(c).AlignItems(ui.Baseline).Gap(12).Margin(6, 0, 0, 0).Children(func() {
					ui.Textf(c, "%d", r.Minutes).FontSize(76).FontWeight(800).FontFeatures("tnum").
						LetterSpacing(-2.5).TextColor(a.pal.text)
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, "分钟").FontSize(15).FontWeight(700).TextColor(hi)
						ui.Textf(c, "%d 首歌曲", r.Songs).FontSize(12.5).TextColor(lo)
					})
				})
				if cmp := weeklyCompare(r); cmp != "" {
					ui.Text(c, cmp).FontSize(13).FontWeight(600).TextColor(hi).Margin(2, 0, 0, 0)
				}
				// The week at a glance: the days it ran without pause, how a
				// day of it sits against the days before, and the fullest day.
				ui.Row(c).Gap(10).Margin(16, 0, 0, 0).MaxWidth(560).Children(func() {
					a.weeklyChip(c, fmt.Sprintf("%d", st.Streak), "连续听歌 · 天")
					a.weeklyChip(c, fmt.Sprintf("%.0f", st.AvgDay), "日均 · 分钟")
					a.weeklyChip(c, fmt.Sprintf("%d", st.BestDayMin), "单日最高 · 分钟")
				})

				// The days, as seven bars, Monday at the left: the fullest day
				// burns at its brightest, the rest fall away.
				chart := ui.Box(c.Key("chart")).Fill().MaxWidth(560).Height(96).Margin(18, 0, 0, 0)
				chart.Draw(func(p *ui.Painter, rect ui.Rect) {
					days := a.weekly.bars(now)
					n := float32(len(days))
					bw := float32(26)
					gap := (rect.W - bw*n) / (n - 1)
					for i, v := range days {
						h := max(v*float32(rect.H-6), 8)
						x := rect.X + float32(i)*(bw+gap)
						col := tint.Alpha(0.28)
						if v >= 0.999 {
							col = hi
						}
						p.Fill(ui.Rect{X: x, Y: rect.Y + rect.H - h, W: bw, H: h}, col, 7)
					}
				})
				ui.Row(c).Justify(ui.SpaceBetween).MaxWidth(560).Margin(6, 0, 0, 0).Children(func() {
					for _, d := range []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"} {
						ui.Text(c, d).FontSize(11).FontWeight(600).TextColor(faint)
					}
				})
				// The hours of the day it ran at, as twenty-four slim bars: the
				// peak burns brightest, the rest fall away with it.
				ui.Text(c, "一天中的时刻").FontSize(12).FontWeight(700).TextColor(faint).Margin(26, 0, 8, 0)
				ui.Box(c.Key("hours")).Fill().MaxWidth(560).Height(64).
					Draw(func(p *ui.Painter, rect ui.Rect) {
						var top float64
						for _, v := range st.Hours24 {
							if v > top {
								top = v
							}
						}
						if top <= 0 {
							return
						}
						slot := rect.W / 24
						bw := slot * 0.6
						for h, v := range st.Hours24 {
							if v <= 0 {
								continue
							}
							rel := float32(v / top)
							hh := max(rel*float32(rect.H-4), 4)
							x := rect.X + float32(h)*slot + (slot-bw)/2
							col := tint.Alpha(0.22 + 0.5*rel)
							if rel >= 0.999 {
								col = hi
							}
							p.Fill(ui.Rect{X: x, Y: rect.Y + rect.H - hh, W: bw, H: hh}, col, bw/2)
						}
					})
				ui.Row(c).Justify(ui.SpaceBetween).MaxWidth(560).Margin(5, 0, 0, 0).Children(func() {
					for _, h := range []string{"0", "6", "12", "18", "24"} {
						ui.Text(c, h).FontSize(10.5).FontWeight(600).FontFeatures("tnum").TextColor(faint)
					}
				})
				if r.Story != "" {
					ui.Text(c, r.Story).FontSize(13.5).FontWeight(700).TextColor(hi).Margin(16, 0, 0, 0)
				}

				// The song of the week, on a record of its own, and how much of
				// the week it carried.
				if len(r.Tops) > 0 {
					t := r.Tops[0]
					ui.Row(c).Gap(20).AlignItems(ui.Center).Margin(26, 0, 0, 0).Children(func() {
						a.weeklyDisc(c, t.Track.Pic, 132)
						ui.Column(c).Gap(5).Children(func() {
							ui.Text(c, "本周最常听").FontSize(12).FontWeight(600).TextColor(faint)
							ui.Text(c, t.Track.Name).FontSize(21).FontWeight(800).SingleLine()
							if t.Track.Artist != "" {
								ui.Text(c, t.Track.Artist).FontSize(13).TextColor(lo).SingleLine()
							}
							ui.Textf(c, "转了 %d 分钟", t.Minutes).FontSize(12.5).FontWeight(600).
								FontFeatures("tnum").TextColor(hi).Margin(2, 0, 0, 0)
						})
					})
					if r.Minutes > 0 {
						share := float32(t.Minutes) / float32(max(r.Minutes, 1))
						ui.Column(c).MaxWidth(560).Margin(14, 0, 0, 0).Gap(5).Children(func() {
							bar := ui.Box(c.Key("share")).Fill().Height(6).Radius(3).Background(a.pal.hairline)
							bar.Children(func() {
								ui.Box(c).Height(6).Width(max(share*300, 12)).MaxWidth(300).Radius(3).Background(hi)
							})
							ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
								ui.Text(c, t.Track.Name).FontSize(10.5).FontWeight(600).TextColor(faint)
								ui.Textf(c, "占本周 %d%%", int(share*100+0.5)).FontSize(10.5).
									FontWeight(600).FontFeatures("tnum").TextColor(faint)
							})
						})
					}
				}

				// The songs after the first, each with its cover for a face.
				if len(r.Tops) > 1 {
					ui.Text(c, "其他常听").FontSize(12).FontWeight(700).TextColor(faint).Margin(28, 0, 10, 0)
					ui.Column(c).MaxWidth(560).Gap(4).Children(func() {
						for i, t := range r.Tops[1:] {
							rkey := t.Track.Pic
							if rkey == "" {
								rkey = t.Track.Name
							}
							row := ui.Row(c.Key(rkey)).Fill().Height(52).Padding(0, 10).Gap(12).
								AlignItems(ui.Center).Radius(10).Label("周报歌曲" + t.Track.Name)
							if row.Hovered() {
								row.Background(a.pal.hover)
							}
							row.Children(func() {
								// The second and the third carry their medals;
								// the rest wear the house color.
								bg, tx := hi.Alpha(0.16), hi
								if i == 0 {
									bg, tx = ui.Hex("#9fa4ad"), ui.Hex("#ffffff")
								} else if i == 1 {
									bg, tx = ui.Hex("#c08a5a"), ui.Hex("#ffffff")
								}
								ui.Box(c).Size(22, 22).Radius(11).Background(bg).
									AlignItems(ui.Center).Justify(ui.Center).Children(func() {
									ui.Textf(c, "%d", i+2).FontSize(12).FontWeight(800).FontFeatures("tnum").TextColor(tx)
								})
								cover := ui.Box(c.Key("cover")).Size(38, 38).Radius(7).Background(a.pal.hover).Shrink(0)
								bmp := bitmap(a.art.get(netease.Sized(t.Track.Pic, 160), false))
								in := artIn(cover, bmp)
								cover.Children(func() { artImage(c, bmp, 7, in) })
								ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
									ui.Text(c, t.Track.Name).FontSize(13.5).FontWeight(700).SingleLine()
									if t.Track.Artist != "" {
										ui.Text(c, t.Track.Artist).FontSize(11.5).TextColor(lo).SingleLine()
									}
								})
								ui.Textf(c, "%d 分钟", t.Minutes).FontSize(12).FontWeight(600).
									FontFeatures("tnum").TextColor(lo)
							})
						}
					})
				}

				// The artists their songs ran to.
				if len(r.Artists) > 0 {
					ui.Text(c, "最常听的歌手").FontSize(12).FontWeight(700).TextColor(faint).Margin(24, 0, 10, 0)
					ui.Row(c).MaxWidth(560).Gap(8).Wrap().Children(func() {
						for _, ar := range r.Artists {
							ui.Row(c.Key(ar.Name)).Padding(0, 12, 0, 12).Height(30).Radius(999).Gap(6).
								AlignItems(ui.Center).Background(a.pal.hover).Children(func() {
								ui.Text(c, ar.Name).FontSize(13).FontWeight(700).SingleLine()
								ui.Textf(c, "%d′", ar.Minutes).FontSize(12).FontWeight(600).
									FontFeatures("tnum").TextColor(lo)
							})
						}
					})
				}

				// The fortnight behind, one cell a day: the brighter the cell,
				// the more the day carried — and today outlined at its end.
				ui.Text(c, "近两周").FontSize(12).FontWeight(700).TextColor(faint).Margin(28, 0, 10, 0)
				ui.Row(c).MaxWidth(560).Gap(6).Children(func() {
					var top float64
					for _, v := range st.Fortnight {
						if v > top {
							top = v
						}
					}
					for i, v := range st.Fortnight {
						day := now.AddDate(0, 0, -13+i)
						cell := ui.Box(c.Key(i)).Size(30, 30).Radius(8)
						switch {
						case i == 13:
							cell.Border(1.5, hi).Background(hi.Alpha(0.14))
						case top <= 0 || v <= 0:
							cell.Background(a.pal.hover)
						default:
							cell.Background(tint.Alpha(0.18 + 0.62*float32(v/top)))
						}
						cell.Label(fmt.Sprintf("%d月%d日 · %.0f 分钟", int(day.Month()), day.Day(), v))
					}
				})
			})
			// The side rooms of the week: its covers, its clock, its
			// small numbers.
			a.weeklySide(c, r, st, tint, hi, lo, faint, now)
		})
	})
}

// weeklySide is the week's right-hand column: the covers it ran to as a
// small wall, the clock it ran on as a grid of cells, and the numbers
// the totals alone don't say.
func (a *app) weeklySide(c *ui.Context, r weeklyReport, st weeklyStats, tint, hi, lo, faint ui.Color, now time.Time) {
	ui.Column(c).Width(278).Shrink(0).Gap(24).Children(func() {
		// The wall: the covers of the week, fullest first, nine at the
		// most, each a small square of the week it carried.
		wt := a.weekly.weekTracks(now)
		if len(wt) > 0 {
			ui.Text(c, "封面墙").FontSize(12).FontWeight(700).TextColor(faint)
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for i, t := range wt {
					if i == 9 {
						break
					}
					// Keyed by its cover, not by its place: wherever
					// the song sits in the order, its square is its
					// own, and its cover does not dance.
					key := t.Track.Pic
					if key == "" {
						key = t.Track.Name
					}
					sq := ui.Box(c.Key(key)).Size(86, 86).Radius(10).Background(a.pal.hover).Shrink(0)
					sq.Label(t.Track.Name + " · " + t.Track.Artist)
					bmp := bitmap(a.art.get(netease.Sized(t.Track.Pic, 160), false))
					in := artIn(sq, bmp)
					sq.Children(func() { artImage(c, bmp, 10, in) })
				}
			})
		}

		// The clock: a cell a day and hour, Monday at the top, midnight
		// at the left — the brighter the cell, the more it carried.
		clock := a.weekly.weekClock(now)
		ui.Text(c, "一周的时钟").FontSize(12).FontWeight(700).TextColor(faint)
		ui.Column(c).Gap(3).Children(func() {
			var top float64
			for _, row := range clock {
				for _, v := range row {
					if v > top {
						top = v
					}
				}
			}
			wd := []string{"一", "二", "三", "四", "五", "六", "日"}
			for d := 0; d < 7; d++ {
				ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
					ui.Text(c, wd[d]).FontSize(9.5).FontWeight(600).TextColor(faint)
					for h := 0; h < 24; h++ {
						cell := ui.Box(c.Key(h)).Size(9, 9).Radius(2.5)
						v := clock[d][h]
						switch {
						case top <= 0 || v <= 0:
							cell.Background(a.pal.hairline)
						default:
							cell.Background(tint.Alpha(0.2 + 0.62*float32(v/top)))
						}
					}
				})
			}
		})

		// The small numbers: the songs and singers of the week, the
		// hours it woke and slept, the day it leaned on.
		first, last := -1, -1
	scan:
		for h := 0; h < 24; h++ {
			for d := 0; d < 7; d++ {
				if clock[d][h] > 0 {
					first = h
					break scan
				}
			}
		}
		for h := 23; h >= 0; h-- {
			for d := 0; d < 7; d++ {
				if clock[d][h] > 0 {
					last = h
					break
				}
			}
			if last >= 0 {
				break
			}
		}
		singers := map[string]bool{}
		for _, t := range wt {
			for _, name := range strings.Split(t.Track.Artist, "/") {
				if name = strings.TrimSpace(name); name != "" {
					singers[name] = true
				}
			}
		}
		wd := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
		facts := [][2]string{
			{"听到歌手", fmt.Sprintf("%d 位", len(singers))},
		}
		if first >= 0 {
			facts = append(facts,
				[2]string{"最早一次", fmt.Sprintf("%d 点", first)},
				[2]string{"最晚一次", fmt.Sprintf("%d 点", last)},
				[2]string{"最投入的一天", fmt.Sprintf("%s · %d 分钟", wd[st.BestDayWd], st.BestDayMin)},
			)
		}
		ui.Column(c).Gap(10).Padding(14, 16, 14, 16).Radius(14).Background(a.pal.hover).Children(func() {
			for _, f := range facts {
				ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					ui.Text(c, f[0]).FontSize(12).TextColor(faint)
					ui.Text(c, f[1]).FontSize(12.5).FontWeight(700).FontFeatures("tnum").TextColor(a.pal.text)
				})
			}
		})
	})
}

// mondayOf is the Monday at the head of now's week.
func mondayOf(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(int(now.Weekday())+6)%7)
}

// weeklyCompare stands this week against the last, as the listener says
// it. Nothing to stand against, and it says the week is the first.
func weeklyCompare(r weeklyReport) string {
	switch {
	case r.LastWeekMinutes <= 0:
		return "这是你被记录的第一周"
	case r.Minutes > r.LastWeekMinutes:
		return fmt.Sprintf("比上周多听了 %d 分钟", r.Minutes-r.LastWeekMinutes)
	case r.Minutes < r.LastWeekMinutes:
		return fmt.Sprintf("比上周少听了 %d 分钟", r.LastWeekMinutes-r.Minutes)
	default:
		return "和上周持平"
	}
}

// weeklyDisc is the song of the week on a record: the cover for a label,
// turning slowly, whatever plays — the week's own, spinning whether or
// not the room is silent.
func (a *app) weeklyDisc(c *ui.Context, pic string, d float32) {
	platter := ui.Box(c.Key("weekly-disc")).Size(d, d).Radius(d/2).Background(ui.Hex("#101013")).
		Shadow(0, 24, 48, -10, a.pal.cardShade)
	platter.Children(func() {
		rim := d - 3
		ui.Box(c.Key("rim")).Size(rim, rim).Radius(rim/2).Border(1, white.Alpha(0.1)).
			Absolute().Top(1.5).Left(1.5)
		disc := ui.Box(c.Key("record")).Absolute().Top(0).Left(0).Size(d, d)
		if !c.Preferences().ReduceMotion {
			disc.TransformOrigin(0.5, 0.5).Rotate(disc.Loop("wspin", vinylRev, ui.Linear) * 360)
		}
		disc.Children(func() {
			for i, k := range []float32{0.475, 0.45, 0.425, 0.40, 0.375, 0.35, 0.325, 0.30, 0.275} {
				gd := d * k * 2
				ui.Box(c.Key(i)).Size(gd, gd).Radius(gd/2).Border(1, white.Alpha(0.05)).
					Absolute().Top((d - gd) / 2).Left((d - gd) / 2)
			}
			ld := d * 0.52
			label := ui.Box(c.Key("label")).Size(ld, ld).Radius(ld / 2).Background(white.Alpha(0.08)).
				Absolute().Top((d - ld) / 2).Left((d - ld) / 2)
			bmp := bitmap(a.art.get(netease.Sized(pic, 300), false))
			if bmp == nil {
				label.Background(ui.RGB(70, 70, 78))
			}
			in := artIn(label, bmp)
			label.Children(func() { artImage(c, bmp, ld/2, in) })
			hd := max(d*0.032, 5)
			ui.Box(c.Key("spindle")).Size(hd, hd).Radius(hd / 2).Background(ui.Hex("#0b0b0d")).
				Absolute().Top((d - hd) / 2).Left((d - hd) / 2)
		})
	})
}

// weeklyStory names the part of the day the week ran at, from its hours:
// the hour that carried the most, as the listener says it — the small
// hours, the evening, the afternoon.
func weeklyStory(perHour map[int]float64) string {
	var best int
	var top float64
	for h, v := range perHour {
		if v > top {
			best, top = h, v
		}
	}
	if top <= 0 {
		return ""
	}
	switch {
	case best >= 5 && best < 11:
		return "这一周，你在早晨听得最多"
	case best >= 11 && best < 14:
		return "这一周，你在午间听得最多"
	case best >= 14 && best < 18:
		return "这一周，你在下午听得最多"
	case best >= 18 && best < 23:
		return "这一周，你在夜晚听得最多"
	default:
		return "这一周，你在深夜听得最多"
	}
}

// bars is the week's listening as a bar a day, Monday first, each from 0
// to 1 against the fullest day of the week.
func (w *weeklyState) bars(now time.Time) []float32 {
	if w == nil {
		return make([]float32, 7)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	off := (int(now.Weekday()) + 6) % 7
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	days := make([]float32, 7)
	var top float64
	for i := range days {
		day := weeklyKey(monday.AddDate(0, 0, i))
		var secs float64
		for _, v := range w.data.Days[day] {
			secs += v
		}
		days[i] = float32(secs)
		if secs > top {
			top = secs
		}
	}
	if top > 0 {
		for i := range days {
			days[i] /= float32(top)
		}
	}
	return days
}

// weeklyStats gathers the shapes of the week that the totals alone don't
// tell: the hour of the day it ran at, the fortnight behind it, the days
// it has run without pause, and how a day of it sits against the average.
type weeklyStats struct {
	Hours24    [24]float64 // seconds per hour of the day, this week
	Fortnight  [14]float64 // minutes per day, the fortnight ending today
	Streak     int         // the days it has run without pause, today in
	BestDayMin int         // the fullest day of the week, in minutes
	BestDayWd  int         // which weekday that was, Monday nought
	AvgDay     float64     // the days that have run, shared evenly
}

// stats gathers those shapes from the record.
func (w *weeklyState) stats(now time.Time) weeklyStats {
	var s weeklyStats
	if w == nil {
		return s
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	off := (int(now.Weekday()) + 6) % 7
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	var weekTop float64
	var weekSecs float64
	for i := 0; i < 7; i++ {
		day := weeklyKey(monday.AddDate(0, 0, i))
		var secs float64
		for _, v := range w.data.Days[day] {
			secs += v
		}
		weekSecs += secs
		if secs > weekTop {
			weekTop = secs
			s.BestDayMin = int(secs/60 + 0.5)
			s.BestDayWd = i
		}
		for h, v := range w.data.Hours[day] {
			s.Hours24[h] += v
		}
	}
	s.AvgDay = weekSecs / 60 / float64(off+1)
	for i := 0; i < 14; i++ {
		day := weeklyKey(now.AddDate(0, 0, -13+i))
		var secs float64
		for _, v := range w.data.Days[day] {
			secs += v
		}
		s.Fortnight[i] = secs / 60
	}
	// The streak runs back from today — or from yesterday, where today
	// has not yet begun — over the days the record heard something.
	start := now
	if s.Fortnight[13] <= 0 {
		start = now.AddDate(0, 0, -1)
	}
	for d := start; s.Streak < 14; d = d.AddDate(0, 0, -1) {
		var secs float64
		for _, v := range w.data.Days[weeklyKey(d)] {
			secs += v
		}
		if secs <= 0 {
			break
		}
		s.Streak++
	}
	return s
}

// weeklyChip is one small card of the week at a glance: a figure big
// enough to read across the room, and what the figure counts.
func (a *app) weeklyChip(c *ui.Context, big, label string) {
	ui.Column(c).Grow(1).Gap(2).Padding(12, 14, 12, 14).Radius(12).
		Background(a.pal.hover).Children(func() {
		ui.Textf(c, "%s", big).FontSize(22).FontWeight(800).FontFeatures("tnum").
			TextColor(a.pal.text)
		ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(a.pal.faint)
	})
}

// weekClock is the week as the clock saw it: a cell a day and hour,
// Monday first, the seconds each cell carried.
func (w *weeklyState) weekClock(now time.Time) [7][24]float64 {
	var g [7][24]float64
	if w == nil {
		return g
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	off := (int(now.Weekday()) + 6) % 7
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	for i := 0; i < 7; i++ {
		day := weeklyKey(monday.AddDate(0, 0, i))
		for h, v := range w.data.Hours[day] {
			g[i][h] = v
		}
	}
	return g
}

// weekTracks is the week's songs, fullest first, a dozen at most.
func (w *weeklyState) weekTracks(now time.Time) []weeklyTop {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	off := (int(now.Weekday()) + 6) % 7
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	perSong := map[int64]float64{}
	for i := 0; i < 7; i++ {
		for id, secs := range w.data.Days[weeklyKey(monday.AddDate(0, 0, i))] {
			perSong[id] += secs
		}
	}
	ids := make([]int64, 0, len(perSong))
	for id := range perSong {
		if w.data.Tracks[id].Name != "" {
			ids = append(ids, id)
		}
	}
	// Fullest first, and where two ran the same, the smaller id first:
	// the order must not wander, or the covers dance.
	sort.Slice(ids, func(i, j int) bool {
		if perSong[ids[i]] != perSong[ids[j]] {
			return perSong[ids[i]] > perSong[ids[j]]
		}
		return ids[i] < ids[j]
	})
	if len(ids) > 12 {
		ids = ids[:12]
	}
	tops := make([]weeklyTop, 0, len(ids))
	for _, id := range ids {
		tops = append(tops, weeklyTop{Track: w.data.Tracks[id], Minutes: int(perSong[id]/60 + 0.5)})
	}
	return tops
}
