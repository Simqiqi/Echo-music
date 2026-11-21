package app

import (
	"fmt"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/qishui"
)

// 搬运歌单：把一个歌单里的歌搬到另一个平台。这边读得到歌单的平台都能当来源
// （网易云、汽水、酷狗、QQ），能写的才当得了目标——网易云（新建歌单）与汽水
// 音乐（已有歌单，桥接提供接口）；QQ 音乐与酷狗音乐的第三方接口只有读，搬不
// 过去，所以不进目标列表。
//
// 匹配按「歌名 + 歌手」在目标平台搜一遍，歌名相等（归一化后）才算数，歌手能
// 对上更好；搜不到就记一笔「未找到」，不会拿一首不对的歌凑数。

// migrateMax is how many songs one migration takes: each one costs a
// search on the target, and a playlist of thousands would look hung.
const migrateMax = 300

// migrateBatch is how many songs go in at once: 网易云 refuses very long
// requests, and a smaller batch reports progress more often.
const migrateBatch = 100

// migrateState is a migration as the toast and the menu show it. The
// counters move on the goroutine that runs the migration while the toast
// reads them on the UI thread, so they go through a lock.
type migrateState struct {
	mu      sync.Mutex
	name    string // the playlist being carried over
	total   int
	matched int
	missing int
	added   int
	done    bool
	err     string
}

// count moves the migration's counters on as it works: one lookup done,
// and whether it found the song.
func (m *migrateState) count(found bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if found {
		m.matched++
	} else {
		m.missing++
	}
}

// add records songs that went into the target playlist.
func (m *migrateState) add(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.added += n
}

// finish records how the migration ended.
func (m *migrateState) finish(err string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.done, m.err = true, err
}

// progress is the line the toast shows while a migration runs.
func (m *migrateState) progress() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != "" {
		return "搬运失败：" + m.err
	}
	if m.done {
		line := fmt.Sprintf("已搬运 %d 首到「%s」", m.added, m.name)
		if m.missing > 0 {
			line += fmt.Sprintf("，%d 首没找到", m.missing)
		}
		return line
	}
	return fmt.Sprintf("正在搬运「%s」：%d/%d", m.name, m.matched+m.missing, m.total)
}

// migrateToNetease carries the playlist on screen into a new playlist of
// the listener's own on 网易云.
func (a *app) migrateToNetease() {
	if a.profile == nil {
		a.toast = "请先登录网易云音乐，再搬运过去"
		a.redraw()
		return
	}
	name, tracks := a.playlistToExport()
	if len(tracks) == 0 {
		a.toast = "没有可搬运的歌曲"
		a.redraw()
		return
	}
	if len(tracks) > migrateMax {
		tracks = tracks[:migrateMax]
	}
	st := &migrateState{name: name, total: len(tracks)}
	a.migrate = st
	a.toast = st.progress()
	a.redraw()

	go func() {
		var ids []int64
		for _, t := range tracks {
			tr, ok := matchOnNetease(t)
			if ok {
				ids = append(ids, tr.ID)
			}
			st.count(ok)
			a.post(func() { a.toast = st.progress(); a.redraw() })
		}
		if len(ids) == 0 {
			st.finish("一首都没匹配上，没有建立歌单")
			a.post(func() { a.toast = st.progress(); a.redraw() })
			return
		}
		pl, err := netease.CreatePlaylist(name + "（搬运）")
		if err != nil || pl == nil {
			st.finish("新建歌单失败")
			a.post(func() { a.toast = st.progress(); a.redraw() })
			return
		}
		for i := 0; i < len(ids); i += migrateBatch {
			end := min(i+migrateBatch, len(ids))
			if err := netease.AddToPlaylist(pl.ID, ids[i:end]...); err != nil {
				st.finish("添加歌曲失败：" + err.Error())
				a.post(func() { a.toast = st.progress(); a.redraw() })
				return
			}
			st.add(end - i)
			a.post(func() { a.toast = st.progress(); a.redraw() })
		}
		st.finish("")
		a.post(func() { a.toast = st.progress(); a.redraw() })
	}()
}

