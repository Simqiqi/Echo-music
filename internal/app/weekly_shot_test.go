package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
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
		aw := &artwork{bmp: ui.NewBitmap(stripedCover()), tint: stripeColors[i%len(stripeColors)], used: a.art.now()}
		for _, size := range []int{160, 300, 500} {
			key := netease.Sized(pic, size)
			a.art.art[key] = aw
			// The poster asks for the blurred backdrop of the cover.
			a.art.art[key+"#backdrop"] = aw
		}
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
	h := 720
	if v := os.Getenv("NM_H"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			h = n
		}
	}
	tt := ui.NewTester(a.view, 1100, h)
	tt.SetDark(os.Getenv("NM_DARK") != "")
	// The ring sweeps in and the clock's cells arrive over about a
	// second of the tester's own time; advance past it, so the frame
	// shows the page settled.
	tt.Advance(1600 * time.Millisecond)
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
