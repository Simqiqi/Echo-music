package app

import (
	"context"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
	"github.com/simqiqi/simqiqi-music/internal/sidecar"
)

type pageKind int

const (
	pageHome pageKind = iota
	pageDaily
	pageCharts
	pagePlaylist
	pageSearch
	pageFM
	pageArtist
	pageAlbum
	pageUser
	pagePodcast
	pagePodcasts
	pageWeekly
	pageWeather
	pageDownloads
	pageFeed
	pageNewAlbums
)

type route struct {
	page pageKind
	// id is the playlist, artist, album, user or podcast the page shows.
	id int64
	// query and kind are what a search page searched for.
	query string
	kind  netease.SearchType
}

// app is the state both windows show. Everything in it is read and changed
// on the main thread: by the views, and by goroutines through post.
type app struct {
	win    *mygo.Window
	notch  *notch
	mini   miniPlayer // the card that stands in the window's place
	tray   *mygo.Tray // the icon in the menu bar or notification area, while it shows
	art    *artCache
	pl     *player.Player
	pal    palette
	wash   wallpaper // the colors the app opened onto: see newWallpaper
	opened time.Time // when it opened: the clock the wallpaper breathes by

	route     route
	history   []route
	forward   []route // the pages gone back from, the last first to go to
	rail      bool    // the directory folded into its icon-only rail
	sideAnimW float32 // the column's width as it glides open or shut

	profile         *netease.Profile
	qishui          *sidecar.Account           // 汽水音乐 登录态，nil 表示未登录
	bridge          *bridgeStore               // 汽水音乐的歌曲与歌单，负数合成 ID 的注册表
	bridgeVMs       map[providerID]*bridgeRows // 侧栏的行，只在歌单有变化时重建
	qishuiLists     []sidecar.BridgePlaylist   // 汽水账号的歌单，侧栏列出
	qishuiPlErr     string                     // 歌单加载失败的原因
	qishuiCollapsed bool
	mine, saved     []netease.Playlist // the listener's playlists: made, and saved
	recPlaylists    []netease.Playlist // the day's playlists picked for the listener
	login           loginState
	topProv         providerID         // 顶栏头像显示哪个账号（最近登录的）
	accountOpen     bool               // the dropdown of the account row shows
	naming          nameDialog         // the name of a playlist asked for
	deleting        deleteDialog       // whether to delete a playlist, asked
	liked           map[int64]bool     // the songs the listener likes
	toast           string             // a message to show once
	saveImage       func([]byte) error // where a lyric card goes: the clipboard, unless a test says
	checking        bool               // a connection check is out, waiting for its probes
	checks          []checkResult      // what the last connection check found
	updatePeek      updatePeek         // the newest release heard of, if newer
	picked          pickedUp           // the song a login picked up, waiting to play

	// The episodes the listener likes, and doesn't, as far as NetEase has
	// answered, and those asked about: see ui_like.go.
	likedPrograms, askedPrograms map[int64]bool

	daily       []netease.Track
	dailySpin   spinState // the sheen that turns on the daily card's playing disc
	heroSpin    heroTurn  // the lead record's angle, held while paused
	heroBars    barState  // the spectrum along the lead record's foot
	dailyBars   barState  // the bars on its play button
	dailyWave   barState  // the wave of sound that crosses the daily card
	dailyHue    float64   // where the rainbow across the daily card begins
	dailyHued   bool      // whether a hue has been taken yet
	npBars      barState  // the spectrum under Now Playing's cover
	featured    []netease.Playlist
	charts      []netease.Playlist
	homeErr     string
	homeLoading bool
	homeAt      time.Time // when what the home page shows was asked for

	playlists  map[int64]*netease.Playlist
	loadingPl  map[int64]bool
	plErr      map[int64]string
	artists    map[int64]*artistData
	artistTab  int // what an artist's page lists: artistHot, artistLatest, artistAlbums or artistEvents
	albums     map[int64]*albumData
	users      map[int64]*userData
	podcasts   map[int64]*podcastData
	podcastTab *podcastTab // the podcasts tab's lists
	trackList  ui.ListState
	// Where the page and the sidebar are scrolled, kept while Now Playing
	// covers them, unbuilt.
	pageScroll ui.ScrollState
	navScroll  ui.ScrollState
	scrolled   route // the page trackList and pageScroll are of

	mineCollapsed, savedCollapsed bool

	query       string
	searchBox   ui.Handle
	searches    map[route]*searchData // the results of each search, by its page
	searched    route                 // the search last shown, which the sidebar goes back to
	suggestions suggestState          // what the search field's query may be after

	nowPlaying  bool
	npOpen      bool      // Now Playing as it last showed: open or not
	npChangedAt time.Time // when it last opened or closed
	npPane      int       // paneLyrics, paneQueue, paneStage, paneComments or paneProgram
	npBare      bool      // Tab has put Now Playing's chrome away
	npPokeAt    time.Time // when the pointer last moved over Now Playing
	npPokeX     float32   // where it stood when it last moved
	npPokeY     float32
	npPokeSet   bool      // whether it has moved there since the view opened
	npBackUntil time.Time // the countdown the way back was last put on the clock for
	npTabsLeft  time.Time // when the pointer last left the tab row
	npTabsV     float32   // how far the tab row is unfolded this frame
	heroHover   bool      // the pointer is on the home page's lead record
	npPaneTop   float32   // how far the cover beside the pane starts below its top
	// The tabs' boxes of the last build, and whether a press is being
	// carried across them: a drag chooses whichever tab is under it.
	npTabBoxes []ui.Rect
	npTabPanes []int
	npTabRowX  float32
	npTabPadX  float32 // the tabs' side padding, eased down on a narrow view
	npTabDrag  bool
	// The measured width of the tabs, cached until a label or the view's
	// width changes: measuring text on every frame would cost the view its
	// frame budget.
	npTabWidthKey  string
	npTabWidthRoom float32
	npTabWidthVal  float32
	// The lyric card's own picking: which lines the listener chose, and
	// the lines as they were when the picker opened, so playback moving on
	// does not shuffle what is being picked.
	sharePickOpen  bool
	sharePickTrack int64
	shareSel       []bool
	shareLines     []lyricPick
	shareScroll    ui.ScrollState
	// The clock and the smoothed spectrum the three visual panes share.
	visual      visualState
	lyricScroll ui.ScrollState
	lyricGlide  lyricGlide
	lyricAt     int
	lyricTrack  int64
	queueList   ui.ListState
	queueAt     int       // the row of the queue pane last scrolled to, -1 while it is hidden
	queueDrag   queueDrag // the queue's row being carried elsewhere, if one is
	comments    commentState
	tabCount    rollState // the count on the comments' tab, as it rolls

	fmMode        int    // the FM mode chosen, in fmModes
	radioStarting string // the radio asked for its first songs, or ""
	fmSeed        string // the song the heart mode began with, named for its reason
	sky           fmSky  // the landscape the FM's card wears

	scrub      float64
	scrubbing  bool
	volume     float64
	volumeOpen bool // the volume popover of the player bar shows
	mutedFrom  float64
	sleep      *sleepTimer // the sleep timer, if the listener set one

	rowBars barState
	// underTop and underBottom say whether the page has content under its
	// bars, which show their scroll edges then.
	underTop, underBottom bool
	navTitle              string // the section the capsule over the page names
	barScrub              scrubState
	barSong               scrubState // the bar's own progress bar, on its left
	npScrub               scrubState
	backdropCache         backdropCache           // the backdrop of Now Playing, as last painted
	npArt                 *artwork                // the cover whose colors Now Playing shows
	swatches              map[string]*swatchCache // the picker's little pictures of each backdrop
	homeArtURL            string                  // the cover whose tint the home's background holds
	homeTint              ui.Color                // the last tint the home's background took
	songAccent            ui.Color                // the accent the chrome wears, of the song playing
	npCoverState          coverState              // the cover of Now Playing, as it is tilted and flicked
	wallState             wallState               // the wall of the queue's covers, as it is dragged
	wall3                 wall3State              // the wall of covers turning in depth, as it is spun
	recAt, recLyric       ui.Rect                 // where the particles page stands its record, and the line being sung
	recTilt               tiltState               // how the record leans after the pointer
	pulse                 pulseState              // the bass of the song playing, for the record to beat to
	miniSpec              miniSpectrumState       // the little spectrum of the bar, as it falls
	weekly                *weeklyState            // the week's listening, as the app hears it

	// The three pages beside the playlists: what the listener kept on disk,
	// the feed of who they follow, and the albums just out.
	downloads downloadsState
	feed      []netease.Event
	feedState paged
	feedLast  int64
	newAlbums newAlbumsState
	weekSky   weekSweep // the light that crosses the week's poster

	wx       *wxState   // the sky outside, and the music it asks for
	quiet    quietState // the quiet parts of the song playing, for its hot comments
	sig      playerSig
	quitting bool

	settings     settings
	settingsOpen bool  // the settings card covers the page
	cacheUsed    int64 // the bytes the song cache takes, -1 until measured
	updates      bool  // the update plugin is used (useUpdater)

	restored     bool   // playback picked up where it was left
	savedSession string // the session as last kept
	sessionDue   bool   // a save of the session is due
}

