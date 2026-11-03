package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// TestFMShot draws the FM page, for its looks; skip it unless NM_SHOTS says
// where the pictures go.
func TestFMShot(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	a.route = route{page: pageFM}
	os.MkdirAll(dir, 0o755)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.SetDark(true)
	tt.Frame()
	f, err := os.Create(filepath.Join(dir, "fm-dark.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// TestHomeShot draws the home page with the day's picks, the charts and
// what the app heard lately, for the layout.
func TestHomeShot(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	fillHome(t, a)
	now := time.Now()
	// A landscape for the day's first pick, and a blurred copy of it:
	// clear at the left of the card, blur by the right.
	clearImg := image.NewRGBA(image.Rect(0, 0, 900, 600))
	for y := range 600 {
		for x := range 900 {
			r := uint8(40 + 180*x/900)
			g := uint8(90 + 60*y/600)
			b := uint8(220 - 150*x/900)
			// A grid of lines, so the drawing's scale reads.
			if x%60 < 3 || y%60 < 3 {
				r, g, b = 255, 255, 255
			}
			i := clearImg.PixOffset(x, y)
			clearImg.Pix[i], clearImg.Pix[i+1], clearImg.Pix[i+2], clearImg.Pix[i+3] = r, g, b, 255
		}
	}
	a.wx = newWx()
	a.wx.shots = []wxShot{{URL: "sky"}}
	a.sky.url = "sky"
	a.art.art["sky"] = &artwork{bmp: ui.NewBitmap(clearImg), used: a.art.now()}
	a.weekly = newWeeklyAt("")
	for i := range 6 {
		tr := netease.Track{ID: int64(i + 1), Name: fmt.Sprintf("最近听的歌%d", i+1),
			Artists: []netease.Artist{{Name: "歌手"}}, Album: netease.Album{PicURL: fmt.Sprintf("pic%d", i+1)}}
		a.weekly.add(now, tr, 300+float64(i)*40)
	}
	for i := range 10 {
		a.recPlaylists = append(a.recPlaylists, netease.Playlist{
			ID: int64(i + 1), Name: fmt.Sprintf("今日推荐歌单 %d", i+1), PlayCount: 100000 + float64(i)*5, TrackCount: 30})
	}
	for i := range 4 {
		a.charts = append(a.charts, netease.Playlist{
			ID: int64(100 + i), Name: []string{"飙升榜", "新歌榜", "原创榜", "热歌榜"}[i], UpdateFreq: "每天更新"})
	}
	for i := range 8 {
		a.featured = append(a.featured, netease.Playlist{
			ID: int64(200 + i), Name: fmt.Sprintf("推荐歌单 %d", i+1), PlayCount: 90000, TrackCount: 25})
	}
	if a.sky.bmp(a) == nil {
		t.Fatal("no landscape for the day's first pick")
	}
	a.route = route{page: pageHome}
	os.MkdirAll(dir, 0o755)
	tt := ui.NewTester(a.view, 1200, 2300)
	tt.Frame()
	tt.Advance(time.Second)
	for range 3 {
		tt.Frame()
	}
	write := func(name string) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	write("home.png")
}

// TestDailyShot draws the daily card on a wide page, with the day's picks
// listed between the words and the discs of glass, for its layout.
func TestDailyShot(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	coverImg := func(hue int) image.Image {
		img := image.NewRGBA(image.Rect(0, 0, 240, 240))
		for y := range 240 {
			for x := range 240 {
				r := uint8(60 + 150*(x+y)/480)
				g := uint8(40 + hue)
				b := uint8(180 - hue)
				i := img.PixOffset(x, y)
				img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = r, g, b, 255
			}
		}
		return img
	}
	for i := range 30 {
		pic := fmt.Sprintf("daily%d", i+1)
		a.daily = append(a.daily, netease.Track{ID: int64(i + 1), Name: fmt.Sprintf("今日之歌 %d", i+1),
			Artists: []netease.Artist{{Name: "歌手甲"}}, Album: netease.Album{PicURL: pic}})
		if i < 3 {
			a.art.art[netease.Sized(pic, 240)] = &artwork{bmp: ui.NewBitmap(coverImg(i * 40)), used: a.art.now()}
		}
	}
	big := coverImg(30)
	a.art.art[netease.Sized(a.daily[0].Album.PicURL, 600)] = &artwork{bmp: ui.NewBitmap(big), tint: ui.Hex("#b8642e"), used: a.art.now()}
	for i := range 4 {
		a.charts = append(a.charts, netease.Playlist{ID: int64(100 + i), Name: []string{"飙升榜", "新歌榜", "原创榜", "热歌榜"}[i]})
	}
	a.route = route{page: pageHome}
	os.MkdirAll(dir, 0o755)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	tt.Advance(time.Second)
	for range 3 {
		tt.Frame()
	}
	f, err := os.Create(filepath.Join(dir, "daily.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
	// Again, later: the rainbow runs with the clock, so the two differ.
	tt.Advance(4 * time.Second)
	for range 3 {
		tt.Frame()
	}
	f2, err := os.Create(filepath.Join(dir, "daily2.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	if err := png.Encode(f2, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// fillHome gives a test app a home page's worth of things to show.
func fillHome(t *testing.T, a *app) {
	t.Helper()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	now := time.Now()
	a.weekly = newWeeklyAt("")
	for i := range 6 {
		tr := netease.Track{ID: int64(i + 1), Name: fmt.Sprintf("最近听的歌%d", i+1),
			Artists: []netease.Artist{{Name: "歌手"}}, Album: netease.Album{PicURL: fmt.Sprintf("pic%d", i+1)}}
		a.weekly.add(now, tr, 300+float64(i)*40)
	}
	for i := range 10 {
		a.recPlaylists = append(a.recPlaylists, netease.Playlist{
			ID: int64(i + 1), Name: fmt.Sprintf("今日推荐歌单 %d", i+1), PlayCount: 100000 + float64(i)*5, TrackCount: 30})
	}
	for i := range 4 {
		a.charts = append(a.charts, netease.Playlist{
			ID: int64(100 + i), Name: []string{"飙升榜", "新歌榜", "原创榜", "热歌榜"}[i], UpdateFreq: "每天更新"})
	}
	for i := range 8 {
		a.featured = append(a.featured, netease.Playlist{
			ID: int64(200 + i), Name: fmt.Sprintf("推荐歌单 %d", i+1), PlayCount: 90000, TrackCount: 25})
	}
}

// TestCapsuleShot draws the home page once scrolled, so that the capsule
// over it names the section the page has come to.
func TestCapsuleShot(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	fillHome(t, a)
	a.route = route{page: pageHome}
	os.MkdirAll(dir, 0o755)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	write := func(name string) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	write("capsule-rest.png")
	// Scrolled, the capsule names the section the page has come to.
	a.pageScroll.ScrollTo(0, 760, ui.Smooth)
	tt.Advance(900 * time.Millisecond)
	for range 8 {
		tt.Frame()
		tt.Advance(80 * time.Millisecond)
	}
	write("capsule.png")
	// And further, past the picks, the name follows.
	a.pageScroll.ScrollTo(0, 1250, ui.Smooth)
	tt.Advance(900 * time.Millisecond)
	for range 8 {
		tt.Frame()
		tt.Advance(80 * time.Millisecond)
	}
	write("capsule2.png")
}
