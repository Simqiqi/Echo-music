package app

import (
	"strings"
	"testing"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

func TestParsePlaylistWrittenByThisApp(t *testing.T) {
	data := []byte(`{"name":"夜里的歌","exportedAt":"2026-11-14T00:00:00Z","tracks":[{"id":42,"name":"不将就","ar":[{"name":"李荣浩"}],"dt":240000}]}`)
	tracks, ids, err := parsePlaylist(data, ".json")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 || len(tracks) != 1 {
		t.Fatalf("got %d tracks and %d ids, want 1 track", len(tracks), len(ids))
	}
	if tracks[0].ID != 42 || tracks[0].Name != "不将就" {
		t.Errorf("the track was not read back: %+v", tracks[0])
	}
}

func TestParsePlaylistBareArray(t *testing.T) {
	data := []byte(`[{"id":1,"name":"A"},{"id":2,"name":"B"}]`)
	tracks, _, err := parsePlaylist(data, ".json")
	if err != nil || len(tracks) != 2 {
		t.Fatalf("bare array: %d tracks, %v", len(tracks), err)
	}
}

func TestParsePlaylistOnlyIDs(t *testing.T) {
	data := []byte(`{"ids":[11,22],"tracks":[{"id":33}]}`)
	tracks, ids, err := parsePlaylist(data, ".json")
	if err != nil || len(tracks) != 0 || len(ids) != 3 {
		t.Fatalf("id-only file: %d tracks, %d ids, %v", len(tracks), len(ids), err)
	}
	if ids[0] != 11 || ids[2] != 33 {
		t.Errorf("ids came back out of order: %v", ids)
	}
}

func TestParsePlaylistM3U(t *testing.T) {
	text := "#EXTM3U\n" +
		"#EXTINF:240,不将就\n" +
		"https://music.163.com/song?id=123\n" +
		"https://music.163.com/song?id=123\n" +
		"https://music.163.com/song/456\n" +
		"#not-a-link\n"
	for _, ext := range []string{".m3u", ".m3u8", ""} {
		_, ids, err := parsePlaylist([]byte(text), ext)
		if err != nil || len(ids) != 2 || ids[0] != 123 || ids[1] != 456 {
			t.Errorf("ext %q: ids %v, err %v", ext, ids, err)
		}
	}
}

func TestParsePlaylistRejectsGarbage(t *testing.T) {
	if _, _, err := parsePlaylist([]byte("not a playlist"), ".json"); err == nil {
		t.Error("garbage was accepted as a playlist")
	}
	if _, _, err := parsePlaylist([]byte("#EXTM3U\nno links here\n"), ".m3u"); err == nil {
		t.Error("an M3U with no song links was accepted")
	}
}

func TestUniqueTracksAndSafeFileName(t *testing.T) {
	tracks := []netease.Track{{ID: 1, Name: "A"}, {ID: 1, Name: "A again"}, {ID: 0}, {ID: 2, Name: "B"}}
	got := uniqueTracks(tracks)
	if len(got) != 2 || got[0].Name != "A" || got[1].ID != 2 {
		t.Errorf("uniqueTracks kept %+v", got)
	}
	if name := safeFileName(" 深夜/电台: 02 "); strings.ContainsAny(name, `/\:`) {
		t.Errorf("unsafe characters survived in %q", name)
	}
	if safeFileName("") != "歌单" {
		t.Error("an empty name did not fall back")
	}
}

// TestEnqueueKeepsWhatPlays: an imported playlist joins the end of the
// queue without replacing the song that was already there.
func TestEnqueueKeepsWhatPlays(t *testing.T) {
	a := newTestApp()
	first := netease.Track{ID: 1, Name: "Already queued"}
	a.pl.Cue([]netease.Track{first})
	a.pl.PlayQueue([]netease.Track{first}, 0)
	a.pl.Enqueue([]netease.Track{first, {ID: 2, Name: "Imported"}})
	q := a.pl.Queue()
	if len(q) != 2 || q[0].ID != 1 || q[1].ID != 2 {
		t.Errorf("queue after import: %+v", q)
	}
	if cur := a.pl.Current(); cur == nil || cur.ID != 1 {
		t.Error("importing changed the song that plays")
	}
}
