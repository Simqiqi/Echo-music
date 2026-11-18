package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// A playlist is carried between machines as one JSON file: the name, when
// it was written, and the tracks themselves in the shape NetEase's own
// answers use, so a file this app wrote reads back exactly. Import also
// takes plain arrays of tracks, files that name only song ids, and M3U
// playlists whose lines are NetEase song links.
//
// Import never writes to the account's playlists — it has no business
// changing what the listener keeps online. What it brings in is added to
// the end of the queue, leaving the song that plays alone.

type playlistFile struct {
	Name       string          `json:"name"`
	ExportedAt string          `json:"exportedAt"`
	Tracks     []netease.Track `json:"tracks"`
}

var songIDInURL = regexp.MustCompile(`(?:[?&]id=|/song/)(\d+)`)

// exportPlaylist writes the playlist on screen, or the queue when no
// playlist is open, to a file the listener names.
func (a *app) exportPlaylist() {
	name, tracks := a.playlistToExport()
	if len(tracks) == 0 {
		a.toast = "没有可导出的歌曲"
		a.redraw()
		return
	}
	a.exportTracks(name, tracks)
}

// playlistToExport prefers the playlist the listener is looking at; away
// from one, the queue is what a playlist means here.
func (a *app) playlistToExport() (string, []netease.Track) {
	if a.route.page == pagePlaylist {
		if pl := a.playlists[a.route.id]; pl != nil && len(pl.Tracks) > 0 {
			return pl.Name, pl.Tracks
		}
	}
	return "播放队列", a.pl.Queue()
}

// exportTracks asks for a path and writes the file. The dialog blocks, so
// it runs off the UI thread and reports back through post.
func (a *app) exportTracks(name string, tracks []netease.Track) {
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:       a.win,
			Title:        "导出歌单",
			DefaultPath:  safeFileName(name) + ".json",
			NameFieldLabel: "文件名",
			Filters:      []mygo.FileFilter{{Name: "歌单 JSON", Extensions: []string{"json"}}},
		})
		if err != nil || path == "" {
			return
		}
		if filepath.Ext(path) == "" {
			path += ".json"
		}
		doc := playlistFile{
			Name:       name,
			ExportedAt: time.Now().Format(time.RFC3339),
			Tracks:     tracks,
		}
		data, err := json.MarshalIndent(doc, "", "  ")
		if err == nil {
			err = os.WriteFile(path, data, 0o644)
		}
		a.post(func() {
			if err != nil {
				a.toast = "导出失败：" + err.Error()
			} else {
				a.toast = "已导出 " + strconv.Itoa(len(tracks)) + " 首到 " + filepath.Base(path)
			}
			a.redraw()
		})
	}()
}

// importPlaylist reads a playlist file and adds what it holds to the queue.
func (a *app) importPlaylist() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  a.win,
			Title:   "导入歌单",
			Message: "支持本应用导出的 JSON、仅含歌曲 ID 的 JSON，以及 M3U 歌单",
			Filters: []mygo.FileFilter{
				{Name: "歌单文件", Extensions: []string{"json", "m3u", "m3u8"}},
			},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			a.post(func() { a.toast = "读取失败：" + err.Error(); a.redraw() })
			return
		}
		tracks, ids, err := parsePlaylist(data, filepath.Ext(paths[0]))
		if err != nil {
			a.post(func() { a.toast = "不是可识别的歌单：" + err.Error(); a.redraw() })
			return
		}
		// A file that only names ids needs the songs' names and artists
		// before it is worth putting in the queue.
		if len(ids) > 0 {
			more, derr := netease.SongDetails(ids)
			if derr != nil {
				a.post(func() { a.toast = "歌曲信息获取失败：" + derr.Error(); a.redraw() })
				return
			}
			tracks = append(tracks, more...)
		}
		tracks = uniqueTracks(tracks)
		if len(tracks) == 0 {
			a.post(func() { a.toast = "文件里没有可用的歌曲"; a.redraw() })
			return
		}
		count := len(tracks)
		a.post(func() {
			a.pl.Enqueue(tracks)
			a.toast = "已导入 " + strconv.Itoa(count) + " 首到播放队列"
			a.redraw()
		})
	}()
}

// parsePlaylist reads the shapes an imported file may take, reporting
// tracks that are already complete and ids that still need looking up.
func parsePlaylist(data []byte, ext string) ([]netease.Track, []int64, error) {
	trimmed := strings.TrimSpace(string(data))
	if ext == ".m3u" || ext == ".m3u8" || strings.HasPrefix(trimmed, "#EXTM3U") {
		ids := parseM3U(trimmed)
		if len(ids) == 0 {
			return nil, nil, errors.New("M3U 里没有网易云歌曲链接")
		}
		return nil, ids, nil
	}

	// An object written by this app, or one that only names song ids.
	// Both may appear in one file, so their ids are gathered together.
	var obj struct {
		IDs    []int64           `json:"ids"`
		Tracks []json.RawMessage `json:"tracks"`
	}
	if err := json.Unmarshal(data, &obj); err == nil && (len(obj.Tracks) > 0 || len(obj.IDs) > 0) {
		var found []netease.Track
		for _, raw := range obj.Tracks {
			var tr netease.Track
			if json.Unmarshal(raw, &tr) == nil {
				found = append(found, tr)
			}
		}
		full, missing := splitTracks(found)
		return full, append(append([]int64(nil), obj.IDs...), missing...), nil
	}
	// A bare array of tracks.
	var arr []netease.Track
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		full, missing := splitTracks(arr)
		return full, missing, nil
	}
	return nil, nil, errors.New("找不到歌曲")
}

// splitTracks keeps complete tracks and pulls out the ones that are only
// an id, so they may be filled in from the network.
func splitTracks(tracks []netease.Track) ([]netease.Track, []int64) {
	var full []netease.Track
	var ids []int64
	for _, t := range tracks {
		if t.ID <= 0 {
			continue
		}
		if t.Name == "" {
			ids = append(ids, t.ID)
			continue
		}
		full = append(full, t)
	}
	return full, ids
}

// parseM3U takes the ids out of NetEase song links on the file's lines.
func parseM3U(text string) []int64 {
	seen := map[int64]bool{}
	var ids []int64
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := songIDInURL.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id, err := strconv.ParseInt(m[1], 10, 64)
		if err == nil && id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// uniqueTracks drops repeated ids, keeping the first of each.
func uniqueTracks(tracks []netease.Track) []netease.Track {
	seen := map[int64]bool{}
	out := make([]netease.Track, 0, len(tracks))
	for _, t := range tracks {
		if t.ID <= 0 || seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		out = append(out, t)
	}
	return out
}

// safeFileName keeps a playlist's name from escaping the folder it is
// saved into.
func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.NewReplacer("/", "-", "\\", "-", ":", "-", "\n", " ", "\r", "").Replace(name)
	if name == "" || name == "." || name == ".." {
		return "歌单"
	}
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}
