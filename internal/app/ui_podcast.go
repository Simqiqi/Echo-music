package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// These load a podcast and its episodes from NetEase, and subscribe to it,
// and the podcasts the tab picks and the most popular; tests replace them.
var (
	podcastDetail    = netease.PodcastDetail
	podcastPrograms  = netease.PodcastPrograms
	subscribePodcast = netease.SubscribePodcast

	recommendedPodcasts = netease.RecommendedPodcasts
	hotPodcasts         = netease.HotPodcasts
)

// programsPage is how many of a podcast's episodes load at a time.
const programsPage = 30

// podcastData is what the app has of a podcast: what it is, and its
// episodes, the newest first, as far as they have loaded.
type podcastData struct {
	podcast     *netease.Podcast
	loading     bool
	err         string
	programs    []netease.Program
	page        paged
	subscribing bool // a subscription or its end is on its way
	thenPlay    bool // its card's play button waits for it to load
}

func (a *app) loadPodcast(id int64) {
	d := a.podcasts[id]
	if d == nil {
		d = &podcastData{}
		a.podcasts[id] = d
	}
	if !d.page.loaded {
		a.loadPrograms(id)
	}
	if d.podcast != nil || d.loading {
		return
	}
	d.loading, d.err = true, ""
	go func() {
		pod, err := podcastDetail(id)
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err, d.thenPlay = "播客加载失败", false
				return
			}
			d.podcast = pod
			a.playLoaded(d)
		})
	}()
}

// loadPrograms loads the next page of a podcast's episodes.
func (a *app) loadPrograms(id int64) {
	d := a.podcasts[id]
	if d.page.loading {
		return
	}
	d.page.loading, d.page.err = true, ""
	offset := len(d.programs)
	go func() {
		programs, more, err := podcastPrograms(id, programsPage, offset)
		a.post(func() {
			d.page.loading = false
			if err != nil {
				d.page.err, d.thenPlay = "节目加载失败", false
				return
			}
			d.programs = append(d.programs, programs...)
			d.page.loaded, d.page.more = true, more && len(programs) > 0
			// Which the listener likes, for their hearts.
			ids := make([]int64, len(programs))
			for i := range programs {
				ids[i] = programs[i].ID
			}
			a.askProgramLikes(ids)
			a.playLoaded(d)
		})
	}()
}

// playProgram plays the episodes loaded, from the one at i: those with
// audio to play.
func (a *app) playProgram(d *podcastData, i int) {
	var queue []netease.Track
	at := 0
	for j := range d.programs {
		if d.programs[j].MainSong.ID == 0 {
			continue
		}
		if j <= i {
			at = len(queue)
		}
		queue = append(queue, d.programs[j].Track(d.podcast))
	}
	if len(queue) > 0 {
		a.pl.PlayQueue(queue, at)
	}
}

// playPodcast plays a podcast's episodes from the newest, from its card,
// once it and they have loaded.
func (a *app) playPodcast(id int64) {
	a.loadPodcast(id)
	d := a.podcasts[id]
	d.thenPlay = true
	a.playLoaded(d)
}

// playLoaded plays a podcast its card asked to play, once it can.
func (a *app) playLoaded(d *podcastData) {
	if d.thenPlay && d.podcast != nil && d.page.loaded {
		d.thenPlay = false
		a.playProgram(d, 0)
	}
}

// toggleSubscribe subscribes the listener to a podcast, or unsubscribes
// them, at once, going back if NetEase refuses. Signed out, it asks the
// listener to sign in.
func (a *app) toggleSubscribe(d *podcastData) {
	if d.podcast == nil || d.subscribing {
		return
	}
	if a.profile == nil {
		a.openLogin()
		return
	}
	pod := d.podcast
	tab := a.podcastTab
	sub := !pod.Subed
	pod.Subed, d.subscribing = sub, true
	if sub {
		pod.SubCount++
	} else {
		pod.SubCount--
	}
	go func() {
		err := subscribePodcast(pod.ID, sub)
		a.post(func() {
			d.subscribing = false
			switch {
			case err != nil:
				pod.Subed = !sub
				if sub {
					pod.SubCount--
				} else {
					pod.SubCount++
				}
				a.toast = "操作失败，请稍后再试"
			case sub:
				tab.subscribed(pod)
				a.toast = "已订阅"
			default:
				tab.unsubscribed(pod.ID)
				a.toast = "已取消订阅"
			}
		})
	}()
}

