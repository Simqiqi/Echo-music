package app

import (
	"fmt"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// userDetail and userPlaylists load a user and their playlists from
// NetEase, and followedArtists and subscribedPodcasts what the listener
// follows; tests replace them.
var (
	userDetail         = netease.UserDetail
	userPlaylists      = netease.UserPlaylists
	followedArtists    = netease.FollowedArtists
	subscribedPodcasts = netease.SubscribedPodcasts
)

// userData is what the app has of a user: who they are, and the playlists
// they made that anyone may see. Of the listener's own page, it has the
// artists they follow and the podcasts they subscribe to too, which
// NetEase shows no one else.
type userData struct {
	user      *netease.User
	playlists []netease.Playlist
	loading   int // how many of the two are on their way
	err       string

	artists       []netease.ArtistProfile
	podcasts      []netease.Podcast
	artistsShown  bool // whether artists has loaded once
	podcastsShown bool
	subsLoading   int
}

func (a *app) loadUser(id int64) {
	d := a.users[id]
	if d == nil {
		d = &userData{}
		a.users[id] = d
	}
	if a.profile != nil && a.profile.UserID == id {
		a.loadSubscriptions(d)
	}
	if d.user != nil || d.loading > 0 {
		return
	}
	d.loading, d.err = 2, ""
	go func() {
		u, err := userDetail(id)
		a.post(func() {
			d.loading--
			if err != nil {
				d.err = "用户加载失败"
				return
			}
			d.user = u
		})
	}()
	go func() {
		made, _, err := userPlaylists(id)
		a.post(func() {
			d.loading--
			if err == nil {
				d.playlists = publicPlaylists(made, id)
			}
		})
	}()
}

// loadSubscriptions loads the artists the listener follows and the
// podcasts they subscribe to, each time their page shows: they follow and
// subscribe on other pages, and these keep up. What they had shows while
// the lists are on their way.
func (a *app) loadSubscriptions(d *userData) {
	if d.subsLoading > 0 {
		return
	}
	d.subsLoading = 2
	go func() {
		ars, err := followedArtists()
		a.post(func() {
			d.subsLoading--
			if err == nil {
				d.artists, d.artistsShown = ars, true
			}
		})
	}()
	go func() {
		pods, err := subscribedPodcasts()
		a.post(func() {
			d.subsLoading--
			if err == nil {
				d.podcasts, d.podcastsShown = pods, true
			}
		})
	}()
}

// avatarTint is the color a photo is mostly of, the same one the player
// takes from a cover; while the photo loads, a quiet color of the user's
// own name, as a playlist with no cover has.
func (a *app) avatarTint(url, name string) ui.Color {
	if art := a.art.get(netease.Sized(url, 600), false); art != nil && art.tint.A > 0 {
		return art.tint
	}
	return hueTint(nameHue(name), 0.42, 0.55)
}

// vivid pushes a color away from its own gray: the same hue, more of
// itself, for when a color lies over a dark ground, which muddies it.
func vivid(c ui.Color, k float32) ui.Color {
	r, g, b := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255
	l := 0.299*r + 0.587*g + 0.114*b
	at := func(v float32) uint8 { return uint8(min(1, max(0, l+(v-l)*k))*255 + 0.5) }
	return ui.RGB(at(r), at(g), at(b))
}

// userWash lays a user's color across their header: strongest where the
// photo is, fading out to the right, so the pane reads as lit by them
// rather than painted a flat color. It is drawn on the pane itself, under
// what the pane holds.
func (a *app) userWash(tint ui.Color, radius float32) func(*ui.Painter, ui.Rect) {
	var stops []ui.GradientStop
	if a.pal.dark {
		// On the dark ground a plain wash reads gray, so the color goes
		// out more of itself, and gives up sooner.
		t := vivid(tint, 1.5)
		at := func(alpha float32) ui.Color { return ui.RGBA(t.R, t.G, t.B, alpha) }
		stops = []ui.GradientStop{
			{Color: at(0.06), At: 0},
			{Color: at(0.19), At: 0.16},
			{Color: at(0.24), At: 0.34},
			{Color: at(0.08), At: 0.62},
			{Color: at(0), At: 0.9},
		}
	} else {
		at := func(alpha float32) ui.Color { return ui.RGBA(tint.R, tint.G, tint.B, alpha) }
		stops = []ui.GradientStop{
			{Color: at(0.05), At: 0},
			{Color: at(0.22), At: 0.14},
			{Color: at(0.30), At: 0.32},
			{Color: at(0.13), At: 0.64},
			{Color: at(0), At: 1},
		}
	}
	return func(pt *ui.Painter, r ui.Rect) {
		pt.FillGradient(r, ui.LinearGradient{Angle: 90, Stops: stops}, radius)
	}
}

// userPhoto is a user's picture, ringed where it sits on their own color:
// the rim keeps the photo and the wash behind it from reading as one
// shape, whatever the photo is colored.
func (a *app) userPhoto(c *ui.Context, url string, ringed bool, size float32) {
	p := a.pal
	if !ringed {
		ui.Box(c).Size(size, size).Radius(size/2).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			a.coverFill(c, url, 600, size/2)
		})
		return
	}
	rim := ui.RGBA(255, 255, 255, 0.92)
	if p.dark {
		rim = ui.RGBA(255, 255, 255, 0.26)
	}
	ui.Box(c).Size(size, size).Radius(size/2).Center().Background(rim).
		Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
		inner := size - 6
		ui.Box(c).Size(inner, inner).Radius(inner / 2).Children(func() {
			a.coverFill(c, url, 600, inner/2)
		})
	})
}