// matchOnNetease looks a song up on 网易云 by its name and artist, and
// reports whether a version of it was found.
func matchOnNetease(t netease.Track) (netease.Track, bool) {
	q := t.Name
	if len(t.Artists) > 0 && t.Artists[0].Name != "" {
		q += " " + t.Artists[0].Name
	}
	res, err := searchResults(q, netease.SearchSongs, 5, 0)
	if err != nil {
		return netease.Track{}, false
	}
	return pickMatch(t.Name, artistLabel(t.Artists), res.Songs)
}

// artistLabel is a track's artists as one line, the way both sides write
// them (a song may have several).
func artistLabel(artists []netease.Artist) string {
	names := make([]string, 0, len(artists))
	for _, ar := range artists {
		if ar.Name != "" {
			names = append(names, ar.Name)
		}
	}
	return strings.Join(names, "/")
}

// pickMatch chooses the song from found that is the one wanted. A title
// that matches exactly wins over one that only matches once its bracketed
// qualifiers are stripped — 「青花瓷」 is the studio take, 「青花瓷
// (Live)」 is not — and the artist breaks a tie between versions. It
// reports false rather than settling for a song that is not it.
func pickMatch(want, wantArtist string, found []netease.Track) (netease.Track, bool) {
	wantExact := strings.ToLower(strings.TrimSpace(want))
	wantNorm := normalizeSong(want)
	if wantNorm == "" {
		return netease.Track{}, false
	}
	wantArtist = normalizeSong(wantArtist)
	best, bestScore := -1, 0
	for i, cand := range found {
		score := 0
		switch {
		case strings.ToLower(strings.TrimSpace(cand.Name)) == wantExact:
			score = 2
		case normalizeSong(cand.Name) == wantNorm:
			score = 1 // 同一首歌，只是版本说明（Live、重制）不同
		default:
			continue
		}
		if wantArtist != "" && strings.Contains(normalizeSong(artistLabel(cand.Artists)), wantArtist) {
			score += 2
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 {
		return netease.Track{}, false
	}
	return found[best], true
}

// normalizeSong strips what differs between two catalogues' spellings of
// the same title: case, spaces, punctuation and bracketed qualifiers.
func normalizeSong(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// 括号里的内容多是「(Live)」「【重制版】」这类版本说明，两边常常不一致。
	for _, pair := range [][2]string{{"(", ")"}, {"（", "）"}, {"[", "]"}, {"【", "】"}} {
		for {
			i := strings.Index(s, pair[0])
			if i < 0 {
				break
			}
			j := strings.Index(s[i:], pair[1])
			if j < 0 {
				break
			}
			s = s[:i] + s[i+j+len(pair[1]):]
		}
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r > 127: // 汉字等：留下
			b.WriteRune(r)
		}
	}
	return b.String()
}

// migrateToQishui carries the playlist on screen into one of the
// listener's own playlists on 汽水音乐, matched by name and artist.
func (a *app) migrateToQishui(listID, listName string) {
	if a.bridgeAccountOf(provQishui) == nil {
		a.toast = "请先登录汽水音乐，再搬运过去"
		a.redraw()
		return
	}
	_, tracks := a.playlistToExport()
	if len(tracks) == 0 {
		a.toast = "没有可搬运的歌曲"
		a.redraw()
		return
	}
	if len(tracks) > migrateMax {
		tracks = tracks[:migrateMax]
	}
	st := &migrateState{name: listName, total: len(tracks)}
	a.migrate = st
	a.toast = st.progress()
	a.redraw()

	go func() {
		var ids []string
		for _, t := range tracks {
			id, ok := matchOnQishui(t)
			if ok {
				ids = append(ids, id)
			}
			st.count(ok)
			a.post(func() { a.toast = st.progress(); a.redraw() })
		}
		if len(ids) == 0 {
			st.finish("一首都没匹配上")
			a.post(func() { a.toast = st.progress(); a.redraw() })
			return
		}
		for i := 0; i < len(ids); i += 50 {
			end := min(i+50, len(ids))
			n, err := qishui.AddTracks(listID, ids[i:end])
			if err != nil {
				st.finish("添加歌曲失败：" + err.Error())
				a.post(func() { a.toast = st.progress(); a.redraw() })
				return
			}
			st.add(n)
			a.post(func() { a.toast = st.progress(); a.redraw() })
		}
		st.finish("")
		a.post(func() { a.toast = st.progress(); a.redraw() })
	}()
}

// matchOnQishui looks a song up on 汽水音乐 by its name and artist.
func matchOnQishui(t netease.Track) (string, bool) {
	q := t.Name
	if len(t.Artists) > 0 && t.Artists[0].Name != "" {
		q += " " + t.Artists[0].Name
	}
	items, _, _, err := bridgeAPI(provQishui).Search(q, 5, 0)
	if err != nil {
		return "", false
	}
	want := normalizeSong(t.Name)
	wantArtist := normalizeSong(artistLabel(t.Artists))
	for _, it := range items {
		if normalizeSong(it.Name) != want {
			continue
		}
		if wantArtist != "" && !strings.Contains(normalizeSong(it.Artist), wantArtist) {
			continue
		}
		return it.ID, true
	}
	return "", false
}

// migrateLine is the toast line for a migration that just finished, kept
// apart so the menu can say what happened last.
func (a *app) migrateLine() string {
	if a.migrate == nil {
		return ""
	}
	return a.migrate.progress()
}

// migratePicker is the little dialog that asks which of the listener's own
// playlists on a bridge source a migration should go into.
type migratePicker struct {
	open bool
	prov providerID
}

// migrateToQishuiPick asks for the target playlist on 汽水音乐 first.
func (a *app) migrateToQishuiPick() {
	if a.bridgeAccountOf(provQishui) == nil {
		a.toast = "请先登录汽水音乐，再搬运过去"
		a.redraw()
		return
	}
	st := a.bridgeOf(provQishui)
	if len(st.lists) == 0 {
		// 侧栏的歌单还没取回来：取一次，取回来再让人挑。
		a.loadBridgeLists(provQishui)
		a.toast = "正在取汽水音乐的歌单，稍等一会儿再点一次"
		a.redraw()
		return
	}
	a.migratePick = migratePicker{open: true, prov: provQishui}
}

// migratePickerDialog lists the account's playlists for a migration to go
// into; nothing is written until one is chosen.
func (a *app) migratePickerDialog(c *ui.Context) {
	p := a.pal
	d := &a.migratePick
	if !d.open {
		return
	}
	st := a.bridgeOf(d.prov)
	ui.DialogBase(c.Key("migrate-target"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(400).Padding(22).Radius(18).Gap(12).AlignItems(ui.Stretch).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		ui.Text(c, "搬运到 "+d.prov.name()+" 的哪个歌单？").FontSize(16).FontWeight(700)
		ui.Text(c, "按「歌名 + 歌手」在 "+d.prov.name()+" 里找到对应版本再添加，找不到的会跳过。").FontSize(12).TextColor(p.muted)
		ui.Column(c).MaxHeight(320).Children(func() {
			sc := ui.Scroll(c.Key("migrate-lists")).Fill().Gap(2)
			sc.Children(func() {
				for _, l := range st.lists {
					row := ui.ButtonBase(c.Key(l.ID)).Padding(9, 11).Radius(10).Gap(8).AlignItems(ui.Center).Label(l.Name)
					if row.Hovered() {
						row.Background(p.hover)
					}
					row.Children(func() {
						ui.Text(c, l.Name).FontSize(13).TextColor(p.text).Grow(1).SingleLine()
						if l.Count > 0 {
							ui.Text(c, fmt.Sprintf("%d 首", l.Count)).FontSize(11.5).TextColor(p.faint)
						}
					})
					if row.Clicked() {
						name := l.Name
						id := l.ID
						d.open = false
						a.post(func() { a.migrateToQishui(id, name) })
					}
				}
			})
		})
		ui.Row(c).Justify(ui.End).Children(func() {
			if pillButton(c.Key("cancel"), nil, "取消", p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
			}
		})
	})
}
