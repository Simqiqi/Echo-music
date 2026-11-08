package sidecar

import (
	"fmt"
	"time"
)

// BridgePlaylist is a 汽水/酷狗 playlist as the bridge reports it. ID is
// the provider's own, as a string: it need not fit an int64.
type BridgePlaylist struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Cover   string `json:"cover"`
	Count   int    `json:"count"`
	IsOwned bool   `json:"isOwned"`
}

// BridgeTrack is a track of those playlists. 汽水 plays by its ID alone;
// 酷狗 also needs the Hash and AlbumAudioID the track list carries.
type BridgeTrack struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Artist       string `json:"artist"`
	Album        string `json:"album"`
	Duration     int64  `json:"duration"` // milliseconds
	Cover        string `json:"cover"`
	Hash         string `json:"hash"`
	AlbumAudioID string `json:"albumAudioId"`
}

// Playlists lists the signed-in account's playlists, limit from offset.
func (c *Client) Playlists(provider string, limit, offset int) ([]BridgePlaylist, error) {
	var r struct {
		Items []BridgePlaylist `json:"items"`
	}
	body := map[string]int{"limit": limit, "offset": offset}
	if err := c.call(provider, "playlists", body, &r); err != nil {
		return nil, err
	}
	return r.Items, nil
}

// PlaylistTracks lists a playlist's tracks, limit from offset. The bool
// says whether more follow, next where the next page begins.
func (c *Client) PlaylistTracks(provider, id string, limit, offset int) (items []BridgeTrack, hasMore bool, next int, err error) {
	var r struct {
		Items      []BridgeTrack `json:"items"`
		HasMore    bool          `json:"hasMore"`
		NextOffset int           `json:"nextOffset"`
	}
	body := map[string]any{"id": id, "limit": limit, "offset": offset}
	if err := c.call(provider, "playlist", body, &r); err != nil {
		return nil, false, 0, err
	}
	return r.Items, r.HasMore, r.NextOffset, nil
}

// BridgeStream is where a bridge track streams from, and in which format.
// Type is mp3 or flac, the two the player decodes: 汽水's AAC comes back
// converted to FLAC by the bridge, ready to stream from the bridge's own
// port. Preview says the platform served an excerpt.
type BridgeStream struct {
	URL     string `json:"url"`
	Type    string `json:"type"`
	Quality string `json:"quality"`
	Preview bool   `json:"preview"`
}

// trackURLWait is how long resolving a play address may take: the bridge
// downloads the song to serve it, so a long one on a slow line still lands.
const trackURLWait = 3 * time.Minute

// TrackURL resolves where a bridge track streams from. quality is one of
// standard, high, lossless, hires for 汽水, or 128, 320, flac for 酷狗;
// what comes back may be lower.
func (c *Client) TrackURL(provider string, t BridgeTrack, quality string) (*BridgeStream, error) {
	var r struct {
		BridgeStream
		Error string `json:"error"`
	}
	body := map[string]string{
		"id":           t.ID,
		"hash":         t.Hash,
		"albumAudioId": t.AlbumAudioID,
		"quality":      quality,
	}
	// 桥接取地址时要把整首歌拉回本地（必要时再转码），等它比等别的事久。
	if err := c.callWithin(provider, "url", body, &r, trackURLWait); err != nil {
		return nil, err
	}
	if r.Error != "" {
		return nil, fmt.Errorf("%s", r.Error)
	}
	if r.URL == "" {
		return nil, fmt.Errorf("这首歌暂时无法播放")
	}
	if r.Type == "" {
		r.Type = "mp3"
	}
	out := r.BridgeStream
	return &out, nil
}

// BridgeLyrics is a track's lyrics as the bridge serves them. Format is
// "lrc" (timed lines) or "yrc" (word-timed, NetEase's own shape); the
// player reads both.
type BridgeLyrics struct {
	Lrc    string `json:"lrc"`
	Format string `json:"format"`
}

// Lyrics fetches a bridge track's lyrics. 汽水 keeps them by ID alone;
// 酷狗 wants the Hash and AlbumAudioID, and matches by duration and
// keyword when those do not settle it.
func (c *Client) Lyrics(provider string, t BridgeTrack) (string, string, error) {
	var r struct {
		BridgeLyrics
		Error string `json:"error"`
	}
	body := map[string]string{
		"id":           t.ID,
		"hash":         t.Hash,
		"albumAudioId": t.AlbumAudioID,
		"name":         t.Name,
		"artist":       t.Artist,
	}
	if err := c.call(provider, "lyric", body, &r); err != nil {
		return "", "", err
	}
	if r.Error != "" {
		return "", "", fmt.Errorf("%s", r.Error)
	}
	if r.Lrc == "" {
		return "", "", fmt.Errorf("这首歌没有歌词")
	}
	format := r.Format
	if format == "" {
		format = "lrc"
	}
	return r.Lrc, format, nil
}
