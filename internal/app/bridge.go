package app

// 汽水音乐的歌曲与歌单，在应用内以负数的合成 ID 出现：歌单页、
// 播放队列与网易云共用一套数据结构（netease.Playlist / netease.Track），
// 而它们的真实凭据（平台、各自的 ID）记在这里，由注册表把
// 合成 ID 翻译回去。负数永不与网易云的 ID 相撞；播放器认得负数，向
// Source（bridgeSource）要播放地址，而不是问网易云。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/kugou"
	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/qq"
	"github.com/simqiqi/simqiqi-music/internal/qishui"
	"github.com/simqiqi/simqiqi-music/internal/sidecar"
)

// bridgeProviders are the sidecar services the app can sign into and list
// playlists from, in the order the sidebar and the login dialog show them.
var bridgeProviders = []providerID{provQishui, provKugou, provQQ}

// bridgeAccount is one bridge provider's state: who is signed in there, the
// playlists the sidebar lists for them, and why those failed to load.
type bridgeAccount struct {
	acc       *sidecar.Account         // nil 表示这家没登录
	lists     []sidecar.BridgePlaylist // 账号的歌单，侧栏列出
	plErr     string                   // 歌单加载失败的原因
	collapsed bool                     // 侧栏里这一组的折叠状态
}

// bridgeOf returns a bridge provider's state, making it on first use.
func (a *app) bridgeOf(prov providerID) *bridgeAccount {
	if a.bridgeAcc == nil {
		a.bridgeAcc = map[providerID]*bridgeAccount{}
	}
	st := a.bridgeAcc[prov]
	if st == nil {
		st = &bridgeAccount{}
		a.bridgeAcc[prov] = st
	}
	return st
}

// bridgeAccountOf reports who a bridge provider is signed in as, or nil when
// nobody is. Reading through this keeps the views from having to know whether
// the map has been touched yet.
func (a *app) bridgeAccountOf(prov providerID) *sidecar.Account {
	if st := a.bridgeAcc[prov]; st != nil {
		return st.acc
	}
	return nil
}

// bridgeList is a playlist of a sidecar provider, as the registry keeps it.
type bridgeList struct {
	Prov  providerID `json:"prov"`
	ID    string     `json:"id"` // the provider's own id
	Name  string     `json:"name"`
	Cover string     `json:"cover"`
}

// bridgeStore holds what the synthetic IDs stand for. The player resolves
// streams on its own goroutines while the views assign IDs on the main
// one, so everything goes through mu.
type bridgeStore struct {
	mu       sync.Mutex
	next     int64                         // the next ID to hand out, going down
	lists    map[int64]bridgeList          // synthetic playlist ID → the list
	listKey  map[string]int64              // provider and its id → the same, so IDs hold across reloads
	songs    map[int64]sidecar.BridgeTrack // synthetic track ID → the track
	songKey  map[string]int64              // provider, its id and hash → the same
	songProv map[int64]providerID          // synthetic track ID → its provider
}

// newBridgeStore is an empty registry.
func newBridgeStore() *bridgeStore {
	b := &bridgeStore{}
	b.init()
	return b
}

func (b *bridgeStore) init() {
	b.next = -1
	b.lists = map[int64]bridgeList{}
	b.listKey = map[string]int64{}
	b.songs = map[int64]sidecar.BridgeTrack{}
	b.songKey = map[string]int64{}
	b.songProv = map[int64]providerID{}
}

func bridgeKey(prov providerID, id string) string {
	return fmt.Sprintf("%d/%s", int(prov), id)
}

// listID returns the synthetic ID of a bridge playlist, assigning one if
// it is new.
func (b *bridgeStore) listID(prov providerID, id, name, cover string) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := bridgeKey(prov, id)
	if n, ok := b.listKey[k]; ok {
		if l, ok := b.lists[n]; ok {
			l.Name, l.Cover = name, cover
			b.lists[n] = l
		}
		return n
	}
	n := b.next
	b.next--
	b.listKey[k] = n
	b.lists[n] = bridgeList{Prov: prov, ID: id, Name: name, Cover: cover}
	return n
}

// trackID returns the synthetic ID of a bridge track, assigning one if it
// is new. 取播放地址的凭据（含 hash）一并进键里。
func (b *bridgeStore) trackID(prov providerID, t sidecar.BridgeTrack) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := bridgeKey(prov, t.ID+"|"+t.Hash)
	if n, ok := b.songKey[k]; ok {
		return n
	}
	n := b.next
	b.next--
	b.songKey[k] = n
	b.songs[n] = t
	b.songProv[n] = prov
	return n
}

