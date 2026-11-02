package app

import (
	"fmt"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
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
func (a *app) userPhoto(c *ui.Context, url string, ringed bool) {
	size := float32(artistPhotoSize)
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
	p := a.pal
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
		// Liquid Glass: who they are sits on a pane of it, from the
		// photo out across the name and the counts, lit along its rim,
		// rather than loose on the page. The pane carries a wash of the
		// photo's own color, from the photo out to the right.
		head := ui.Row(c.Key("user-head")).Gap(28).AlignItems(ui.End)
		if onGlass {
			head.Padding(20, 22).Radius(28).Clip().Material(glass.Glass{Clarity: a.glassClarity(0.4)})
			head.Draw(a.userWash(tint, 28))
		}
		head.Children(func() {
			a.userPhoto(c, u.AvatarURL, onGlass)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
				ui.Text(c, "用户").FontSize(12).FontWeight(600).TextColor(p.accent)
				ui.Text(c, u.Nickname).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
				ui.Text(c, fmt.Sprintf("%s 关注 · %s 粉丝", count(u.Follows), count(u.Followeds))).FontSize(13).TextColor(p.muted)
				if u.Signature != "" {
					ui.Text(c, u.Signature).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45).Margin(2, 0, 0, 0)
				}
			})
		})
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
		if own && d.podcastsShown {
			a.section(c, "订阅的播客")
			if len(d.podcasts) == 0 {
				a.empty(c, "还没有订阅播客")
			}
			ui.Grid(c).Columns(a.columns(c, 340)).GapX(8).GapY(4).Margin(0, -12).Children(func() {
				for i := range d.podcasts {
					a.podcastRow(c.Key(d.podcasts[i].ID), &d.podcasts[i])
				}
			})
		}
		if own && d.artistsShown {
			a.section(c, "关注的歌手")
			if len(d.artists) == 0 {
				a.empty(c, "还没有关注歌手")
			}
			ui.Grid(c).Columns(a.columns(c, 132)).GapX(20).GapY(28).Children(func() {
				for i := range d.artists {
					a.artistCard(c.Key(d.artists[i].ID), &d.artists[i])
				}
			})
		}
	})
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
