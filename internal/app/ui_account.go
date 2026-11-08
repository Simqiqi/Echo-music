package app

import (
	"encoding/base64"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"rsc.io/qr"

	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/qishui"
	"github.com/simqiqi/simqiqi-music/internal/sidecar"
)

// providerID names a music service you can sign into. NetEase keeps driving
// the library; 汽水音乐 logs in through the Node sidecar and lives
// alongside it, their sessions saved to the same keychain.
type providerID int

const (
	provNone providerID = iota
	provNetease
	provQishui
)

func (p providerID) name() string {
	switch p {
	case provQishui:
		return "汽水音乐"
	default:
		return "网易云音乐"
	}
}

// loginState is a QR code login under way, for any of the providers.
type loginState struct {
	open     bool
	gen      int // counts logins, to stop the polls of closed ones
	provider providerID
	key      string
	code     *qr.Code   // NetEase's own QR
	image    *ui.Bitmap // 汽水音乐 PNG QR from the bridge
	status   int        // NetEase's QR state, or 0 while the code loads
	state    string     // 统一状态：waiting|scanned|confirmed|expired
	scanner  string     // the account that just scanned / signed in
	avatar   string     // its avatar, to greet on success
	err      string
	finished bool
}

// boot signs in with the session in the keychain, if any, then loads the
// home page for whoever is listening. The NetEase library loads first; the
// 汽水音乐 session restores a moment later, so a slow network there never
// holds up the home page.
func (a *app) boot() {
	a.homeLoading = true
	go func() {
		var prof *netease.Profile
		if netease.RestoreSession() {
			p, err := netease.Account()
			switch {
			case err == nil && p == nil:
				netease.ClearSession() // expired
			case err == nil:
				prof = p
			}
		}
		// 汽水音乐的桥接进程可能比这里慢半拍，恢复失败就重试几次，
		// 否则一重启就丢了登录态（之前头像因此回退成网易云）。
		qi, qiOk := a.restoreBridgeLogin(provQishui)
		a.post(func() {
			a.signedIn(prof)
			if qiOk {
				a.qishui = qi
				a.loadBridgeLists(provQishui)
			}
			a.finalizeTopAccount()
		})
	}()
}

