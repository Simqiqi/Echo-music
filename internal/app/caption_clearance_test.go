package app

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The window controls a hidden title bar draws at the top right on
// Windows: three buttons 46 DIP wide, in a bar as tall as the edge over a
// page. Nothing the app draws may sit under them, or it shows through the
// glass and takes the clicks meant for them — the maximize button read
// through a translucent capsule as a tofu block, once.
const (
	testWinW, testWinH     = 1180, 780
	testSmallW, testSmallH = 880, 580
	testButtonsW           = 138
	testButtonsH           = 46
)

// clearanceCase is one page or one pane of Now Playing to draw.
type clearanceCase struct {
	name string
	r    route
	pane int
}

// clearancePages walks the pages the sidebar reaches without signing in.
func clearancePages() []clearanceCase {
	return []clearanceCase{
		{"home", route{page: pageHome}, 0},
		{"daily", route{page: pageDaily}, 0},
		{"charts", route{page: pageCharts}, 0},
		{"fm", route{page: pageFM}, 0},
		{"weather", route{page: pageWeather}, 0},
		{"podcasts", route{page: pagePodcasts}, 0},
		{"downloads", route{page: pageDownloads}, 0},
		{"feed", route{page: pageFeed}, 0},
		{"newalbums", route{page: pageNewAlbums}, 0},
		{"weekly", route{page: pageWeekly}, 0},
	}
}

// clearancePanes walks every pane Now Playing can show.
func clearancePanes() []clearanceCase {
	return []clearanceCase{
		{"lyrics", route{}, paneLyrics},
		{"queue", route{}, paneQueue},
		{"stage", route{}, paneStage},
		{"comments", route{}, paneComments},
		{"wall", route{}, paneWall},
		{"wall3d", route{}, paneWall3D},
		{"particles", route{}, paneParticles},
	}
}

// clearanceTester draws with the Windows title bar in place, signed in,
// with a track cued so Now Playing can open.
func clearanceTester(t *testing.T, w, h int) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	tr := netease.Track{ID: 1, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000}
	a.pl.Cue([]netease.Track{tr})
	a.pl.SetLyrics(nil)
	tt := ui.NewTester(a.view, w, h)
	tt.SetTitleBar(ui.TitleBar{Height: testButtonsH, Right: testButtonsW})
	return a, tt
}

// frame settles one frame of work, as a pane's opening takes longer than
// a page's.
func frameSettled(tt *ui.Tester, pane bool) {
	tt.Frame()
	if pane {
		tt.Advance(700 * time.Millisecond)
	} else {
		tt.Advance(20 * time.Millisecond)
	}
	tt.Frame()
}

// frameLabels collects the text boxes of the last frame, one per distinct
// box, in reading order.
type labelled struct {
	name string
	box  ui.Rect
}

func frameLabels(tt *ui.Tester) []labelled {
	var out []labelled
	seen := map[ui.Rect]bool{}
	for _, s := range tt.Texts() {
		r, ok := tt.Find(s)
		if !ok || r.W <= 0 || r.H <= 0 || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, labelled{name: s, box: r})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].box.Y != out[j].box.Y {
			return out[i].box.Y < out[j].box.Y
		}
		return out[i].box.X < out[j].box.X
	})
	return out
}

// overlapArea is how much of the smaller of two boxes the other covers.
func overlapArea(a, b ui.Rect) float32 {
	x := max(a.X, b.X)
	y := max(a.Y, b.Y)
	r := min(a.X+a.W, b.X+b.W)
	t := min(a.Y+a.H, b.Y+b.H)
	if r <= x || t <= y {
		return 0
	}
	smallest := min(a.W*a.H, b.W*b.H)
	if smallest <= 0 {
		return 0
	}
	return (r - x) * (t - y) / smallest
}

// textOverlapsAbove returns the pairs of text boxes above y that overlap
// without one holding the other. A label on a card holds the texts of
// what it shows, so that pair is the card speaking for them, not one text
// over another; two texts crossing each other's edges is. A page scrolls
// under the player bar on purpose, so texts down there meeting the bar's
// own controls is the design rather than a clash — the caller gives the
// y the bar starts at.
func textOverlapsAbove(tt *ui.Tester, y float32) []string {
	boxes := frameLabels(tt)
	holds := func(outer, inner ui.Rect) bool {
		return inner.X >= outer.X-0.5 && inner.Y >= outer.Y-0.5 &&
			inner.X+inner.W <= outer.X+outer.W+0.5 && inner.Y+inner.H <= outer.Y+outer.H+0.5
	}
	var out []string
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i].box, boxes[j].box
			if holds(a, b) || holds(b, a) {
				continue
			}
			if a.Y+a.H > y && b.Y+b.H > y {
				continue
			}
			if c := overlapArea(a, b); c > 0.12 {
				out = append(out, fmt.Sprintf("%q%v over %q%v (%.0f%%)",
					boxes[i].name, a, boxes[j].name, b, c*100))
			}
		}
	}
	return out
}

// checkClearance asks one drawn frame its two questions: that no text
// reaches under the window controls, and that no two texts are drawn over
// one another. The player bar hangs over the foot of the h-tall window,
// and a page scrolls under it on purpose, so its band stays out of the
// second question.
func checkClearance(t *testing.T, tt *ui.Tester, w, h int, where string) {
	t.Helper()
	for _, s := range tt.Texts() {
		r, ok := tt.Find(s)
		if !ok || r.W <= 0 {
			continue
		}
		if r.X+r.W > float32(w-testButtonsW) && r.Y < testButtonsH {
			t.Errorf("%s: %q at %v reaches under the caption buttons", where, s, r)
		}
	}
	for _, o := range textOverlapsAbove(tt, float32(h-playerBarHeight)) {
		t.Errorf("%s: %s", where, o)
	}
}

// TestPagesClearCaptionButtons walks every page the sidebar reaches and
// asks both questions of each.
func TestPagesClearCaptionButtons(t *testing.T) {
	a, tt := clearanceTester(t, testWinW, testWinH)
	for _, pg := range clearancePages() {
		a.show(pg.r)
		frameSettled(tt, false)
		checkClearance(t, tt, testWinW, testWinH, "page "+pg.name)
	}
}

// TestNowPlayingClearCaptionButtons asks the same of every pane of Now
// Playing, which draws its own top row over the pages'.
func TestNowPlayingClearCaptionButtons(t *testing.T) {
	a, tt := clearanceTester(t, testWinW, testWinH)
	a.nowPlaying = true
	for _, pane := range clearancePanes() {
		a.npPane = pane.pane
		frameSettled(tt, true)
		checkClearance(t, tt, testWinW, testWinH, "pane "+pane.name)
	}
}

// TestClearanceAtTheSmallestWindow asks both questions of the window at
// its least size, where there is the least room to keep apart in.
func TestClearanceAtTheSmallestWindow(t *testing.T) {
	a, tt := clearanceTester(t, testSmallW, testSmallH)
	for _, pg := range clearancePages() {
		a.show(pg.r)
		frameSettled(tt, false)
		checkClearance(t, tt, testSmallW, testSmallH, "small page "+pg.name)
	}
	a.nowPlaying = true
	for _, pane := range clearancePanes() {
		a.npPane = pane.pane
		frameSettled(tt, true)
		checkClearance(t, tt, testSmallW, testSmallH, "small pane "+pane.name)
	}
}