func (a *app) podcastPage(c *ui.Context) {
	p := a.pal
	id := a.route.id
	d := a.podcasts[id]
	if d.podcast == nil {
		if !d.loading {
			a.centered(c, d.err, func() { a.loadPodcast(id) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, true, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	pod := d.podcast
	cur := a.pl.Current()
	a.scrollPage(c, func() {
		ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 8, 0).Children(func() {
			ui.Box(c).Size(212, 212).Radius(14).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
				a.coverFill(c, pod.PicURL, 600, 14)
			})
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
				kind := "播客"
				if pod.Category != "" {
					kind += " · " + pod.Category
				}
				ui.Text(c, kind).FontSize(12).FontWeight(600).TextColor(p.accent)
				ui.Text(c, pod.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
				ui.RichText(c).FontSize(13).TextColor(p.muted).Children(func() {
					if pod.DJ.UserID != 0 {
						a.link(c.Key("dj"), pod.DJ.Nickname, p.muted, p.text, route{page: pageUser, id: pod.DJ.UserID})
						ui.Text(c, " · ")
					}
					ui.Text(c, fmt.Sprintf("%d 期 · %s 人订阅", pod.ProgramCount, count(int(pod.SubCount))))
				})
				if desc := strings.TrimSpace(pod.Description); desc != "" {
					ui.Text(c, strings.ReplaceAll(desc, "\n", " ")).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45).Margin(2, 0, 0, 0)
				}
				ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
					if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() {
						a.playProgram(d, 0)
					}
					icon, label := iconPlus, "订阅"
					if pod.Subed {
						icon, label = iconCheck, "已订阅"
					}
					if pillButton(c.Key("subscribe"), icon, label, p.hover.Over(p.bg), p.text).Clicked() {
						a.toggleSubscribe(d)
					}
				})
			})
		})
		a.section(c, "节目")
		for i := range d.programs {
			a.programRow(c.Key(d.programs[i].ID), d, i, cur)
		}
		a.pageFoot(c, &d.page, func() { a.loadPrograms(id) })
		if d.page.loaded && len(d.programs) == 0 {
			a.empty(c, "暂无节目")
		}
	})
}

// programRow is an episode in its podcast's list, which a click plays.
func (a *app) programRow(c *ui.Context, d *podcastData, i int, cur *netease.Track) {
	p := a.pal
	pr := &d.programs[i]
	playing := cur != nil && cur.ProgramID == pr.ID
	row := ui.ButtonBase(c).Height(60).PaddingX(12).Gap(14).Radius(8).AlignItems(ui.Center).FocusRing(false).
		Label(pr.Name).Disabled(pr.MainSong.ID == 0)
	switch {
	case playing:
		row.Background(p.accent.Alpha(0.08))
	case row.Hovered():
		row.Background(p.hover)
	}
	row.Children(func() {
		cover := pr.CoverURL
		if cover == "" {
			cover = d.podcast.PicURL
		}
		a.cover(c, cover, 96, 40, 6)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			title := ui.Text(c, pr.Name).FontSize(13.5).FontWeight(500).SingleLine()
			if playing {
				title.TextColor(p.accent)
			}
			ui.Text(c, time.UnixMilli(pr.CreateTime).Format("2006-01-02")).FontSize(12).TextColor(p.muted)
		})
		if playing {
			a.nowPlayingBars(c)
		}
		// A liked episode's heart shows; the others' as the pointer is on them.
		ui.Box(c).Size(32, 32).Center().Children(func() {
			if a.likedPrograms[pr.ID] || row.Hovered() {
				tr := pr.Track(d.podcast)
				a.likeButton(c.Key("like"), &tr, 15, p.muted)
			}
		})
		ui.Text(c, clock(time.Duration(pr.Duration)*time.Millisecond)).FontSize(12).FontFeatures("tnum").TextColor(p.muted)
	})
	if row.Clicked() {
		a.playProgram(d, i)
	}
}