// providerOf returns which service a synthetic song ID belongs to.
func (b *bridgeStore) providerOf(id int64) providerID {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.songProv[id]
}

// bridgeRows are the sidebar rows bridgePlaylistVMs last built for an
// account, and the lists it built them from.
type bridgeRows struct {
	lists []sidecar.BridgePlaylist
	vms   []netease.Playlist
}

// bridgePlaylistVMs turns an account's playlists into the sidebar's rows,
// handing each its synthetic ID. The sidebar asks every frame, and each
// ID taken goes through the registry's lock, so the rows are built once
// per change of the lists rather than once per frame.
func (a *app) bridgePlaylistVMs(prov providerID, lists []sidecar.BridgePlaylist) []netease.Playlist {
	if a.bridge == nil {
		return nil // a test's bare app: no registry to hand IDs out of
	}
	if a.bridgeVMs == nil {
		a.bridgeVMs = map[providerID]*bridgeRows{}
	}
	if rows := a.bridgeVMs[prov]; rows != nil && slices.Equal(rows.lists, lists) {
		return rows.vms
	}
	out := make([]netease.Playlist, 0, len(lists))
	for _, l := range lists {
		out = append(out, netease.Playlist{
			ID:         a.bridge.listID(prov, l.ID, l.Name, l.Cover),
			Name:       l.Name,
			CoverURL:   l.Cover,
			TrackCount: l.Count,
		})
	}
	a.bridgeVMs[prov] = &bridgeRows{lists: slices.Clone(lists), vms: out}
	return out
}

// bridgeListsSection lists an account's playlists in the sidebar, under
// its own name, once they have come in; a failure says so quietly.
func (a *app) bridgeListsSection(c *ui.Context, prov providerID, title string, lists []sidecar.BridgePlaylist, errMsg string, collapsed *bool) {
	if len(lists) == 0 && errMsg == "" {
		return // 还在取，或这家没有登录
	}
	a.myPlaylists(c, title, a.bridgePlaylistVMs(prov, lists), false, collapsed)
	if errMsg != "" {
		ui.Text(c, errMsg).FontSize(11).TextColor(a.pal.faint).Margin(0, 0, 6, 6).SingleLine()
	}
}

// loadBridgeLists fetches an account's playlists for the sidebar, once it
// has signed in or come back from the keychain.
func (a *app) loadBridgeLists(prov providerID) {
	go func() {
		lists, err := bridgeAPI(prov).Playlists(100, 0)
		a.post(func() {
			msg := ""
			if err != nil {
				msg = friendlyBridgeErr(err)
				lists = nil
			}
			st := a.bridgeOf(prov)
			st.lists, st.plErr = lists, msg
		})
	}()
}

// bridgeAPI names the provider's playlist reads.
func bridgeAPI(prov providerID) interface {
	Playlists(limit, offset int) ([]sidecar.BridgePlaylist, error)
	PlaylistTracks(id string, limit, offset int) ([]sidecar.BridgeTrack, bool, int, error)
	TrackURL(t sidecar.BridgeTrack, quality string) (*sidecar.BridgeStream, error)
	Lyrics(t sidecar.BridgeTrack) (string, string, error)
} {
	switch prov {
	case provKugou:
		return kugouAPI{}
	case provQQ:
		return qqAPI{}
	default:
		return qishuiAPI{}
	}
}

type qishuiAPI struct{}

func (qishuiAPI) Playlists(limit, offset int) ([]sidecar.BridgePlaylist, error) {
	return qishui.Playlists(limit, offset)
}
func (qishuiAPI) PlaylistTracks(id string, limit, offset int) ([]sidecar.BridgeTrack, bool, int, error) {
	return qishui.PlaylistTracks(id, limit, offset)
}
func (qishuiAPI) TrackURL(t sidecar.BridgeTrack, quality string) (*sidecar.BridgeStream, error) {
	return qishui.TrackURL(t, quality)
}
func (qishuiAPI) Lyrics(t sidecar.BridgeTrack) (string, string, error) {
	return qishui.Lyrics(t)
}

type kugouAPI struct{}

