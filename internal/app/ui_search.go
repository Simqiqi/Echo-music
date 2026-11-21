package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// searchResults and searchSuggest search NetEase; tests replace them.
var (
	searchResults = netease.SearchPage
	searchSuggest = netease.SearchSuggest
)

// searchLimit is how many results load at a time.
const searchLimit = 60

// searchKinds are the kinds of search, as their tabs show them.
var searchKinds = []struct {
	kind        netease.SearchType
	label, unit string
}{
	{netease.SearchSongs, "单曲", "首单曲"},
	{netease.SearchArtists, "歌手", "位歌手"},
	{netease.SearchAlbums, "专辑", "张专辑"},
	{netease.SearchUsers, "用户", "位用户"},
	{netease.SearchPodcasts, "播客", "个播客"},
}

// searchData is what a search found, as far as its pages have loaded.
// 网易云的结果与桥接音源的结果各存一份：换了来源就看另一份，互不打扰。
type searchData struct {
	res  netease.SearchResult
	page paged
	// bTracks are a bridge source's results, bPage their paging, bNext
	// where the next page begins on that source.
	bTracks []netease.Track
	bPage   paged
	bNext   int
}

// search searches for q, of the kind last searched for.
func (a *app) search(q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	kind := a.searched.kind
	if kind == 0 {
		kind = netease.SearchSongs
	}
	a.searchFor(route{page: pageSearch, query: q, kind: kind})
}

// searchFor opens the page of a search, remembering its query. Searched
// for again, it loads anew.
func (a *app) searchFor(r route) {
	a.rememberSearch(r.query)
	a.dismissSuggestions()
	if r == a.route {
		a.nowPlaying = false
		a.loadSearch(r, true)
		return
	}
	a.open(r)
}

// loadSearch loads the first page of a search, unless it has, or again.
func (a *app) loadSearch(r route, again bool) {
	if a.searches[r] != nil && !again {
		return
	}
	d := &searchData{}
	a.searches[r] = d
	a.moreResults(r, d)
}

// moreResults loads the next page of a search. A search done again has
// new data, so the last one's pages land where nothing shows them.
func (a *app) moreResults(r route, d *searchData) {
	if r.source() != provNetease {
		a.moreBridgeResults(r, d)
		return
	}
	if d.page.loading {
		return
	}
	d.page.loading, d.page.err = true, ""
	offset := d.res.Len()
	go func() {
		res, err := searchResults(r.query, r.kind, searchLimit, offset)
		a.post(func() {
			d.page.loading = false
			if err != nil {
				d.page.err = "搜索失败，请稍后再试"
				return
			}
			d.res.Append(res)
			// A server can claim more while sending none.
			d.page.loaded, d.page.more = true, res.Len() > 0 && d.res.Len() < d.res.Total
		})
	}()
}

// moreBridgeResults loads the next page of a search run against a bridge
// provider: 汽水音乐 / 酷狗音乐 / QQ 音乐 each search their own catalogue,
// so the kinds of 网易云's search (歌手、专辑…) do not apply.
func (a *app) moreBridgeResults(r route, d *searchData) {
	if d.bPage.loading {
		return
	}
	prov := r.source()
	d.bPage.loading, d.bPage.err = true, ""
	offset := d.bNext
	go func() {
		items, hasMore, next, err := bridgeAPI(prov).Search(r.query, searchLimit, offset)
		a.post(func() {
			d.bPage.loading = false
			if err != nil {
				d.bPage.err = "搜索失败：" + friendlyBridgeErr(err)
				return
			}
			for _, t := range items {
				d.bTracks = append(d.bTracks, a.bridgeTrack(prov, t))
			}
			// 登记一下，重启后这些搜索结果还能接着播。
			a.saveBridgeSongs()
			d.bPage.loaded, d.bPage.more = true, hasMore
			d.bNext = next
		})
	}()
}

// searchHistoryLimit is how many searches the history keeps.
const searchHistoryLimit = 20

// rememberSearch puts q first in the history of searches, once.
func (a *app) rememberSearch(q string) {
	history := []string{q}
	for _, s := range a.settings.SearchHistory {
		if s != q && len(history) < searchHistoryLimit {
			history = append(history, s)
		}
	}
	a.settings.SearchHistory = history
	a.settings.save()
}

func (a *app) clearSearchHistory() {
	a.settings.SearchHistory = nil
	a.settings.save()
}

