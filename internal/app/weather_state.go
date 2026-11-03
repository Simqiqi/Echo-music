package app

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// wxState is the weather, the mood it picked, the playlists it asked
// for, and the songs of the one it picked: all of it, until it goes
// stale, half an hour to the minute. What it last heard is kept on the
// disk, so the page is written the moment it opens and asks the sky
// again behind what it shows.
type wxState struct {
	data   *weatherNow
	err    string
	asking bool
	at     time.Time

	mood      wxMood
	playlists []netease.Playlist
	plErr     string
	plAsking  bool

	picked      *netease.Playlist
	tracks      []netease.Track
	tracksErr   string
	tracksAsk   bool
	autoPlayed  bool
	seekKeyword string
	rng         *rand.Rand

	shots    []wxShot
	shot     wxShot
	shotAsk  bool
	shotAt   time.Time       // when the day's photographs were asked for
	shown    string          // the photograph the strip is drawing
	washed   map[string]bool // the photographs too bright to read over
	shotTry  int             // how many of them the page has turned down
	rollNext bool            // a photograph is owed to the next opening

	path string
	art  *artCache

	hover   bool      // the pointer is on the strip
	sweepAt time.Time // when the light began to pass over it
}

// wxCache is what the page keeps between openings: the sky it last
// heard, the mood it made of it, the playlist that answered, the songs
// of it, and the day's photographs.
type wxCache struct {
	At       time.Time         `json:"at"`
	Data     *weatherNow       `json:"data"`
	Keyword  string            `json:"keyword"`
	Playlist *netease.Playlist `json:"playlist"`
	Tracks   []netease.Track   `json:"tracks"`
	Shots    []wxShot          `json:"shots"`
	ShotAt   time.Time         `json:"shotAt"`
	Shot     wxShot            `json:"shot"`
}

// wxPath is where the weather is kept, beside the settings.
func wxPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "weather.json"), nil
}

// newWx starts the weather's state from what was kept, its dice ready.
func newWx() *wxState {
	s := &wxState{rng: rand.New(rand.NewSource(time.Now().UnixNano())), washed: map[string]bool{}}
	if path, err := wxPath(); err == nil {
		s.path = path
		s.load()
	}
	return s
}