func (kugouAPI) Playlists(limit, offset int) ([]sidecar.BridgePlaylist, error) {
	return kugou.Playlists(limit, offset)
}
func (kugouAPI) PlaylistTracks(id string, limit, offset int) ([]sidecar.BridgeTrack, bool, int, error) {
	return kugou.PlaylistTracks(id, limit, offset)
}
func (kugouAPI) TrackURL(t sidecar.BridgeTrack, quality string) (*sidecar.BridgeStream, error) {
	return kugou.TrackURL(t, quality)
}
func (kugouAPI) Lyrics(t sidecar.BridgeTrack) (string, string, error) {
	return kugou.Lyrics(t)
}

type qqAPI struct{}

func (qqAPI) Playlists(limit, offset int) ([]sidecar.BridgePlaylist, error) {
	return qq.Playlists(limit, offset)
}
func (qqAPI) PlaylistTracks(id string, limit, offset int) ([]sidecar.BridgeTrack, bool, int, error) {
	return qq.PlaylistTracks(id, limit, offset)
}
func (qqAPI) TrackURL(t sidecar.BridgeTrack, quality string) (*sidecar.BridgeStream, error) {
	return qq.TrackURL(t, quality)
}
func (qqAPI) Lyrics(t sidecar.BridgeTrack) (string, string, error) {
	return qq.Lyrics(t)
}

// bridgeSong looks up what a synthetic song ID stands for: its track and
// its provider. One lock for both maps — calling providerOf inside would
// lock twice, and sync.Mutex does not nest.
func (a *app) bridgeSong(id int64) (sidecar.BridgeTrack, providerID, bool) {
	if a.bridge == nil {
		return sidecar.BridgeTrack{}, provNetease, false
	}
	a.bridge.mu.Lock()
	defer a.bridge.mu.Unlock()
	bt, ok := a.bridge.songs[id]
	return bt, a.bridge.songProv[id], ok
}

// bridgeLyrics is the player's BridgeLyrics: where a negative-ID track's
// words come from. It runs on the player's loading goroutines.
func (a *app) bridgeLyrics(t netease.Track) (string, string, error) {
	bt, prov, ok := a.bridgeSong(t.ID)
	if !ok {
		return "", "", fmt.Errorf("这首歌没有歌词")
	}
	return bridgeAPI(prov).Lyrics(bt)
}

// friendlyBridgeErr says what went with the bridge in the app's own words.
func friendlyBridgeErr(err error) string {
	msg := err.Error()
	if msg == "" {
		return "加载失败"
	}
	return msg
}

// loadBridgePlaylist fills a.playlists[id] with a bridge playlist's tracks,
// as the playlist page and the player both read them.
func (a *app) loadBridgePlaylist(id int64) {
	if a.bridge == nil || a.playlists[id] != nil || a.loadingPl[id] {
		return
	}
	a.bridge.mu.Lock()
	list, ok := a.bridge.lists[id]
	a.bridge.mu.Unlock()
	if !ok {
		return
	}
	a.loadingPl[id] = true
	delete(a.plErr, id)
	prov, realID := list.Prov, list.ID
	go func() {
		var all []sidecar.BridgeTrack
		var err error
		offset := 0
		for page := 0; page < 20; page++ { // 20 页 = 2000 首，足够一份歌单
			var items []sidecar.BridgeTrack
			var hasMore bool
			var next int
			items, hasMore, next, err = bridgeAPI(prov).PlaylistTracks(realID, 100, offset)
			if err != nil {
				break
			}
			all = append(all, items...)
			if !hasMore || next <= offset {
				break
			}
			offset = next
		}
		a.post(func() {
			delete(a.loadingPl, id)
			if err != nil {
				a.plErr[id] = "歌单加载失败：" + friendlyBridgeErr(err)
				return
			}
			tracks := make([]netease.Track, 0, len(all))
			for _, t := range all {
				tracks = append(tracks, a.bridgeTrack(prov, t))
			}
			a.playlists[id] = &netease.Playlist{
				ID:         id,
				Name:       list.Name,
				CoverURL:   list.Cover,
				TrackCount: len(tracks),
				Tracks:     tracks,
			}
			a.playlists[id].Creator.Nickname = prov.name()
			a.saveBridgeSongs()
		})
	}()
}

// bridgeTrack turns a bridge track into what the queue and the pages show,
// under its synthetic ID.
func (a *app) bridgeTrack(prov providerID, t sidecar.BridgeTrack) netease.Track {
	return netease.Track{
		ID:       a.bridge.trackID(prov, t),
		Name:     t.Name,
		Artists:  []netease.Artist{{Name: t.Artist}},
		Album:    netease.Album{Name: t.Album, PicURL: t.Cover},
		Duration: t.Duration,
	}
}

