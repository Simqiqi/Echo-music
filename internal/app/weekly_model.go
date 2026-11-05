package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The week's listening, as the app itself hears it: every quarter minute
// that a song plays, a quarter minute goes to that song on that day, and
// the week's card on the home page gathers the days up. It keeps a fortnight
// of them, no more.

// weeklyTrack is what the card says of a song: its name, its artists, and
// where its cover hangs.
type weeklyTrack struct {
	Name    string `json:"name"`
	Artist  string `json:"artist"`
	Pic     string `json:"pic"`
	AlbumID int64  `json:"album_id,omitempty"`
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
		w.data.Tracks[tr.ID] = weeklyTrack{Name: tr.Name, Artist: tr.ArtistNames(), Pic: tr.Album.PicURL, AlbumID: tr.Album.ID}
	} else {
		t := w.data.Tracks[tr.ID]
		if t.AlbumID == 0 {
			t.AlbumID = tr.Album.ID
		}
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
	// A month and a little more: the weekly page needs a fortnight, and
	// a look back over thirty days needs the rest.
	cut := weeklyKey(now.AddDate(0, 0, -35))
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
	ID      int64
	Track   weeklyTrack
	Minutes int
}

// song is the top's track as the player knows it, so that a click in
// the report can set it spinning at once.
func (t weeklyTop) song() netease.Track {
	tr := netease.Track{ID: t.ID, Name: t.Track.Name, Album: netease.Album{PicURL: t.Track.Pic}}
	if t.Track.Artist != "" {
		tr.Artists = []netease.Artist{{Name: t.Track.Artist}}
	}
	return tr
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
		tr := netease.Track{ID: h.id, Name: t.Name, Album: netease.Album{ID: t.AlbumID, PicURL: t.Pic}}
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
	r.Minutes, r.Songs, r.Tops, r.Artists = w.rank(perSong)
	return r
}

// rank turns a span's seconds per song into its totals, the songs heard
// most and the singers behind them. Its caller holds the record's lock.
func (w *weeklyState) rank(perSong map[int64]float64) (minutes, songs int, tops []weeklyTop, artists []weeklyArtist) {
	perArtist := map[string]float64{}
	ids := make([]int64, 0, len(perSong))
	for id, secs := range perSong {
		songs++
		minutes += int(secs/60 + 0.5)
		if t, ok := w.data.Tracks[id]; ok && t.Artist != "" {
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
		tops = append(tops, weeklyTop{ID: id, Track: t, Minutes: int(perSong[id]/60 + 0.5)})
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
		artists = append(artists, weeklyArtist{Name: name, Minutes: int(perArtist[name]/60 + 0.5)})
	}
	return
}

// periodDays is how many minutes each of the last days heard, oldest first.
func (w *weeklyState) periodDays(now time.Time, days int) []float64 {
	if w == nil {
		return nil
	}
	days = max(days, 1)
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]float64, days)
	for i := range days {
		var secs float64
		for _, v := range w.data.Days[weeklyKey(now.AddDate(0, 0, -(days-1-i)))] {
			secs += v
		}
		out[i] = secs / 60
	}
	return out
}

// periodReport gathers the last days of listening, today in, for a poster
// that looks back over a month as the page looks back over a week. It has
// no week to stand against, so LastWeekMinutes stays empty.
func (w *weeklyState) periodReport(now time.Time, days int) weeklyReport {
	if w == nil {
		return weeklyReport{}
	}
	days = max(days, 1)
	w.mu.Lock()
	defer w.mu.Unlock()
	r := weeklyReport{}
	from := weeklyKey(now.AddDate(0, 0, -(days - 1)))
	perSong := map[int64]float64{}
	perHour := map[int]float64{}
	for day, songs := range w.data.Days {
		if day < from {
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
	r.Minutes, r.Songs, r.Tops, r.Artists = w.rank(perSong)
	return r
}
