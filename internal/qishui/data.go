// Playlist reads for the signed-in 汽水音乐 account, through the bridge.
package qishui

import "github.com/simqiqi/simqiqi-music/internal/sidecar"

// Playlists lists the account's playlists (made and collected), limit from
// offset. Nothing without a session.
func Playlists(limit, offset int) ([]sidecar.BridgePlaylist, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, err
	}
	return c.Playlists("qishui", limit, offset)
}

// PlaylistTracks lists a playlist's tracks, limit from offset. The bool
// says whether more follow, next where the next page begins.
func PlaylistTracks(id string, limit, offset int) ([]sidecar.BridgeTrack, bool, int, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, false, 0, err
	}
	return c.PlaylistTracks("qishui", id, limit, offset)
}

// TrackURL resolves where a track streams. quality is one of standard,
// high, lossless, hires; what comes back may be lower.
func TrackURL(t sidecar.BridgeTrack, quality string) (*sidecar.BridgeStream, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, err
	}
	return c.TrackURL("qishui", t, quality)
}

// Lyrics fetches a track's word-timed lyrics, as YRC the player parses.
func Lyrics(t sidecar.BridgeTrack) (string, string, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return "", "", err
	}
	return c.Lyrics("qishui", t)
}