// load reads the weather kept at the state's path, if it is there.
func (s *wxState) load() {
	if s.path == "" {
		return
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var c wxCache
	if json.Unmarshal(b, &c) != nil || c.Data == nil {
		return
	}
	s.data, s.at = c.Data, c.At
	s.mood = wxMoodOf(c.Data.Kind, c.Data.Night, c.Data.Temp)
	s.seekKeyword = c.Keyword
	if c.Playlist != nil {
		pl := *c.Playlist
		s.playlists = []netease.Playlist{pl}
		s.picked = &s.playlists[0]
	}
	s.tracks = c.Tracks
	s.shots, s.shotAt = c.Shots, c.ShotAt
	s.shot = c.Shot
	for i := range s.shots {
		s.shots[i].Credit = creditOf(s.shots[i].Credit)
	}
	s.shot.Credit = creditOf(s.shot.Credit)
	if s.shot.URL != "" {
		s.shown = s.shot.URL
	}
	// A playlist from another day is another weather's answer: ask
	// again, and show the sky meanwhile.
	if time.Since(c.At) > 6*time.Hour {
		s.playlists, s.picked, s.tracks = nil, nil, nil
	}
}

// save keeps what the page has, for the next time it opens: on a
// goroutine, since the disk is no place to keep a frame waiting.
func (s *wxState) save() {
	if s.path == "" || s.data == nil {
		return
	}
	c := wxCache{
		At:       s.at,
		Data:     s.data,
		Keyword:  s.seekKeyword,
		Playlist: s.picked,
		Tracks:   s.tracks,
		Shots:    s.shots,
		ShotAt:   s.shotAt,
		Shot:     s.shot,
	}
	if len(c.Tracks) > 40 {
		c.Tracks = c.Tracks[:40]
	}
	path := s.path
	go func() {
		b, err := json.Marshal(c)
		if err != nil {
			return
		}
		// Written beside and moved over: a half-written report is no
		// report at all.
		tmp := path + ".tmp"
		if os.WriteFile(tmp, b, 0o644) == nil {
			_ = os.Rename(tmp, path)
		}
	}()
}

// warm asks for the sky and the day's photographs as the app starts, so
// that the page has them before it is opened.
func (s *wxState) warm(post func(func())) {
	if s.data == nil || time.Since(s.at) >= wxStale {
		s.ensure(post, nil)
	}
	s.ensureShot(post)
}

// stale is how long the sky's word holds. Past it, the page asks again.
const wxStale = 30 * time.Minute

// shotsStale is how long the day's photographs hold: they change once a
// day, so a few hours of them is enough.
const shotsStale = 4 * time.Hour

// ensure asks for the weather where it is missing or gone stale, then
// for the playlists the mood asks for. What it keeps shows meanwhile.
func (s *wxState) ensure(post func(func()), onWeather func()) {
	if s.asking {
		return
	}
	if s.data != nil && time.Since(s.at) < wxStale {
		s.ensurePlaylists(post)
		return
	}
	s.asking, s.err = true, ""
	go func() {
		w, err := weatherIn()
		post(func() {
			s.asking = false
			if err != nil {
				if s.data == nil {
					s.err = "天气拿不到：" + err.Error()
				}
				return
			}
			s.data, s.at = w, time.Now()
			// Another sky is another mood, and the playlist that
			// answered the old one answers nothing now.
			mood := wxMoodOf(w.Kind, w.Night, w.Temp)
			if s.mood.Title != mood.Title {
				s.playlists, s.picked, s.tracks = nil, nil, nil
				s.plErr, s.tracksErr = "", ""
			}
			s.mood = mood
			s.save()
			if onWeather != nil {
				onWeather()
			}
			s.ensurePlaylists(post)
		})
	}()
}

// ensureShot asks for the day's photographs, once a few hours, and picks
// one. The photographs it has show meanwhile.
func (s *wxState) ensureShot(post func(func())) {
	if s.shotAsk {
		return
	}
	if len(s.shots) > 0 && time.Since(s.shotAt) < shotsStale {
		return
	}
	s.shotAsk = true
	go func() {
		shots, err := wxShots()
		post(func() {
			s.shotAsk = false
			if err != nil || len(shots) == 0 {
				return
			}
			s.shots, s.shotAt = shots, time.Now()
			if s.rollNext {
				s.rollNext = false
				s.otherShot()
			} else if s.shot.URL == "" {
				s.pickShot()
			}
			s.warmShots()
			s.save()
		})
	}()
}

// warmShots starts the day's photographs loading before the page needs
// them, so that opening it shows one at once.
func (s *wxState) warmShots() {
	if s.art == nil {
		return
	}
	if s.shot.URL != "" {
		s.art.get(s.shot.URL, false)
	}
	for i := 0; i < len(s.shots) && i < 4; i++ {
		s.art.get(s.shots[i].URL, false)
	}
}

// rollShot takes another photograph, for a page about to open: another
// one every time, never the one that showed before.
func (s *wxState) rollShot(post func(func())) {
	if len(s.shots) < 2 {
		s.rollNext = true
		s.ensureShot(post)
		return
	}
	s.otherShot()
	s.warmShots()
	s.save()
}

// pickShot throws the dice over the day's photographs, and takes the
// first that is not one the page turned down for being too bright.
func (s *wxState) pickShot() {
	if len(s.shots) == 0 {
		return
	}
	n := s.rng.Intn(len(s.shots))
	for i := 0; i < len(s.shots); i++ {
		cand := s.shots[(n+i)%len(s.shots)]
		if !s.washed[cand.URL] {
			s.shot, s.shotTry = cand, 0
			return
		}
	}
	s.shot, s.shotTry = s.shots[n], 0
}

// wash turns a photograph down for being too bright to read a report
// over, and takes another in its place.
func (s *wxState) wash(url string) {
	s.washed[url] = true
	s.pickShot()
}

// otherShot takes another photograph, when the listener asks for one,
// or the page opens again: never the one that showed before.
func (s *wxState) otherShot() {
	if len(s.shots) < 2 {
		return
	}
	cur := s.shot.URL
	for i := 0; i < 6; i++ {
		s.pickShot()
		if s.shot.URL != cur {
			return
		}
	}
}

// ensurePlaylists asks the catalogue for the mood's playlists, over a
// keyword thrown like a dice, unless they are in and fresh.
func (s *wxState) ensurePlaylists(post func(func())) {
	if s.plAsking || len(s.playlists) > 0 || s.plErr != "" {
		return
	}
	mood := s.mood
	if len(mood.Keywords) == 0 {
		return
	}
	kw := mood.Keywords[s.rng.Intn(len(mood.Keywords))]
	s.plAsking, s.seekKeyword = true, kw
	go func() {
		res, err := netease.SearchPage(kw, netease.SearchPlaylists, 16, 0)
		post(func() {
			s.plAsking = false
			if err != nil {
				s.plErr = "歌单拿不到：" + err.Error()
				return
			}
			if len(res.Playlists) == 0 {
				s.plErr = "没有找到合天气的歌单"
				return
			}
			s.playlists = wxChinese(res.Playlists)
			s.pickPlaylist()
			s.save()
			s.ensureTracks(post)
		})
	}()
}

// pickPlaylist throws the dice again over the mood's playlists, and
// asks the winner for its songs.
func (s *wxState) pickPlaylist() {
	if len(s.playlists) == 0 {
		return
	}
	n := s.rng.Intn(len(s.playlists))
	// The same playlist twice in a row is no new weather; throw again.
	if s.picked != nil && len(s.playlists) > 1 && s.playlists[n].ID == s.picked.ID {
		n = (n + 1) % len(s.playlists)
	}
	s.picked = &s.playlists[n]
	s.tracks, s.tracksErr = nil, ""
}

// ensureTracks asks for the picked playlist's songs, if they are not in.
func (s *wxState) ensureTracks(post func(func())) {
	if s.picked == nil || s.tracksAsk || len(s.tracks) > 0 || s.tracksErr != "" {
		return
	}
	id := s.picked.ID
	s.tracksAsk = true
	go func() {
		pl, err := netease.PlaylistDetail(id)
		post(func() {
			s.tracksAsk = false
			if s.picked == nil || s.picked.ID != id {
				return
			}
			if err != nil {
				s.tracksErr = "歌曲拿不到：" + err.Error()
				return
			}
			s.tracks = wxChineseTracks(pl.Tracks)
			if len(s.tracks) > 40 {
				s.tracks = s.tracks[:40]
			}
			s.save()
		})
	}()
}

// refresh throws the whole sky away and asks again: the page's own
// button, for when the listener doubts the report.
func (s *wxState) refresh(post func(func()), onWeather func()) {
	s.playlists, s.plErr, s.picked, s.tracks, s.tracksErr = nil, "", nil, nil, ""
	s.data, s.at = nil, time.Time{}
	s.ensure(post, onWeather)
}