// restoreBridgeLogin brings a sidecar provider's session back from the
// keychain into the bridge, retrying while the bridge process is still
// coming up. It returns the account it restored, if any.
func (a *app) restoreBridgeLogin(prov providerID) (*sidecar.Account, bool) {
	var restore func() (bool, error)
	var profile func() (*sidecar.Account, error)
	switch prov {
	case provQishui:
		restore, profile = qishui.Restore, qishui.Profile
	default:
		return nil, false
	}
	var acc *sidecar.Account
	for attempt := 0; attempt < 12; attempt++ {
		ok, err := restore()
		if err == nil && ok {
			if got, perr := profile(); perr == nil && got != nil {
				acc = got
				break
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
	return acc, acc != nil
}

// signedIn shows what is personal to prof, or to nobody.
func (a *app) signedIn(prof *netease.Profile) {
	a.profile = prof
	// Playback picks up where it was left, once the session that lets
	// the listener play their songs is back.
	a.restoreSession()
	a.mine, a.saved, a.recPlaylists, a.liked = nil, nil, nil, nil
	a.likedPrograms, a.askedPrograms = nil, nil
	// What the listener may play, and follows, changed: load the pages
	// again.
	a.playlists = map[int64]*netease.Playlist{}
	a.plErr = map[int64]string{}
	a.artists, a.albums, a.users, a.podcasts = map[int64]*artistData{}, map[int64]*albumData{}, map[int64]*userData{}, map[int64]*podcastData{}
	a.podcastTab = &podcastTab{}
	a.searches = map[route]*searchData{}
	a.load(a.route)
	a.loadHome()
	if prof == nil {
		return
	}
	// 顶栏账号改由 finalizeTopAccount 统一决定（尊重用户的偏好，也不因
	// 网易云登录把汽水音乐挤掉），这里不再独占 topProv。
	a.loadLiked(prof.UserID)
	a.loadMine(prof.UserID)
}

// openLogin begins a NetEase sign-in, the app's library account. The nine
// callers that reach it (like, comments, artist…) mean NetEase, so this
// keeps that meaning.
func (a *app) openLogin() {
	a.openLoginProvider(provNetease)
}

// openLoginProvider begins a QR sign-in for prov, NetEase or the sidecar
// providers.
func (a *app) openLoginProvider(prov providerID) {
	a.login = loginState{open: true, gen: a.login.gen + 1, provider: prov}
	gen := a.login.gen
	switch prov {
	case provNetease:
		a.startNeteaseLogin(gen)
	case provQishui:
		a.startBridgeLogin(gen, provQishui, qishui.StartQR, qishui.CheckQR,
			func() (*sidecar.Account, error) { return qishui.Profile() }, qishui.Save)
	}
}

func (a *app) startNeteaseLogin(gen int) {
	go func() {
		key, err := netease.QRKey()
		var code *qr.Code
		if err == nil {
			code, err = qr.Encode(netease.LoginURL(key), qr.M)
		}
		a.post(func() {
			if gen != a.login.gen {
				return
			}
			if err != nil {
				a.login.err = "无法获取二维码，请检查网络"
				return
			}
			a.login.key, a.login.code, a.login.status = key, code, netease.QRWaiting
			a.pollLogin(gen, key)
		})
	}()
}

// startBridgeLogin begins a QR login for a sidecar provider: it fetches the
// QR as a PNG and decodes it for display, then polls until it finishes.
func (a *app) startBridgeLogin(gen int, prov providerID,
	start func() (string, string, error),
	check func(string) (string, error),
	profile func() (*sidecar.Account, error),
	save func() error) {
	go func() {
		key, image, err := start()
		var bmp *ui.Bitmap
		if err == nil && image != "" {
			bmp, err = decodeLoginQR(image)
		}
		a.post(func() {
			if gen != a.login.gen {
				return
			}
			if err != nil {
				a.login.err = "无法获取二维码，请确认已安装 Node.js 且网络畅通"
				return
			}
			a.login.key, a.login.image, a.login.state = key, bmp, "waiting"
			a.pollBridgeLogin(gen, prov, key, check, profile, save)
		})
	}()
}

// decodeLoginQR turns the QR the bridge returned into a bitmap. The sidecar
// may hand back a bare base64 string (its usual form), a data: URI it kept
// the prefix of, or an http(s) URL to fetch. The old code assumed base64
// and failed the moment a provider returned a URL, which left the QR blank
// on 汽水音乐 — the "登陆二维码有问题" the listener hits there.
func decodeLoginQR(image string) (*ui.Bitmap, error) {
	var raw []byte
	var err error
	switch {
	case strings.HasPrefix(image, "http://"), strings.HasPrefix(image, "https://"):
		raw, err = netease.Fetch(image)
	default:
		// A data: URI keeps its comma; strip it so only the base64 remains.
		if i := strings.Index(image, ","); i >= 0 {
			image = image[i+1:]
		}
		raw, err = base64.StdEncoding.DecodeString(image)
	}
	if err != nil {
		return nil, err
	}
	return ui.DecodeBitmap(raw)
}

// pollLogin asks every two seconds whether the NetEase code was scanned,
// until the login ends or the dialog closes.
func (a *app) pollLogin(gen int, key string) {
	go func() {
		for {
			time.Sleep(2 * time.Second)
			code, msg, name, avatar, err := netease.QRCheck(key)
			var prof *netease.Profile
			if code == netease.QRConfirmed {
				netease.SaveSession()
				prof, _ = netease.Account()
			}
			done := make(chan bool, 1)
			a.post(func() {
				l := &a.login
				if gen != l.gen || !l.open {
					done <- true
					return
				}
				switch {
				case err != nil:
					// Try again: the network may come back.
				case code == netease.QRConfirmed:
					l.status, l.finished = code, true
					a.signedIn(prof)
					if prof != nil {
						a.followAccount(provNetease)
						a.pickUp(prof.UserID)
					}
					a.closeLoginSoon(gen)
				case code == netease.QRExpired || code == netease.QRWaiting || code == netease.QRScanned:
					l.status = code
					if name != "" {
						l.scanner, l.avatar = name, avatar
					}
				default:
					l.err = msg
					if l.err == "" {
						l.err = "登录失败，请稍后再试"
					}
				}
				done <- l.status == netease.QRExpired || l.finished || l.err != ""
			})
			if <-done {
				return
			}
		}
	}()
}

// pollBridgeLogin polls a sidecar provider's QR the same way, mapping its
// string states onto the dialog, and saves the session on confirmation.
func (a *app) pollBridgeLogin(gen int, prov providerID, key string,
	check func(string) (string, error),
	profile func() (*sidecar.Account, error),
	save func() error) {
	go func() {
		for {
			time.Sleep(2 * time.Second)
			state, err := check(key)
			var acc *sidecar.Account
			var saveErr error
			if state == "confirmed" {
				saveErr = save()
				acc, _ = profile()
			}
			done := make(chan bool, 1)
			a.post(func() {
				l := &a.login
				if gen != l.gen || !l.open {
					done <- true
					return
				}
				switch {
				case err != nil:
					// Try again: the network may come back.
				case state == "confirmed":
					l.state, l.finished = "confirmed", true
					if saveErr != nil {
						// The login holds for this run, but it will not
						// come back next time: say so rather than let the
						// listener find an empty app on reopening.
						noteDiagnostic(prov.name()+"登录态保存", saveErr)
					}
					if acc != nil {
						l.scanner, l.avatar = acc.Name, acc.Avatar
						a.followAccount(prov)
						switch prov {
						case provQishui:
							a.qishui = acc
							a.loadBridgeLists(provQishui)
						}
					}
					a.closeLoginSoon(gen)
				case state == "expired":
					l.state = "expired"
				case state == "scanned":
					l.state = "scanned"
				case state == "waiting":
					l.state = "waiting"
				default:
					l.err = state
				}
				done <- l.state == "expired" || l.finished || l.err != ""
			})
			if <-done {
				return
			}
		}
	}()
}

// closeLoginSoon closes the dialog a moment after it showed the welcome.
func (a *app) closeLoginSoon(gen int) {
	go func() {
		time.Sleep(1200 * time.Millisecond)
		a.post(func() {
			if gen == a.login.gen {
				a.login.open = false
			}
		})
	}()
}

func (a *app) signOut() {
	a.logoutProvider(provNetease)
}

// logoutProvider signs out the chosen provider. NetEase clears the library
// too; 汽水音乐 only forgets its own session.
func (a *app) logoutProvider(prov providerID) {
	a.login.gen++
	switch prov {
	case provNetease:
		a.signedIn(nil)
		go netease.Logout()
	case provQishui:
		a.qishui, a.qishuiLists, a.qishuiPlErr = nil, nil, ""
		go qishui.Logout()
	}
	// 退出的是顶栏那位，就清掉偏好、换成还登着的另一家。
	if providerID(a.settings.TopAccount) == prov {
		a.settings.TopAccount = int(provNone)
		a.settings.TopAccountManual = false
		a.settings.save()
	}
	a.finalizeTopAccount()
}

// providerAccent is the brand color each login uses, so the dialog reads
// as 网易云 / 汽水音乐 at a glance.
func providerAccent(prov providerID) ui.Color {
	switch prov {
	case provQishui:
		return ui.Hex("#12C2B4") // 汽水 青绿
	default:
		return ui.Hex("#C20C0C") // 网易云 红
	}
}

// loginDialog shows the QR code to scan with the matching app: a centered
// card with a platform pill switcher on top, the title above a brand-ringed
// QR, and the status beneath.
func (a *app) loginDialog(c *ui.Context) {
	p := a.pal
	l := &a.login

	title, sub := "扫码登录", "打开"+l.provider.name()+" App，扫一扫登录"
	scanned := (l.provider == provNetease && l.status == netease.QRScanned) || l.state == "scanned"
	confirmed := l.finished
	expired := (l.provider == provNetease && l.status == netease.QRExpired) || l.state == "expired"
	switch {
	case confirmed:
		title, sub = "登录成功", "欢迎回来，"+a.loginName()
	case l.err != "":
		sub = l.err
	case scanned:
		title, sub = "扫描成功", "请在手机上确认登录"
	case expired:
		sub = "二维码已失效"
	}

	ui.DialogBase(c.Key("login"), &l.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(320).Padding(24, 22, 24, 20).Radius(24).Gap(14).AlignItems(ui.Center).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})

		// 平台选择：网易云 / 汽水音乐。providers 的顺序与标签一一对应，
		// 这样 segment 的 0 基下标就能正确映射到 providerID，而不会因为
		// provNone 占用了 0 而整体错位（之前选「汽水音乐」会去启动网易云的二维码）。
		providers := []providerID{provNetease, provQishui}
		sel := 0
		for i, p := range providers {
			if p == a.login.provider {
				sel = i
				break
			}
		}
		if segmented(c, &sel, "网易云", "汽水音乐") && a.login.open {
			a.post(func() { a.openLoginProvider(providers[sel]) })
		}
		a.login.provider = providers[sel]
		accent := providerAccent(l.provider)

		ui.Text(c, title).FontSize(22).FontWeight(700)
		ui.Text(c, sub).FontSize(13).TextColor(p.muted).TextAlign(ui.Center)

		// 二维码卡：外圈用平台品牌色，内白底，状态层叠在中间。
		ui.Box(c).Size(210, 210).Radius(26).Background(accent).Center().Children(func() {
			ui.Box(c).Size(200, 200).Radius(20).Background(ui.RGB(255, 255, 255)).Border(1, p.hairline).Children(func() {
				if l.code != nil {
					code := l.code
					opacity := float32(1)
					if scanned || expired || confirmed || l.err != "" {
						opacity = 0.1
					}
					ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Opacity(opacity).
						Draw(func(pt *ui.Painter, r ui.Rect) { drawQR(pt, r, code, 14) })
				} else if l.image != nil {
					opacity := float32(1)
					if scanned || expired || confirmed || l.err != "" {
						opacity = 0.1
					}
					ui.Image(c, l.image).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover).Opacity(opacity)
				} else if l.err == "" {
					s := shimmer{base: ui.RGBA(0, 0, 0, 0.08), sheen: ui.RGBA(0, 0, 0, 0.03)}.in(c)
					qrSkeleton(c, s, 14).Absolute().Top(0).Left(0).Right(0).Bottom(0)
				}
				ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(10).Children(func() {
					switch {
					case confirmed || scanned:
						if l.avatar != "" {
							if bmp := bitmap(a.art.getAt(l.avatar, 76, false)); bmp != nil {
								pic := ui.Box(c).Size(76, 76).Radius(38).Shadow(0, 6, 16, -4, ui.RGBA(0, 0, 0, 0.3))
								in := artIn(pic, bmp)
								pic.Children(func() { artImage(c, bmp, 38, in) })
							}
						}
						if l.scanner != "" {
							ui.Text(c, l.scanner).FontSize(14).FontWeight(600).TextColor(ui.Hex("#1d1d1f"))
						}
					case expired || l.err != "":
						if pillButton(c.Key("refresh"), iconRepeat, "刷新二维码", accent, ui.RGB(255, 255, 255)).Clicked() {
							a.openLoginProvider(l.provider)
						}
					}
				})
			})
		})
		ui.Text(c, "登录后，每日推荐与歌单将根据你的口味生成").FontSize(11.5).TextColor(p.faint)
	})
}