// podcastRow is a podcast in a list, opening its page.
func (a *app) podcastRow(c *ui.Context, pod *netease.Podcast) {
	p := a.pal
	row := ui.ButtonBase(c).Padding(10, 12).Gap(14).Radius(10).AlignItems(ui.Center).FocusRing(false).Label(pod.Name)
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Children(func() {
		a.cover(c, pod.PicURL, 160, 64, 8)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, pod.Name).FontSize(14).FontWeight(600).SingleLine()
			meta := []string{fmt.Sprintf("%d 期", pod.ProgramCount)}
			if pod.DJ.Nickname != "" {
				meta = append([]string{pod.DJ.Nickname}, meta...)
			}
			ui.Text(c, strings.Join(meta, " · ")).FontSize(12.5).TextColor(p.muted).SingleLine()
			if pod.Description != "" {
				ui.Text(c, pod.Description).FontSize(12.5).TextColor(p.muted).SingleLine()
			}
		})
	})
	if row.Clicked() {
		a.open(route{page: pagePodcast, id: pod.ID})
	}
}

// programPane is what the episode playing is about, in Now Playing, in
// place of a song's lyrics.
func (a *app) programPane(c *ui.Context, tr *netease.Track) {
	ui.Scroll(c.Key(tr.ProgramID)).Grow(1).Padding(4, 12, 48, 0).Children(func() {
		desc := strings.TrimSpace(tr.Description)
		if desc == "" {
			desc = "这期节目没有简介"
		}
		ui.Text(c, desc).FontSize(15).LineHeight(1.7).TextColor(white.Alpha(0.75))
	})
}

// podcastTab is what the podcasts tab lists: the podcasts the listener
// subscribes to, NetEase's picks, and the most popular, as far as they have
// loaded.
type podcastTab struct {
	subs    []netease.Podcast // the latest subscribed first
	picks   []netease.Podcast
	hot     []netease.Podcast
	hotPage paged
	hotAt   int // how far into NetEase's list the next page starts
	loading bool
	err     string
}

func (a *app) loadPodcastTab() {
	t := a.podcastTab
	if t.hotPage.loaded || t.loading {
		return
	}
	t.loading, t.err = true, ""
	signedIn := a.profile != nil
	go func() {
		var subs []netease.Podcast
		if signedIn {
			subs, _ = subscribedPodcasts()
		}
		picks, err1 := recommendedPodcasts()
		hot, more, err2 := hotPodcasts(programsPage, 0)
		a.post(func() {
			t.loading = false
			if err1 != nil && err2 != nil {
				t.err = "无法连接网易云音乐，请检查网络"
				return
			}
			t.subs, t.picks = subs, picks
			t.addHot(hot)
			t.hotPage = paged{loaded: true, more: more && len(hot) > 0}
			if err2 != nil {
				t.hotPage.err = "热门播客加载失败"
			}
		})
	}()
}

// loadHotPodcasts loads the next page of the most popular podcasts.
func (a *app) loadHotPodcasts() {
	t := a.podcastTab
	if t.hotPage.loading {
		return
	}
	t.hotPage.loading, t.hotPage.err = true, ""
	offset := t.hotAt
	go func() {
		hot, more, err := hotPodcasts(programsPage, offset)
		a.post(func() {
			t.hotPage.loading = false
			if err != nil {
				t.hotPage.err = "热门播客加载失败"
				return
			}
			t.addHot(hot)
			t.hotPage.more = more && len(hot) > 0
		})
	}()
}

// addHot lists a page of the most popular podcasts after those listed,
// but those already listed: NetEase's pages overlap, a podcast often
// coming again a page or two later.
func (t *podcastTab) addHot(page []netease.Podcast) {
	t.hotAt += len(page)
	for _, pod := range page {
		if !slices.ContainsFunc(t.hot, func(p netease.Podcast) bool { return p.ID == pod.ID }) {
			t.hot = append(t.hot, pod)
		}
	}
}

// subscribed lists a podcast the listener just subscribed to first, once
// the tab has loaded what they subscribe to.
func (t *podcastTab) subscribed(pod *netease.Podcast) {
	if !t.hotPage.loaded {
		return
	}
	t.unsubscribed(pod.ID)
	t.subs = append([]netease.Podcast{*pod}, t.subs...)
}

// unsubscribed takes a podcast out of the listener's subscriptions.
func (t *podcastTab) unsubscribed(id int64) {
	t.subs = slices.DeleteFunc(t.subs, func(p netease.Podcast) bool { return p.ID == id })
}

