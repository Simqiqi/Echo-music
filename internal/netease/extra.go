package netease

import (
	"net/url"
	"strconv"
	"time"
)

// The corners of NetEase beyond the playlists and songs: the feed of who
// the listener follows, and the albums just out.

// FollowedEvents returns the events from the listener's friends and the
// artists they follow, newest first, up to limit. lastTime is the time in
// Unix milliseconds of the last event taken before, or 0 for the newest; it
// returns the time to go on from and whether there are more.
func FollowedEvents(limit, lastTime int64) (events []Event, last int64, more bool, err error) {
	var res struct {
		Events []struct {
			Event
			JSON string `json:"json"`
		} `json:"events"`
		LastTime int64 `json:"lasttime"`
		More     bool  `json:"more"`
	}
	q := url.Values{
		"pagesize": {strconv.FormatInt(limit, 10)},
		"lasttime": {strconv.FormatInt(lastTime, 10)},
	}
	if err := get("/api/event", q, &res); err != nil {
		return nil, 0, false, err
	}
	for _, e := range res.Events {
		e.Content = decodeEvent(e.JSON)
		events = append(events, e.Event)
	}
	return events, res.LastTime, res.More, nil
}

// NewAlbum is an album just released, as the new-albums page lists it.
type NewAlbum struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	PicURL      string `json:"picUrl"`
	ArtistName  string `json:"-"`
	PublishTime int64  `json:"publishTime"` // Unix milliseconds
}

// Published returns when the album came out.
func (a NewAlbum) Published() time.Time { return time.UnixMilli(a.PublishTime) }

// NewAlbums returns up to limit of the albums most recently released.
func NewAlbums(limit int) ([]NewAlbum, error) {
	var res struct {
		Albums []struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			PicURL      string `json:"picUrl"`
			PublishTime int64  `json:"publishTime"`
			Artist      struct {
				Name string `json:"name"`
			} `json:"artist"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
		} `json:"albums"`
	}
	if err := get("/api/album/newest", url.Values{"limit": {strconv.Itoa(limit)}}, &res); err != nil {
		return nil, err
	}
	out := make([]NewAlbum, 0, len(res.Albums))
	for _, a := range res.Albums {
		name := a.Artist.Name
		if name == "" && len(a.Artists) > 0 {
			name = a.Artists[0].Name
		}
		out = append(out, NewAlbum{
			ID:          a.ID,
			Name:        a.Name,
			PicURL:      a.PicURL,
			ArtistName:  name,
			PublishTime: a.PublishTime,
		})
	}
	return out, nil
}