func (a *app) searchPage(c *ui.Context) {
	r := a.route
	d := a.searches[r]
	header := func(c *ui.Context) { a.searchHeader(c, d) }
	more := func() { a.moreResults(r, d) }
	// 桥接音源的搜索只有歌曲一种结果，直接列出来。
	if r.source() != provNetease {
		if !d.bPage.loaded && d.bPage.loading {
			a.trackSkeleton(c, a.pageShimmer(c), true, header)
			return
		}
		a.trackRows(c, d.bTracks, true, header, func(c *ui.Context) {
			a.pageFoot(c, &d.bPage, more)
			a.searchEmptyBridge(c, d)
		})
		return
	}
	if r.kind == netease.SearchSongs {
		if !d.page.loaded && d.page.loading {
			a.trackSkeleton(c, a.pageShimmer(c), true, header)
			return
		}
		a.trackRows(c, d.res.Songs, true, header, func(c *ui.Context) {
			a.pageFoot(c, &d.page, more)
			a.searchEmpty(c, d)
		})
		return
	}
	a.scrollPage(c, func() {
		header(c)
		res := &d.res
		switch r.kind {
		case netease.SearchArtists:
			ui.Grid(c).Columns(a.columns(c, 132)).GapX(20).GapY(28).Children(func() {
				for i := range res.Artists {
					a.artistCard(c.Key(res.Artists[i].ID), &res.Artists[i])
				}
			})
		case netease.SearchAlbums:
			a.albumGrid(c, res.Albums)
		case netease.SearchUsers:
			for i := range res.Users {
				a.userRow(c.Key(res.Users[i].UserID), &res.Users[i])
			}
		case netease.SearchPodcasts:
			for i := range res.Podcasts {
				a.podcastRow(c.Key(res.Podcasts[i].ID), &res.Podcasts[i])
			}
		}
		if !d.page.loaded && d.page.loading {
			a.playlistCardsSkeleton(c, a.pageShimmer(c), a.columns(c, 156), 156, false)
			return
		}
		a.pageFoot(c, &d.page, more)
		a.searchEmpty(c, d)
	})
}

// searchEmptyBridge is searchEmpty for a bridge source's results.
func (a *app) searchEmptyBridge(c *ui.Context, d *searchData) {
	if d.bPage.loaded && len(d.bTracks) == 0 {
		a.empty(c, "没有找到相关结果")
	}
}

func (a *app) searchEmpty(c *ui.Context, d *searchData) {
	if d.page.loaded && d.res.Len() == 0 {
		a.empty(c, "没有找到相关结果")
	}
}

// searchSources are the catalogues a search can be run against: 网易云
// always, plus whichever bridge providers are signed in.
func (a *app) searchSources() []providerID {
	srcs := []providerID{provNetease}
	for _, prov := range bridgeProviders {
		if a.bridgeAccountOf(prov) != nil {
			srcs = append(srcs, prov)
		}
	}
	return srcs
}

// searchHeader is what was searched for, how many it found, the catalogues
// it may be searched in, and the tabs of what it may look for instead.
func (a *app) searchHeader(c *ui.Context, d *searchData) {
	r := a.route
	src := r.source()
	sub := "搜索"
	at := 0
	if src == provNetease {
		for i, k := range searchKinds {
			if k.kind == r.kind {
				at = i
				if d.page.loaded {
					sub = fmt.Sprintf("搜索 · 找到 %d %s", d.res.Total, k.unit)
				}
			}
		}
	} else if d.bPage.loaded {
		sub = fmt.Sprintf("搜索 · 找到 %d 首单曲", len(d.bTracks))
	}
	a.heading(c, "“"+r.query+"”", sub)

	// 来源：网易云 + 已登录的桥接音源。只有一家能搜时不必摆出来。
	if srcs := a.searchSources(); len(srcs) > 1 {
		here := 0
		labels := make([]string, len(srcs))
		for i, p := range srcs {
			if p == src {
				here = i
			}
			if p == provNetease {
				labels[i] = "网易云"
				continue
			}
			labels[i] = p.name()
		}
		ui.Row(c).Margin(0, 0, 12, 0).Children(func() {
			if segmented(c.Key("source"), &here, labels...) {
				r.src = srcs[here]
				// 桥接音源只有歌曲一种结果，切过去就回到单曲。
				if r.src != provNetease {
					r.kind = netease.SearchSongs
				}
				a.show(r)
			}
		})
	}
	if src != provNetease {
		return
	}
	ui.Row(c).Margin(0, 0, 18, 0).Children(func() {
		labels := make([]string, len(searchKinds))
		for i, k := range searchKinds {
			labels[i] = k.label
		}
		if segmented(c.Key("kind"), &at, labels...) {
			r.kind = searchKinds[at].kind
			a.show(r)
		}
	})
}

func (a *app) artistCard(c *ui.Context, ar *netease.ArtistProfile) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(10).AlignItems(ui.Stretch).FocusRing(false).Label(ar.Name)
	hovered := card.Hovered()
	card.Children(func() {
		photo := ar.Avatar
		if photo == "" {
			photo = ar.PicURL
		}
		art := ui.Box(c).AspectRatio(1).Radius(999)
		if hovered {
			art.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			art.Shadow(0, 2, 8, -2, p.cardShade)
		}
		art.Children(func() { a.coverFill(c, photo, 400, 999) })
		ui.Text(c, ar.Name).FontSize(13).FontWeight(500).SingleLine().AlignSelf(ui.Center)
	})
	if card.Clicked() {
		a.open(route{page: pageArtist, id: ar.ID})
	}
}