func (a *app) podcastsPage(c *ui.Context) {
	t := a.podcastTab
	if !t.hotPage.loaded && !t.loading {
		a.centered(c, t.err, a.loadPodcastTab)
		return
	}
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("podcasts")).TrackScroll(&a.pageScroll).Grow(1).Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		a.heading(c, "播客", "")
		if !t.hotPage.loaded {
			a.podcastsSkeleton(c, a.pageShimmer(c))
			return
		}
		// One pick leads the page on its own cover; the subscriptions sit
		// round beneath it, the other picks as cards washed in their
		// covers' colors, and the popular ones as rows.
		hero, picks, hot := (*netease.Podcast)(nil), t.picks, t.hot
		switch {
		case len(t.picks) > 0:
			hero, picks = &t.picks[0], t.picks[1:]
		case len(t.hot) > 0:
			hero, hot = &t.hot[0], t.hot[1:]
		}
		if hero != nil {
			a.podcastHero(c, hero)
		}
		first := true
		sec := func(title string) {
			if s := a.section(c, title); first {
				s.Margin(0, 0, 14, 0)
			}
			first = false
		}
		if len(t.subs) > 0 {
			sec("我的订阅")
			ui.Grid(c).Columns(a.columns(c, 104)).GapX(20).GapY(18).Children(func() {
				for i := range t.subs {
					a.subCard(c.Key(t.subs[i].ID), &t.subs[i])
				}
			})
		}
		if len(picks) > 0 {
			sec("精选播客")
			ui.Grid(c).Columns(a.columns(c, 330)).GapX(20).GapY(14).Children(func() {
				for i := range picks {
					a.podcastTile(c.Key(picks[i].ID), &picks[i])
				}
			})
		}
		if len(hot) > 0 {
			sec("热门播客")
			ui.Grid(c).Columns(a.columns(c, 340)).GapX(20).GapY(4).Children(func() {
				for i := range hot {
					a.podcastRow(c.Key(hot[i].ID), &hot[i])
				}
			})
		}
		a.pageFoot(c, &t.hotPage, a.loadHotPodcasts)
	})
}

// podcastHero leads the podcasts page: one pick run wide across it, its
// cover showing through from the right and a line on why to listen beside
// the name. The button under it plays it; the rest of the banner opens it.
func (a *app) podcastHero(c *ui.Context, pod *netease.Podcast) {
	const radius, height = 20, 224
	base := ui.Hex("#17181c")
	card := ui.ButtonBase(c).Height(height).Radius(radius).Clip().FocusRing(false).Label(pod.Name).Background(base)
	hovered := card.Hovered()
	pose := ui.Pose{}
	if hovered {
		pose.Scale = 1.004
	}
	card.AnimatePose(pose, ui.Snappy)
	bmp := bitmap(a.art.get(netease.Sized(pod.PicURL, 900), false))
	w, _ := c.Size()
	wide := w-sidebarWidth > 700
	card.Children(func() {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
			if bmp != nil {
				p.Image(bmp, r, ui.Cover)
			}
			// The words keep their ground at the left; the cover comes
			// clear by the right, with a last dark breath along the foot.
			p.FillGradient(r, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
				{At: 0, Color: base.Alpha(0.96)},
				{At: 0.34, Color: base.Alpha(0.9)},
				{At: 0.64, Color: base.Alpha(0.52)},
				{At: 1, Color: base.Alpha(0.2)},
			}}, radius)
			p.FillGradient(r, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
				{At: 0, Color: ui.RGBA(0, 0, 0, 0)},
				{At: 1, Color: ui.RGBA(0, 0, 0, 0.42)},
			}}, radius)
		})
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(22).Gap(26).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(6).Children(func() {
				label := pod.Category
				if label == "" {
					label = "编辑精选"
				}
				ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconPodcast).Size(12, 12).TextColor(white.Alpha(0.7))
					ui.Text(c, label).FontSize(11).FontWeight(700).LetterSpacing(0.8).TextColor(white.Alpha(0.7))
				})
				ui.Text(c, pod.Name).FontSize(26).FontWeight(800).LetterSpacing(-0.5).TextColor(white).SingleLine()
				ui.RichText(c).FontSize(12).Children(func() {
					if pod.DJ.UserID != 0 {
						ui.Text(c, pod.DJ.Nickname).FontWeight(600).TextColor(white.Alpha(0.85))
						ui.Text(c, " · ").TextColor(white.Alpha(0.5))
					}
					ui.Textf(c, "%d 期 · %s 人订阅", pod.ProgramCount, count(int(pod.SubCount))).
						TextColor(white.Alpha(0.62))
				})
				if line := strings.TrimSpace(pod.RcmdText); line != "" {
					ui.Text(c, line).FontSize(12.5).LineHeight(1.5).TextColor(white.Alpha(0.72)).MaxLines(2)
				}
				ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(6, 0, 0, 0).Children(func() {
					if pillButton(c.Key("play"), iconPlay, "播放", white, base).Clicked() {
						a.playPodcast(pod.ID)
					}
				})
			})
			if wide {
				ui.Box(c).Size(148, 148).Shrink(0).Radius(14).Shadow(0, 18, 44, -10, ui.RGBA(0, 0, 0, 0.5)).Children(func() {
					a.coverArt(c, pod.PicURL, pod.Name, 500, 14)
				})
			}
		})
	})
	if card.Clicked() {
		a.open(route{page: pagePodcast, id: pod.ID})
	}
}

