package app

import (
	"bytes"
	"image/png"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// A month's poster counts the days a week's cannot see, so the two never
// show the same minutes, and the days' bars keep to the span they name.
func TestPeriodReportGathersDays(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	w := newWeeklyAt("")
	tr := netease.Track{ID: 1, Name: "晴天", Artists: []netease.Artist{{Name: "周杰伦"}}}
	w.add(now.AddDate(0, 0, -20), tr, 600)
	w.add(now.AddDate(0, 0, -3), tr, 300)

	if week := w.periodReport(now, 7); week.Minutes != 5 || week.Songs != 1 {
		t.Errorf("the week: %+v", week)
	}
	month := w.periodReport(now, 30)
	if month.Minutes != 15 || month.Songs != 1 {
		t.Errorf("the month: %+v", month)
	}
	if len(month.Tops) != 1 || month.Tops[0].Minutes != 15 || month.Tops[0].Track.Name != "晴天" {
		t.Errorf("the month's top song: %+v", month.Tops)
	}
	if len(month.Artists) != 1 || month.Artists[0].Name != "周杰伦" {
		t.Errorf("the month's singer: %+v", month.Artists)
	}
	days := w.periodDays(now, 30)
	if len(days) != 30 || days[9] != 10 || days[26] != 5 {
		t.Errorf("the month's days: %v", days)
	}
}

// Exporting a week's or a month's poster copies a PNG of the card's size,
// and says which span it was.
func TestReportPosterCopies(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pal.accent = ui.RGB(200, 80, 60)
	w := newWeeklyAt("")
	w.add(time.Now(), netease.Track{
		ID: 1, Name: "晴天", Artists: []netease.Artist{{Name: "周杰伦"}},
	}, 600)
	a.weekly = w

	for _, tc := range []struct {
		days  int
		toast string
	}{
		{7, "周报海报已复制，粘贴即可分享"},
		{30, "月报海报已复制，粘贴即可分享"},
	} {
		var got []byte
		a.saveImage = func(b []byte) error { got = b; return nil }
		a.shareReport(tc.days)
		if len(got) == 0 {
			t.Fatalf("%d days: no poster was written", tc.days)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(got))
		if err != nil {
			t.Fatalf("%d days: not an image: %v", tc.days, err)
		}
		if cfg.Width != cardW*2 || cfg.Height != cardH*2 {
			t.Errorf("%d days: poster is %dx%d, want %dx%d", tc.days, cfg.Width, cfg.Height, cardW*2, cardH*2)
		}
		if a.toast != tc.toast {
			t.Errorf("%d days: toast %q, want %q", tc.days, a.toast, tc.toast)
		}
	}
}

// With nothing heard yet, exporting says so rather than drawing an empty
// poster.
func TestReportPosterWithoutListening(t *testing.T) {
	a := newTestApp()
	a.weekly = newWeeklyAt("")
	a.shareReport(30)
	if a.toast != "这段时间还没有听歌" {
		t.Errorf("toast %q", a.toast)
	}
}