// loginName is who just signed in, for the welcome line.
func (a *app) loginName() string {
	if a.login.scanner != "" {
		return a.login.scanner
	}
	return a.profileName()
}

// drawQR draws a QR code in r, as crisp squares on white, inside a quiet
// zone of pad.
func drawQR(p *ui.Painter, r ui.Rect, code *qr.Code, pad float32) {
	n := code.Size
	cell := (min(r.W, r.H) - 2*pad) / float32(n)
	x0 := r.X + (r.W-cell*float32(n))/2
	y0 := r.Y + (r.H-cell*float32(n))/2
	ink := ui.Hex("#141416")
	for y := range n {
		for x := range n {
			if code.Black(x, y) {
				// A hair larger, so neighbors join without seams.
				p.Fill(ui.Rect{X: x0 + float32(x)*cell, Y: y0 + float32(y)*cell, W: cell + 0.3, H: cell + 0.3}, ink, 0)
			}
		}
	}
}

func (a *app) profileName() string {
	if a.profile == nil {
		return ""
	}
	return a.profile.Nickname
}

// firstSignedIn names the account to fall back to: NetEase, then 汽水音乐.
func (a *app) firstSignedIn() providerID {
	switch {
	case a.profile != nil:
		return provNetease
	case a.qishui != nil:
		return provQishui
	}
	return provNetease
}