// publicPlaylists are the playlists of lists a user made that anyone may
// see: their own, as made lists those they saved too, but not their
// private ones.
func publicPlaylists(lists []netease.Playlist, uid int64) []netease.Playlist {
	var out []netease.Playlist
	for _, pl := range lists {
		if pl.Creator.UserID == uid && pl.Privacy != 10 {
			out = append(out, pl)
		}
	}
	return out
}

func (a *app) userPage(c *ui.Context) {
	id := a.route.id
	d := a.users[id]
	if d.user == nil {
		if d.loading == 0 {
			a.centered(c, d.err, func() { a.loadUser(id) })
			return
		}
		s := a.pageShimmer(c)
		a.scrollPage(c, func() {
			a.playlistHeaderSkeleton(c, s)
			a.playlistCardsSkeleton(c, s, a.columns(c, 168), 168, false)
		})
		return
	}
	u := d.user
	own := a.profile != nil && a.profile.UserID == id
	onGlass := a.settings.glassOn()
	tint := a.avatarTint(u.AvatarURL, u.Nickname)
	a.scrollPage(c, func() {
		a.userHero(c, u, d, own, onGlass, tint)
		a.section(c, "创建的歌单")
		switch {
		case d.loading > 0:
			a.playlistCardsSkeleton(c, a.pageShimmer(c), a.columns(c, 168), 168, false)
		case len(d.playlists) == 0:
			a.empty(c, "暂无公开歌单")
		default:
			ui.Grid(c).Columns(a.columns(c, 168)).GapX(20).GapY(28).Children(func() {
				for i := range d.playlists {
					a.playlistCard(c.Key(d.playlists[i].ID), &d.playlists[i])
				}
			})
		}
		if own {
			a.userShelf(c, d)
		}
	})
}

// userHero is the card at the head of a user's page: their picture on a
// wash of its own color, their name and signature, and three counts set
// apart as a row of their own — who they follow, who follows them, and
// the playlists they made. Without the glass it is a quiet plate of the
// sidebar's tone, so the same shape reads in either appearance.
func (a *app) userHero(c *ui.Context, u *netease.User, d *userData, own, onGlass bool, tint ui.Color) {
	p := a.pal
	head := ui.Row(c.Key("user-head")).Gap(30).Padding(26, 30).AlignItems(ui.Center).Radius(28).Clip()
	if onGlass {
		head.Material(glass.Glass{Clarity: a.glassClarity(0.4)})
		head.Draw(a.userWash(tint, 28))
	} else {
		head.Background(p.sidebar).Border(1, p.hairline).Shadow(0, 16, 44, -20, p.cardShade)
	}
	head.Children(func() {
		a.userPhoto(c, u.AvatarURL, onGlass, 152)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(9).Children(func() {
			cap := "用户"
			if own {
				cap = "我的主页"
			}
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(3).Height(12).Radius(1.5).Background(p.accent)
				ui.Text(c, cap).FontSize(11.5).FontWeight(700).LetterSpacing(0.8).TextColor(p.accent)
			})
			ui.Text(c, u.Nickname).FontSize(32).FontWeight(800).LetterSpacing(-0.8).MaxLines(2).LineHeight(1.18)
			if u.Signature != "" {
				ui.Text(c, u.Signature).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.5)
			}
			ui.Row(c).Margin(10, 0, 0, 0).Gap(0).AlignItems(ui.Center).Children(func() {
				for i, s := range []struct{ n, label string }{
					{count(u.Follows), "关注"},
					{count(u.Followeds), "粉丝"},
					{count(len(d.playlists)), "歌单"},
				} {
					if i > 0 {
						ui.Box(c).Width(1).Height(28).Margin(0, 20).Background(p.hairline)
					}
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, s.n).FontSize(18).FontWeight(700).FontFeatures("tnum")
						ui.Text(c, s.label).FontSize(11).TextColor(p.muted)
					})
				}
			})
		})
	})
}

