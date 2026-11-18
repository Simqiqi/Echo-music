// Playlist reads for the signed-in QQ 音乐 account, through the bridge.
package qq

import "github.com/simqiqi/simqiqi-music/internal/sidecar"

// Playlists lists the account's playlists (made and collected), limit from
// offset. Nothing without a session.
func Playlists(limit, offset int) ([]sidecar.BridgePlaylist, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, err
	}
	return c.Playlists("qq", limit, offset)
}

// PlaylistTracks lists a playlist's tracks, limit from offset. The bool
// says whether more follow, next where the next page begins.
func PlaylistTracks(id string, limit, offset int) ([]sidecar.BridgeTrack, bool, int, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, false, 0, err
	}
	return c.PlaylistTracks("qq", id, limit, offset)
}

// TrackURL resolves where a track streams. quality is one of 128, 320, flac;
// QQ serves what the account is entitled to, which may be lower.
func TrackURL(t sidecar.BridgeTrack, quality string) (*sidecar.BridgeStream, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, err
	}
	return c.TrackURL("qq", t, quality)
}

// Lyrics fetches a track's lyrics, as LRC the player parses.
func Lyrics(t sidecar.BridgeTrack) (string, string, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return "", "", err
	}
	return c.Lyrics("qq", t)
}
