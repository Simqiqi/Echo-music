package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func TestWeeklyFrame(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.weekly = newWeeklyAt("")
	now := time.Now()
	// Last week's minutes, for the weeks to stand against each other.
	a.weekly.add(now.AddDate(0, 0, -2), netease.Track{ID: 1, Name: "晴天"}, 600)
	a.weekly.add(now, netease.Track{ID: 1, Name: "晴天"}, 1500)
	a.weekly.add(now, netease.Track{ID: 2, Name: "七里香"}, 900)
	a.weekly.add(now, netease.Track{ID: 3, Name: "如也"}, 400)
	a.weekly.add(now, netease.Track{ID: 4, Name: "易碎品"}, 300)
	// Covers for the record and the rows, striped so that they read.
	for i, id := range []int64{1, 2, 3, 4} {
		pic := fmt.Sprintf("pic%d", id)
		aw := &artwork{bmp: ui.NewBitmap(stripedCover()), tint: stripeColors[i%len(stripeColors)]}
		a.art.art[netease.Sized(pic, 300)] = aw
		a.art.art[netease.Sized(pic, 160)] = aw
		a.weekly.data.Tracks[id] = weeklyTrack{Name: a.weekly.data.Tracks[id].Name, Pic: pic}
	}
	artists := map[int64]string{1: "周杰伦", 2: "周杰伦", 3: "陈粒", 4: "汪苏泷"}
	for id, name := range artists {
		t := a.weekly.data.Tracks[id]
		t.Artist = name
		a.weekly.data.Tracks[id] = t
	}
	os.MkdirAll(dir, 0o755)
	a.route = route{page: pageWeekly}
	tt := ui.NewTester(a.view, 1100, 720)
	tt.SetDark(os.Getenv("NM_DARK") != "")
	tt.Frame()
	name := "weekly.png"
	if os.Getenv("NM_DARK") != "" {
		name = "weekly-dark.png"
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
