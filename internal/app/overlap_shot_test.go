package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// With NM_OVERLAP set to a directory, draws every page and every pane of
// Now Playing as Windows shows them — three caption buttons at the top
// right — so that a person can look for anything overlapping them.
func TestOverlapShots(t *testing.T) {
	dir := os.Getenv("NM_OVERLAP")
	if dir == "" {
		t.Skip()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	tr := netease.Track{ID: 1, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000}
	a.pl.Cue([]netease.Track{tr})
	a.pl.SetLyrics(nil)
	save := func(tt *ui.Tester, name string) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	tt := ui.NewTester(a.view, testWinW, testWinH)
	tt.SetTitleBar(ui.TitleBar{Height: testButtonsH, Right: testButtonsW})
	pages := []struct {
		name string
		r    route
	}{
		{"page-home", route{page: pageHome}},
		{"page-daily", route{page: pageDaily}},
		{"page-charts", route{page: pageCharts}},
		{"page-fm", route{page: pageFM}},
		{"page-weather", route{page: pageWeather}},
		{"page-podcasts", route{page: pagePodcasts}},
		{"page-downloads", route{page: pageDownloads}},
		{"page-feed", route{page: pageFeed}},
		{"page-newalbums", route{page: pageNewAlbums}},
		{"page-weekly", route{page: pageWeekly}},
	}
	for _, pg := range pages {
		a.show(pg.r)
		tt.Frame()
		tt.Advance(20 * time.Millisecond)
		tt.Frame()
		save(tt, pg.name)
	}
	a.nowPlaying = true
	panes := []struct {
		name string
		id   int
	}{
		{"np-lyrics", paneLyrics},
		{"np-queue", paneQueue},
		{"np-stage", paneStage},
		{"np-comments", paneComments},
		{"np-wall", paneWall},
		{"np-wall3d", paneWall3D},
		{"np-particles", paneParticles},
	}
	for _, pane := range panes {
		a.npPane = pane.id
		tt.Frame()
		tt.Advance(700 * time.Millisecond)
		tt.Frame()
		save(tt, pane.name)
	}
}