// accountSignedIn reports whether prov's account is currently loaded.
func (a *app) accountSignedIn(prov providerID) bool {
	switch prov {
	case provNetease:
		return a.profile != nil
	case provQishui:
		return a.qishui != nil
	}
	return false
}

// resolveTopAccount picks the account the top bar shows: the listener's
// saved choice when it is still signed in, else the 汽水音乐 account over
// 网易云 by default, else whatever is signed in.
func (a *app) resolveTopAccount() providerID {
	// Only a pick the listener made themselves pins the bar; a value an old
	// build's sign-in wrote is left over, not a choice.
	if pref := providerID(a.settings.TopAccount); a.settings.TopAccountManual && pref != provNone && a.accountSignedIn(pref) {
		return pref
	}
	if a.qishui != nil {
		return provQishui
	}
	if a.profile != nil {
		return provNetease
	}
	return provNone
}

// finalizeTopAccount recomputes the top bar's account after logins restore
// or change, so it no longer depends on the fragile order of boot posts.
func (a *app) finalizeTopAccount() {
	a.topProv = a.resolveTopAccount()
}

// setTopAccount makes prov the top bar's account and remembers the choice,
// because the listener asked for it ("设为常用").
func (a *app) setTopAccount(prov providerID) {
	a.topProv = prov
	a.settings.TopAccount = int(prov)
	a.settings.TopAccountManual = true
	a.settings.save()
}

