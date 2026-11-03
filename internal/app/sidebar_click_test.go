package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// TestSidebarClickPassesThrough checks that the decorative top/bottom fade
// overlays of the nav do not eat clicks meant for the nav items beneath
// them — "为你推荐" sits under the top fade and used to be unclickable.
func TestSidebarClickPassesThrough(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	a.pl.Cue([]netease.Track{
		{ID: 1, Name: "晴天", Artists: []netease.Artist{{Name: "周杰伦"}}, Album: netease.Album{Name: "叶惠美", PicURL: "pic1"}},
	})
	a.pl.Hold(0) // playing, without audio
	// Start on the private FM so "为你推荐" is not already selected; a click
	// on it must change the route to pageHome.
	a.route = route{page: pageFM}

	tt := ui.NewTester(a.view, 1100, 760)
	tt.Frame()

	if err := tt.Click("为你推荐"); err != nil {
		t.Fatalf("click on 为你推荐 failed: %v", err)
	}
	tt.Advance(150 * time.Millisecond)
	tt.Frame()

	if a.route.page != pageHome {
		t.Errorf("click on 为你推荐 did not navigate: route.page = %v, want pageHome", a.route.page)
	}
}