// subCard is a podcast the listener subscribes to, round as a dial, its
// name under it.
func (a *app) subCard(c *ui.Context, pod *netease.Podcast) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(8).AlignItems(ui.Center).FocusRing(false).Label(pod.Name)
	hovered := card.Hovered()
	pose := ui.Pose{}
	if hovered {
		pose.Y = -2
	}
	card.AnimatePose(pose, ui.Snappy)
	card.Children(func() {
		ui.Box(c).Size(84, 84).Radius(999).Children(func() {
			a.coverFill(c, pod.PicURL, 300, 999)
			// A dial of its own in the middle of the dial: it plays the
			// podcast without opening it, and the rest of the card opens.
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Children(func() {
				play := ui.ButtonBase(c.Key("play")).Size(34, 34).Radius(999).Center().
					Background(p.accent).TextColor(white).FocusRing(false).Label("播放").
					Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.3))
				fade := ui.Pose{Fade: 0, Scale: 0.7}
				if hovered {
					fade = ui.Pose{Scale: 1}
				}
				play.AnimatePose(fade, ui.Snappy)
				play.Children(func() { ui.Icon(c, iconPlay).Size(14, 14).Margin(0, 0, 0, 1) })
				if play.Clicked() {
					a.playPodcast(pod.ID)
				}
			})
		})
		ui.Text(c, pod.Name).FontSize(12).FontWeight(600).SingleLine()
	})
	if card.Clicked() {
		a.open(route{page: pagePodcast, id: pod.ID})
	}
}

// podcastTile is one of the other picks: its cover on the left, its name,
// who makes it, and a line on why to listen, on a wash of the cover's own
// color. The pointer lifts it, and a button on the cover plays it.
func (a *app) podcastTile(c *ui.Context, pod *netease.Podcast) {
	p := a.pal
	card := ui.ButtonBase(c).Padding(13).Radius(16).Gap(13).AlignItems(ui.Center).FocusRing(false).Label(pod.Name)
	hovered := card.Hovered()
	tint := chartWash(a.avatarTint(pod.PicURL, pod.Name))
	fill := tint.Alpha(0.1)
	if p.dark {
		fill = tint.Alpha(0.16)
	}
	if hovered {
		fill = tint.Alpha(0.18)
		if p.dark {
			fill = tint.Alpha(0.26)
		}
	}
	card.Background(card.AnimateColor("bg", fill, ui.Animation{Duration: 140 * time.Millisecond})).Border(1, p.hairline)
	pose := ui.Pose{}
	if hovered {
		pose.Y = -2
		card.Shadow(0, 10, 26, -12, p.cardShade)
	}
	card.AnimatePose(pose, ui.Snappy)
	card.Children(func() {
		ui.Box(c).Size(92, 92).Shrink(0).Radius(14).Shadow(0, 3, 10, -3, p.cardShade).Children(func() {
			a.coverArt(c, pod.PicURL, pod.Name, 300, 14)
			a.coverPlayButton(c, hovered, func() { a.playPodcast(pod.ID) })
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, pod.Name).FontSize(14.5).FontWeight(700).SingleLine()
			meta := []string{fmt.Sprintf("%d 期", pod.ProgramCount)}
			if pod.DJ.Nickname != "" {
				meta = append([]string{pod.DJ.Nickname}, meta...)
			}
			ui.Text(c, strings.Join(meta, " · ")).FontSize(11).TextColor(p.faint).SingleLine()
			line := strings.TrimSpace(pod.RcmdText)
			if line == "" {
				line = strings.TrimSpace(pod.Description)
			}
			if line != "" {
				ui.Text(c, line).FontSize(12).LineHeight(1.5).TextColor(p.muted).MaxLines(2).Margin(2, 0, 0, 0)
			}
		})
	})
	if card.Clicked() {
		a.open(route{page: pagePodcast, id: pod.ID})
	}
}