// followAccount puts a freshly signed-in account in the top bar without
// pinning it: signing in is not the same as choosing a favourite, and a
// pinned 网易云 is what kept 汽水音乐's avatar hidden after a restart. A
// pick the listener made themselves still wins while that account is
// signed in.
func (a *app) followAccount(prov providerID) {
	if pref := providerID(a.settings.TopAccount); a.settings.TopAccountManual && pref != provNone && a.accountSignedIn(pref) {
		return
	}
	a.topProv = prov
}

// topIdentity is whose avatar the top bar shows: the account signed in
// most recently, or any that still is. 网易云 drives the library pages,
// but the row shows whoever the listener last logged in as.
func (a *app) topIdentity() (provID providerID, name, avatar string, ok bool) {
	provID = a.topProv
	if _, _, signed := a.accountInfo(provID); !signed {
		provID = a.firstSignedIn()
	}
	name, avatar, ok = a.accountInfo(provID)
	return provID, name, avatar, ok
}

// topAccount sits at the right of the top bar: the avatar of whichever
// account signed in last, round, with a dropdown over all three accounts
// and the settings.
func (a *app) topAccount(c *ui.Context) {
	p := a.pal
	b := ui.ButtonBase(c.Key("account")).Size(30, 30).Center().Radius(999).FocusRing(false).Label("账户")
	prov, name, avatarURL, ok := a.topIdentity()
	if !ok {
		a.accountOpen = false
		b.Label("登录").Children(func() {
			ui.Box(c).Size(28, 28).Radius(999).Background(p.press).Center().Children(func() {
				ui.Icon(c, iconUser).Size(15, 15).TextColor(p.muted)
			})
		})
		if b.Clicked() {
			a.openLogin()
		}
		return
	}
	if b.Hovered() || a.accountOpen {
		b.Background(p.hover)
	}
	b.Label(name).Children(func() {
		if prov == provNetease {
			a.avatar(c, avatarURL, 26, p.press)
		} else {
			// 汽水音乐的头像 CDN 不认网易云的缩放参数。
			a.avatarRaw(c, avatarURL, 26, p.press)
		}
	})
	if b.Clicked() {
		a.accountOpen = !a.accountOpen
	}
	a.accountMenu(c, b)
}