// post runs fn on the main thread and redraws both windows. Without a
// window, as in tests, it drops fn.
func (a *app) post(fn func()) {
	if a.win == nil {
		return
	}
	a.win.Update(func() {
		fn()
		a.redraw()
	})
}

func (a *app) redraw() {
	if a.win != nil {
		a.win.Invalidate()
	}
	if a.mini.win != nil {
		a.mini.win.Invalidate()
	}
	if a.notch != nil && a.notch.win != nil {
		a.notch.win.Invalidate()
	}
}

// shotHook, when set, drives the app to check its design.
var shotHook func(a *app)

func (a *app) showMain() {
	if a.win != nil {
		// The mini player stands in for the window: it goes as it comes.
		if a.mini.win != nil && a.mini.win.IsVisible() {
			a.mini.win.Hide()
		}
		if a.win.IsMinimized() {
			a.win.Restore()
		}
		a.win.Show()
		a.win.Focus()
		// From the menu bar alone the app may be inactive.
		mygo.App.Focus()
	}
}

// Run runs the app until it quits.
func Run() error {
	a := &app{
		playlists:  map[int64]*netease.Playlist{},
		loadingPl:  map[int64]bool{},
		plErr:      map[int64]string{},
		artists:    map[int64]*artistData{},
		albums:     map[int64]*albumData{},
		users:      map[int64]*userData{},
		podcasts:   map[int64]*podcastData{},
		podcastTab: &podcastTab{},
		searches:   map[route]*searchData{},
		lyricAt:    -1,
		queueAt:    -1,
		queueDrag:  queueDrag{from: -1},
		cacheUsed:  -1,
		sleep:      &sleepTimer{},
		wash:       newWallpaper(),
		opened:     time.Now(),
	}
	moveOldData()
	a.useUpdater()
	a.watchUpdates()
	mygo.App.WhenReady(func() {
		a.settings = loadSettings()
		a.settings.applyAppearance()
		a.npPane = a.lyricsPaneChosen()
		// Before the window: on Windows it lays out its content as it is
		// made, and the view asks the player what plays.
		a.art = newArtCache(a.post)
		a.pl = player.New(a.post)
		a.bridge = newBridgeStore()
		a.pl.Source = a.bridgeSource
		a.pl.BridgeLyrics = a.bridgeLyrics
		a.pl.Quality = func() string { return a.settings.Quality }
		a.pl.EQ = player.NewEQ()
		a.applyEq()
		a.pl.Normalize = func() bool { return a.settings.Normalize }
		a.pl.Transition = func() bool { return a.settings.transitions() }
		a.pl.Cache = newSongCache(a.settings.CacheMB)
		a.pl.Changed = func() {
			a.keepSession()
			// A play or pause pressed from a media key or Control Center
			// has no click to redraw for it: say so, or the home page's
			// record keeps its last frame until something else moves.
			a.redraw()
		}
		a.pl.Diagnose = noteDiagnostic
		a.weekly = newWeekly()
		// The weather is read from what was kept before the window
		// stands; asking the sky waits until there is a window to post
		// the answer to.
		a.wx = newWx()
		a.wx.art = a.art
		// The week's listening, heard a quarter minute at a time.
		go func() {
			t := time.NewTicker(15 * time.Second)
			for now := range t.C {
				if tr := a.pl.Current(); tr != nil && a.pl.Playing() {
					a.weekly.add(now, *tr, 15)
				}
			}
		}()
		a.volume = a.settings.Volume
		a.pl.SetVolume(a.volume)
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:         mygo.App.Name(),
			Width:         1180,
			Height:        780,
			MinWidth:      880,
			MinHeight:     580,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHidden,
			// As tall as the edge over a page (Linux, Windows).
			TitleBarHeight: buttonsHeight,
			// Inset from the corner, within the sidebar, as macOS 26's
			// apps with a sidebar place them.
			TrafficLightPosition: &mygo.Point{X: 20, Y: 20},
			Vibrancy:             mygo.VibrancySidebar,
			BackgroundColor:      "light-dark(#ffffff, #141416)",
			Content:              ui.View(a.view),
		})
		a.setMenu()
		a.boot()
		// Now that there is a window to post the answer to, the sky is
		// asked for: the page will have it before it is opened.
		a.wx.warm(a.post)
		// The notch player is macOS's, over its menu bar: elsewhere it would
		// hang at the top of the screen as a black box.
		if runtime.GOOS == "darwin" {
			a.notch = newNotch(a)
		}
		// From the menu bar alone, the Dock icon shows while the window
		// does.
		a.syncDock()
		a.win.OnShow(a.syncDock)
		a.win.OnHide(a.syncDock)
		// Shown or switched to the morning after, it shows the new day's.
		a.win.OnFocus(a.freshenHome)
		a.applyTray()
		if shotHook != nil {
			shotHook(a)
		}
		// Closing the window hides it: music plays on, and the Dock icon
		// or the tray's brings it back. A Linux desktop without a tray
		// has neither, so there closing it quits.
		a.win.OnClose(func(e *mygo.CloseEvent) {
			if a.quitting {
				return
			}
			e.PreventDefault()
			if runtime.GOOS == "darwin" || a.tray != nil {
				a.win.Hide()
			} else {
				mygo.App.Quit()
			}
		})
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.quitting = true
		a.settings.Volume = a.volume
		a.settings.save()
		a.saveSession()
	})
	// Windows leaves an icon in the notification area after its app until
	// the pointer passes over it.
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if a.tray != nil {
			a.tray.Destroy()
		}
		// 关掉登录桥接进程，免得它成为孤儿。
		sidecar.StopShared()
	})
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnActivate(func(bool) { a.showMain() })
	return mygo.App.Run()
}

// downloadTrack downloads a track to the local filesystem.
func (a *app) downloadTrack(tr *netease.Track) {
	if tr == nil {
		return
	}
	// Show a toast to indicate download started
	a.toast = "正在下载：" + tr.Name
	a.redraw()

	go func() {
		ctx := context.Background()
		_, err := a.pl.Download(ctx, tr.ID, "standard")
		a.post(func() {
			if err != nil {
				a.toast = "下载失败：" + err.Error()
			} else {
				a.toast = "下载完成：" + tr.Name
			}
			a.redraw()
		})
	}()
}
