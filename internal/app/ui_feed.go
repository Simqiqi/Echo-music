package app

import (
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// feedPage is the listener's feed: what the artists they follow and the
// friends they keep post, newest first, as the events come.
func (a *app) feedPage(c *ui.Context) {
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("feed")).TrackScroll(&a.pageScroll).Grow(1).
		Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).
		Children(func() {
			a.startMark(c)
			defer a.endMark(c)
			a.heading(c, "关注动态", "你关注的歌手与好友的新消息")

			if !a.feedState.loaded && len(a.feed) == 0 {
				if a.feedState.err != "" {
					a.centered(c, a.feedState.err, func() { a.loadFeed(false) })
					return
				}
				return
			}
			if len(a.feed) == 0 {
				a.empty(c, "还没有关注动态，去关注几位歌手试试")
				return
			}
			ui.Column(c).Children(func() {
				for i := range a.feed {
					a.event(c.Key(a.feed[i].ID), &a.feed[i])
				}
				a.pageFoot(c, &a.feedState, a.loadFeedMore)
			})
		})
}

// loadFeed loads the feed, from the top when starting, or the page after
// the one held when more is true.
func (a *app) loadFeed(start bool) {
	if start {
		if a.feedState.loaded || a.feedState.loading {
			return
		}
	} else {
		if a.feedState.loading || !a.feedState.more {
			return
		}
	}
	a.feedState.loading = true
	a.feedState.err = ""
	last := a.feedLast
	go func() {
		ev, lastTime, more, err := netease.FollowedEvents(30, last)
		a.post(func() {
			a.feedState.loading = false
			if err != nil {
				a.feedState.err = "动态加载失败，请检查网络"
				return
			}
			a.feed = append(a.feed, ev...)
			a.feedLast = lastTime
			a.feedState.more = more
			a.feedState.loaded = true
		})
	}()
}

// loadFeedMore loads the next page of the feed.
func (a *app) loadFeedMore() {
	a.loadFeed(false)
}
