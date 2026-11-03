package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// TestSidebarFXShot draws the sidebar as it looks with the private FM
// playing — the selected icon breathing, the playing FM icon ringed by a
// soft halo, and the now-playing cover slowly breathing — plus a scrolled
// frame so the top and bottom fades of the nav read. Run only when NM_SHOTS
// names a folder for the pictures.
func TestSidebarFXShot(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	dark := os.Getenv("NM_DARK") != ""
	os.MkdirAll(dir, 0o755)

	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}

	// Enough playlists that the nav scrolls, so the edge fades have room.
	for i := range 16 {
		a.mine = append(a.mine, netease.Playlist{
			ID: int64(1000 + i), Name: fmt.Sprintf("我的歌单 %02d", i+1),
			TrackCount: 20 + i, PlayCount: 12345 + float64(i)*7,
		})
	}

	// A track playing on the private FM: selected nav item and playing, so
	// the halo and the cover breathing both show.
	tracks := []netease.Track{
		{ID: 1, Name: "晴天", Artists: []netease.Artist{{Name: "周杰伦"}}, Album: netease.Album{Name: "叶惠美", PicURL: "pic1"}},
		{ID: 2, Name: "七里香", Artists: []netease.Artist{{Name: "周杰伦"}}, Album: netease.Album{Name: "七里香", PicURL: "pic2"}},
		{ID: 3, Name: "如也", Artists: []netease.Artist{{Name: "陈粒"}}, Album: netease.Album{Name: "如也", PicURL: "pic3"}},
	}
	a.pl.Cue(tracks)
	a.pl.AttachRadio(fmRadio(netease.FMDefault), func(func([]netease.Track)) {})
	a.pl.Hold(0) // playing, without audio
	pic := "pic1"
	aw := &artwork{bmp: ui.NewBitmap(stripedCover()), tint: stripeColors[0], used: a.art.now()}
	for _, size := range []int{64, 96, 160, 300, 500} {
		key := netease.Sized(pic, size)
		a.art.art[key] = aw
		a.art.art[key+"#backdrop"] = aw
	}
	a.route = route{page: pageFM}

	w, h := 1100, 760
	tt := ui.NewTester(a.view, w, h)
	tt.SetDark(dark)
	// Let the staggered entrance finish and the breathing settle a touch.
	tt.Advance(1800 * time.Millisecond)
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
	name := "sidebar-light.png"
	if dark {
		name = "sidebar-dark.png"
	}
	write(name)

	// Scrolled: the nav's top and bottom fades should soften its edges.
	a.navScroll.ScrollTo(0, 90, ui.Smooth)
	tt.Advance(900 * time.Millisecond)
	for range 8 {
		tt.Frame()
		tt.Advance(80 * time.Millisecond)
	}
	scrollName := "sidebar-light-scroll.png"
	if dark {
		scrollName = "sidebar-dark-scroll.png"
	}
	write(scrollName)
}