// bridgeQuality maps the settings' stream level onto what each source calls
// it, so a song plays under any pick. 汽水音乐 names its rungs
// standard/high/lossless/hires; 酷狗 and QQ 音乐 count bitrates instead.
func bridgeQuality(prov providerID, level string) string {
	switch prov {
	case provKugou, provQQ:
		switch level {
		case "lossless", "hires":
			return "flac"
		case "higher", "exhigh":
			return "320"
		}
		return "128"
	}
	switch level {
	case "hires":
		return "hires"
	case "lossless":
		return "lossless"
	case "higher", "exhigh":
		return "high"
	}
	return "standard"
}

// bridgeSource is the player's Source: where a negative-ID track streams.
// It runs on the player's loading goroutines.
func (a *app) bridgeSource(id int64, level string) (*netease.Stream, error) {
	t, prov, ok := a.bridgeSong(id)
	if !ok {
		return nil, fmt.Errorf("%w: 不知道这首歌是谁的", netease.ErrUnavailable)
	}
	got, err := bridgeAPI(prov).TrackURL(t, bridgeQuality(prov, level))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", netease.ErrUnavailable, err)
	}
	// The bridge says which container it served, and — where the source names
	// its rungs — which one it settled on: 酷狗 and QQ 音乐 step down when the
	// account cannot take the level asked for, so the badge says what really
	// plays. 汽水音乐 names its rungs differently; those are left to the pick
	// the listener made.
	keep := level
	switch got.Quality {
	case "128":
		keep = "standard"
	case "320":
		keep = "higher"
	case "flac":
		keep = "lossless"
	}
	// 汽水's AAC arrives already transcoded to FLAC. Keep that, rather than
	// guessing from the quality asked for. FLAC is cached as lossless
	// whatever level it was asked at.
	if got.Type == "flac" {
		keep = "lossless"
	}
	return &netease.Stream{URL: got.URL, Type: got.Type, Level: keep}, nil
}

// The songs of the bridge, kept beside the session, so a queue of them
// plays again after the app reopens.

func bridgeSongsPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bridge-songs.json"), nil
}

// saveBridgeSongs keeps the registry, so restored queues still play.
func (a *app) saveBridgeSongs() {
	if a.bridge == nil {
		return // a test's bare app
	}
	a.bridge.mu.Lock()
	dump := struct {
		Next  int64                         `json:"next"`
		Lists map[int64]bridgeList          `json:"lists"`
		ListK map[string]int64              `json:"listKeys"`
		Songs map[int64]sidecar.BridgeTrack `json:"songs"`
		SongK map[string]int64              `json:"songKeys"`
		SongP map[int64]providerID          `json:"songProvs"`
	}{a.bridge.next, a.bridge.lists, a.bridge.listKey, a.bridge.songs, a.bridge.songKey, a.bridge.songProv}
	a.bridge.mu.Unlock()
	b, err := json.Marshal(dump)
	if err != nil {
		return
	}
	path, err := bridgeSongsPath()
	if err != nil {
		return
	}
	if os.WriteFile(path+".part", b, 0o600) == nil {
		os.Rename(path+".part", path)
	}
}

// restoreBridgeSongs loads the registry back, before the player picks up
// its queue of these songs.
func (a *app) restoreBridgeSongs() {
	if a.bridge == nil {
		return // a test's bare app
	}
	path, err := bridgeSongsPath()
	if err != nil {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var dump struct {
		Next  int64                         `json:"next"`
		Lists map[int64]bridgeList          `json:"lists"`
		ListK map[string]int64              `json:"listKeys"`
		Songs map[int64]sidecar.BridgeTrack `json:"songs"`
		SongK map[string]int64              `json:"songKeys"`
		SongP map[int64]providerID          `json:"songProvs"`
	}
	if json.Unmarshal(b, &dump) != nil || dump.Next >= 0 {
		return
	}
	a.bridge.mu.Lock()
	defer a.bridge.mu.Unlock()
	a.bridge.next = dump.Next
	if dump.Lists != nil {
		a.bridge.lists = dump.Lists
	}
	if dump.ListK != nil {
		a.bridge.listKey = dump.ListK
	}
	if dump.Songs != nil {
		a.bridge.songs = dump.Songs
	}
	if dump.SongK != nil {
		a.bridge.songKey = dump.SongK
	}
	if dump.SongP != nil {
		a.bridge.songProv = dump.SongP
	}
}