// userShelf lays the two personal lists out in cards of their own: side by
// side where the page is wide, one above the other where it is not. Every
// row still opens the artist or podcast it names, and the empty lists
// still say so.
func (a *app) userShelf(c *ui.Context, d *userData) {
	if !d.artistsShown && !d.podcastsShown {
		return
	}
	a.section(c, "关注与订阅")
	w, _ := c.Size()
	inner := w - sidebarWidth - 2*(pagePad-12)
	wide := inner >= 760 && d.artistsShown && d.podcastsShown
	cols := a.columns(c, 132)
	if wide {
		cols = max(2, int((inner/2-36)/96))
	}
	build := func() {
		fill := func(el ui.Element) {
			if wide {
				el.Grow(1).Basis(0).MinWidth(0)
			}
		}
		if d.artistsShown {
			fill(a.shelfCard(c, "关注的歌手", iconUser, len(d.artists), func() {
				if len(d.artists) == 0 {
					a.empty(c, "还没有关注歌手")
					return
				}
				ui.Grid(c).Columns(cols).GapX(12).GapY(18).Children(func() {
					for i := range d.artists {
						a.artistCard(c.Key(d.artists[i].ID), &d.artists[i])
					}
				})
			}))
		}
		if d.podcastsShown {
			fill(a.shelfCard(c, "订阅的播客", iconPodcast, len(d.podcasts), func() {
				if len(d.podcasts) == 0 {
					a.empty(c, "还没有订阅播客")
					return
				}
				ui.Column(c).Children(func() {
					for i := range d.podcasts {
						a.podcastRow(c.Key(d.podcasts[i].ID), &d.podcasts[i])
					}
				})
			}))
		}
	}
	if wide {
		ui.Row(c).Gap(16).AlignItems(ui.Stretch).Children(build)
	} else {
		ui.Column(c).Gap(16).Children(build)
	}
}

// shelfCard is one of the two personal lists: a quiet pane with its name
// and count in a head of its own, holding the rows beneath.
func (a *app) shelfCard(c *ui.Context, title string, icon *ui.SVG, n int, body func()) ui.Element {
	p := a.pal
	card := ui.Column(c).Padding(16, 18, 18, 18).Radius(20)
	if a.settings.glassOn() {
		fill := ui.RGBA(255, 255, 255, 0.32)
		edge := ui.RGBA(255, 255, 255, 0.45)
		if p.dark {
			fill, edge = ui.RGBA(255, 255, 255, 0.055), ui.RGBA(255, 255, 255, 0.10)
		}
		card.Background(fill).Border(1, edge)
	} else {
		card.Background(p.sidebar).Border(1, p.hairline)
	}
	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(0, 0, 12, 0).Children(func() {
			ui.Icon(c, icon).Size(15, 15).TextColor(p.accent)
			ui.Text(c, title).FontSize(14.5).FontWeight(700)
			ui.Text(c, fmt.Sprint(n)).FontSize(11).FontWeight(600).TextColor(p.muted).
				Padding(2, 8).Radius(999).Background(p.hover)
		})
		body()
	})
	return card
}

// userRow is a user in a list, opening their page.
func (a *app) userRow(c *ui.Context, u *netease.User) {
	p := a.pal
	row := ui.ButtonBase(c).Padding(10, 12).Gap(14).Radius(10).AlignItems(ui.Center).FocusRing(false).Label(u.Nickname)
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Children(func() {
		a.cover(c, u.AvatarURL, 120, 48, 24)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, u.Nickname).FontSize(14).FontWeight(600).SingleLine()
			if u.Signature != "" {
				ui.Text(c, u.Signature).FontSize(12.5).TextColor(p.muted).SingleLine()
			}
		})
	})
	if row.Clicked() {
		a.open(route{page: pageUser, id: u.UserID})
	}
}
