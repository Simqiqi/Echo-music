package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/lyrics"
	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/player"
)

func lyricLines() []lyrics.Line {
	return []lyrics.Line{
		{At: 1000 * time.Millisecond, End: 3000 * time.Millisecond, Text: "第一句",
			Words: []lyrics.Word{{Text: "第", Start: 1000 * time.Millisecond, End: 1500 * time.Millisecond}, {Text: "一", Start: 1500 * time.Millisecond, End: 2000 * time.Millisecond}, {Text: "句", Start: 2000 * time.Millisecond, End: 3000 * time.Millisecond}}},
		{At: 4000 * time.Millisecond, End: 6000 * time.Millisecond, Text: "第二句",
			Words: []lyrics.Word{{Text: "第", Start: 4000 * time.Millisecond, End: 5000 * time.Millisecond}, {Text: "二", Start: 5000 * time.Millisecond, End: 6000 * time.Millisecond}}},
	}
}

// TestLyricOffsetShiftsTimestamps: the correction has to reach the line
// times, the word times and the ends alike, or the highlight and the
// scroll would drift apart.
func TestLyricOffsetShiftsTimestamps(t *testing.T) {
	p := player.New(func(func()) {})
	p.LyricOffset = func(int64) time.Duration { return 300 * time.Millisecond }
	p.SetLyricsWithOffset(7, lyricLines())
	got, _ := p.Lyrics()
	if got[0].At != 1300*time.Millisecond || got[0].End != 3300*time.Millisecond {
		t.Errorf("first line shifted to %v..%v, want 1.3s..3.3s", got[0].At, got[0].End)
	}
	if got[0].Words[1].Start != 1800*time.Millisecond || got[1].Words[0].End != 5300*time.Millisecond {
		t.Errorf("word times did not shift with the lines: %+v", got[0].Words)
	}
	if idx := lyrics.Index(got, 1400*time.Millisecond); idx != 0 {
		t.Errorf("line at 1.4s is %d, want the first line", idx)
	}
	if idx := lyrics.Index(got, 900*time.Millisecond); idx != -1 {
		t.Errorf("line before the first is %d, want -1", idx)
	}
}

// TestSetLyricOffsetReshiftsLoadedLines: turning the dial must take effect
// now, not after the next song, and must not compound on repeated turns.
func TestSetLyricOffsetReshiftsLoadedLines(t *testing.T) {
	p := player.New(func(func()) {})
	p.Cue([]netease.Track{{ID: 7, Name: "song"}})
	p.LyricOffset = func(int64) time.Duration { return 0 }
	p.SetLyricsWithOffset(7, lyricLines())
	p.SetLyricOffset(7, 500*time.Millisecond)
	got, _ := p.Lyrics()
	if got[0].At != 1500*time.Millisecond {
		t.Fatalf("after +0.5s the first line is at %v", got[0].At)
	}
	p.SetLyricOffset(7, -200*time.Millisecond)
	got, _ = p.Lyrics()
	if got[0].At != 800*time.Millisecond {
		t.Errorf("after -0.2s the first line is at %v, want 0.8s", got[0].At)
	}
	p.SetLyricOffset(8, time.Second) // another song must not touch these lines
	got, _ = p.Lyrics()
	if got[0].At != 800*time.Millisecond {
		t.Errorf("another song's offset changed this one: %v", got[0].At)
	}
}

// TestSettingsLyricOffsets: corrections are per song, and a reset to zero
// removes the entry so settings do not accumulate untouched songs.
func TestSettingsLyricOffsets(t *testing.T) {
	s := defaultSettings()
	if s.lyricOffset(42) != 0 {
		t.Fatal("a fresh setting has an offset")
	}
	s.setLyricOffset(42, 300)
	s.setLyricOffset(43, -100)
	if s.lyricOffset(42) != 300 || s.lyricOffset(43) != -100 {
		t.Errorf("offsets not kept per song: %v", s.LyricOffsets)
	}
	if s.lyricOffsetDuration(42) != 300*time.Millisecond {
		t.Errorf("duration = %v", s.lyricOffsetDuration(42))
	}
	s.setLyricOffset(42, 0)
	if s.lyricOffset(42) != 0 || s.LyricOffsets["42"] != 0 {
		t.Errorf("resetting did not forget the song: %v", s.LyricOffsets)
	}
}

// TestLyricOffsetButtonShowsTheCorrection: the dial names the correction
// it holds, so a listener can see what they set without opening it.
func TestLyricOffsetButtonShowsTheCorrection(t *testing.T) {
	a := newTestApp()
	a.settings.setLyricOffset(42, 300)
	view := func(c *ui.Context) {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Children(func() {
			a.lyricOffsetButton(c, 42)
		})
	}
	tt := ui.NewTester(view, 420, 100)
	tt.Frame()
	if !tt.HasText("歌词 +0.3s") {
		t.Error("the dial does not show the +0.3s correction")
	}
}