// accountMenu drops down from the avatar at the top bar's right, in the
// colors of the volume popover: the two accounts first, then the
// listener's own page and the settings, and signing out.
func (a *app) accountMenu(c *ui.Context, row ui.Element) {
	pop := popoverColors(a.pal.dark)
	shortcut := "Ctrl+,"
	if runtime.GOOS == "darwin" {
		shortcut = "⌘,"
	}
	ui.PopoverBase(c.Key("account-menu"), row, &a.accountOpen, func(panel ui.Element) {
		panel.AttachTo(row, ui.AnchorBottomEnd, ui.AnchorTopEnd).Margin(0, 0, 0, 0).Padding(5).Gap(1).
			Radius(12).Background(pop.fill).Border(0.5, pop.edge).Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.2)).
			Role(ui.RoleMenu).FocusGroup(ui.Vertical).Label("账户").TransformOrigin(0.5, 0).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.96, Y: -4}, Exit: &ui.Motion{Scale: 0.98}})
		const w = 248
		panel.Width(w)

		// 两个平台的账号状态：已登录显示昵称与头像，未登录可一键登录。
		for _, prov := range []providerID{provNetease, provQishui} {
			a.accountRow(c, prov)
		}

		ui.Box(c).Height(0.5).Margin(4, 8).Background(pop.edge)
		// 「我的主页」与「退出登录」说的是网易云账号；只登了汽水音乐时
		// 不出现，那两家的退出在各自那一行上。
		first := true
		if a.profile != nil {
			// 我的主页 is 网易云's: a listener signed in to 汽水音乐 alone
			// has no page here to open.
			home := a.menuItem(c, iconUser, "我的主页", "")
			if first {
				home.AutoFocus()
				first = false
			}
			if home.Clicked() {
				a.accountOpen = false
				a.open(route{page: pageUser, id: a.profile.UserID})
			}
			ui.Box(c).Height(0.5).Margin(4, 8).Background(pop.edge)
		}
		settings := a.menuItem(c, iconSettings, "设置…", shortcut)
		if first {
			settings.AutoFocus()
			first = false
		}
		if settings.Clicked() {
			a.accountOpen = false
			a.openSettings()
		}
		if a.profile != nil {
			ui.Box(c).Height(0.5).Margin(4, 8).Background(pop.edge)
			if a.menuItem(c, iconSignOut, "退出登录", "").Clicked() {
				a.accountOpen = false
				a.signOut()
			}
		}
	})
}

// accountRow is one line of the account menu: the service's mark, its
// signed-in name, and a button to log in or out.
func (a *app) accountRow(c *ui.Context, prov providerID) {
	p := a.pal
	name, avatarURL, loggedIn := a.accountInfo(prov)
	row := ui.Row(c.Key("acct-"+prov.name())).Padding(6, 8).Gap(10).Radius(8).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(p.press)
	}
	row.Children(func() {
		if loggedIn && avatarURL != "" {
			a.avatarRaw(c, avatarURL, 24, p.press)
		} else {
			ui.Box(c).Size(24, 24).Radius(12).Background(p.hover).Center().Children(func() {
				ui.Text(c, string([]rune(prov.name())[0])).FontSize(11).FontWeight(700).TextColor(p.muted)
			})
		}
		ui.Column(c).Gap(0).Grow(1).Children(func() {
			ui.Text(c, prov.name()).FontSize(12.5).FontWeight(600).SingleLine()
			ui.Text(c, name).FontSize(11).TextColor(p.faint).SingleLine()
		})
		// 动作区：登录、退出，或把已登录但非顶栏的账号设为顶栏。
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if loggedIn && prov != a.topProv {
				set := ui.ButtonBase(c.Key("acct-" + prov.name() + "-top")).Height(24).PaddingX(10).Radius(6).
					AlignItems(ui.Center).FocusRing(false).Label("设为常用")
				if set.Hovered() {
					set.Background(p.hover)
				}
				set.Children(func() {
					ui.Text(c, "设为常用").FontSize(11.5).FontWeight(600).TextColor(p.accent)
				})
				if set.Clicked() {
					a.setTopAccount(prov)
					a.accountOpen = false
				}
			}
			if loggedIn && prov == a.topProv {
				now := ui.Box(c).Height(24).PaddingX(10).Radius(6).Center()
				now.Children(func() {
					ui.Text(c, "当前").FontSize(11.5).FontWeight(600).TextColor(p.faint)
				})
			}
			btn := ui.ButtonBase(c.Key("acct-" + prov.name() + "-act")).Height(24).PaddingX(10).Gap(0).Radius(6).
				AlignItems(ui.Center).FocusRing(false)
			if loggedIn {
				btn.TextColor(p.muted).Label("退出")
				if btn.Hovered() {
					btn.Background(p.hover)
				}
				btn.Children(func() {
					ui.Text(c, "退出").FontSize(11.5).FontWeight(600)
				})
				if btn.Clicked() {
					a.logoutProvider(prov)
				}
			} else {
				btn.TextColor(p.accent).Label("登录")
				if btn.Hovered() {
					btn.Background(p.hover)
				}
				btn.Children(func() {
					ui.Text(c, "登录").FontSize(11.5).FontWeight(600)
				})
				if btn.Clicked() {
					a.openLoginProvider(prov)
				}
			}
		})
	})
}

// accountInfo returns the signed-in name, avatar URL, and whether prov is
// signed in.
func (a *app) accountInfo(prov providerID) (name, avatar string, loggedIn bool) {
	switch prov {
	case provQishui:
		if a.qishui != nil {
			return a.qishui.Name, a.qishui.Avatar, true
		}
	default:
		if a.profile != nil {
			return a.profile.Nickname, a.profile.AvatarURL, true
		}
	}
	return "未登录", "", false
}

// avatarRaw draws a round avatar from url with no size hint, used for the
// 汽水音乐 marks whose CDNs ignore NetEase's resize parameter: the
// picture is brought down to the size it shows here instead.
func (a *app) avatarRaw(c *ui.Context, url string, size float32, empty ui.Color) {
	bmp := bitmap(a.art.getAt(url, int(size), false))
	box := ui.Box(c).Size(size, size).Radius(size / 2)
	in := artIn(box, bmp)
	if in < 1 {
		box.Background(empty)
	}
	box.Children(func() { artImage(c, bmp, size/2, in) })
}

// menuItem is an item of a dropdown: an icon, its label, and the shortcut
// that does the same, if any.
func (a *app) menuItem(c *ui.Context, icon *ui.SVG, label, shortcut string) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c.Key(label)).Role(ui.RoleMenuItem).Label(label).Height(30).PaddingX(8).Gap(9).Radius(7).
		AlignItems(ui.Center).Justify(ui.Start).FocusRing(false)
	if b.Hovered() || b.FocusVisible() {
		b.Background(p.press)
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(15, 15).TextColor(p.muted)
		ui.Text(c, label).FontSize(13).Grow(1)
		if shortcut != "" {
			ui.Text(c, shortcut).FontSize(12).TextColor(p.faint)
		}
	})
	return b
}

// myPlaylists lists the listener's playlists under a header that opens
// and closes them: those they made, if mine, with a button to make one
// and a menu to rename them.
func (a *app) myPlaylists(c *ui.Context, title string, lists []netease.Playlist, mine bool, collapsed *bool) {
	if len(lists) == 0 {
		return
	}
	p := a.pal
	ui.Row(c.Key(title)).AlignItems(ui.Center).Margin(18, 4, 6, 6).Children(func() {
		b := ui.ButtonBase(c.Key("toggle")).Grow(1).Height(26).PaddingX(4).Gap(6).Radius(6).
			AlignItems(ui.Center).Justify(ui.Start).Role(ui.RoleDisclosure).Label(title)
		if b.Clicked() {
			*collapsed = !*collapsed
		}
		b.Expanded(!*collapsed)
		if b.Hovered() {
			b.Background(p.hover)
		}
		b.Children(func() {
			ui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(p.faint).Grow(1)
			arrow := iconDown
			if *collapsed {
				arrow = iconForward
			}
			ui.Icon(c, arrow).Size(12, 12).TextColor(p.faint)
		})
	})
	if *collapsed {
		return
	}
	for i := range lists {
		pl := &lists[i]
		selected := a.route.page == pagePlaylist && a.route.id == pl.ID && !a.nowPlaying
		b := ui.ButtonBase(c.Key(pl.ID)).Height(34).PaddingX(8).Gap(10).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(pl.Name)
		switch {
		case selected:
			b.Background(p.press)
		case b.Hovered():
			b.Background(p.hover)
		}
		b.Children(func() {
			// The playlist's cover, and its name: the row opens the playlist.
			ui.Box(c).Size(22, 22).Radius(5).Shrink(0)
			a.cover(c, pl.Cover(), 80, 22, 5)
			ui.Text(c, pl.Name).FontSize(13).SingleLine().Shrink(1).MinWidth(0)
		})
		if mine && pl.SpecialType != 5 {
			b.ContextMenu(func(m *ui.Menu) {
				if m.Item("重命名…").Chosen() {
					a.startRename(pl)
				}
				m.Separator()
				if m.Item("删除歌单…").Chosen() {
					a.confirmDelete(pl)
				}
			})
		}
		if b.Clicked() {
			a.open(route{page: pagePlaylist, id: pl.ID})
		}
	}
}
