package app

import (
	"fmt"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The sky's own codes read into what the music understands: clear is
// the sun, drops of any size are rain, white flecks are snow, a rumble
// is thunder, and a gale is wind no matter how clear.
func TestWeatherKind(t *testing.T) {
	cases := map[string]wxKind{
		"113": wxSun, "116": wxPartly, "119": wxCloud, "122": wxCloud,
		"143": wxFog, "248": wxFog, "260": wxFog,
		"176": wxRain, "293": wxRain, "308": wxRain, "359": wxRain,
		"200": wxThunder, "386": wxThunder, "389": wxThunder,
		"179": wxSnow, "323": wxSnow, "338": wxSnow,
	}
	for code, want := range cases {
		if got := wxKindOf(code); got != want {
			t.Errorf("code %s is %v, want %v", code, got, want)
		}
	}
}

// The open sky's own codes read the same way.
func TestWeatherKindWMO(t *testing.T) {
	cases := map[int]wxKind{
		0: wxSun, 1: wxSun, 2: wxPartly, 3: wxCloud,
		45: wxFog, 48: wxFog,
		51: wxRain, 61: wxRain, 65: wxRain, 80: wxRain, 82: wxRain,
		71: wxSnow, 73: wxSnow, 75: wxSnow, 86: wxSnow,
		95: wxThunder, 96: wxThunder, 99: wxThunder,
	}
	for code, want := range cases {
		if got := wxKindWMO(code); got != want {
			t.Errorf("WMO %d is %v, want %v", code, got, want)
		}
	}
}

// A city's official name loses its suffix, and a province its long one.
func TestWeatherNames(t *testing.T) {
	cases := map[string]string{"银川市": "银川", "北京市": "北京", "阿拉善盟": "阿拉善", "上海市": "上海"}
	for in, want := range cases {
		if got := tidyCity(in); got != want {
			t.Errorf("%s reads %q, want %q", in, got, want)
		}
	}
	if got := tidyRegion("宁夏回族自治区"); got != "宁夏" {
		t.Errorf("the region reads %q, want 宁夏", got)
	}
	if got := tidyRegion("浙江省"); got != "浙江" {
		t.Errorf("the region reads %q, want 浙江", got)
	}
}

// The weather radio plays Chinese songs: the playlists and the songs
// written in Chinese come first where the catalogue gives enough.
func TestWeatherChinese(t *testing.T) {
	if !hasHan("晴天") || hasHan("Sunny Day") {
		t.Error("the language of a name is read wrong")
	}
	pls := []netease.Playlist{{ID: 1, Name: "Rainy Day"}, {ID: 2, Name: "雨天"}}
	if got := wxChinese(pls); len(got) != 2 {
		t.Errorf("two playlists, one Chinese, kept %d: too few to choose from", len(got))
	}
	songs := []netease.Track{{ID: 1, Name: "Rain"}, {ID: 2, Name: "雨"}, {ID: 3, Name: "晴天"}}
	tracks := wxChineseTracks(songs)
	if len(tracks) != 3 || tracks[0].Name != "雨" {
		t.Errorf("the Chinese songs came first as %v", tracks)
	}
}

// The sun's own rising and setting says night from day, whatever the
// clock writes: "07:12" or "7:12 am", "6:45 pm".
func TestWeatherNight(t *testing.T) {
	cases := []struct {
		observed, sunrise, sunset string
		want                      bool
	}{
		{"2026-11-02 12:00", "6:57 am", "5:31 pm", false},
		{"2026-11-02 20:00", "6:57 am", "5:31 pm", true},
		{"2026-11-02 03:00", "6:57 am", "5:31 pm", true},
		{"2026-11-02 07:00", "07:00", "17:31", false},
	}
	for _, c := range cases {
		if got := afterSunset(c.observed, c.sunrise, c.sunset); got != c.want {
			t.Errorf("%s between %s and %s is night=%v, want %v",
				c.observed, c.sunrise, c.sunset, got, c.want)
		}
	}
}

// The mood of the music follows the sky: rain reads as rain, the
// night's sun reads as a hush, and rain at freezing reads as snow.
func TestWeatherMood(t *testing.T) {
	if m := wxMoodOf(wxRain, false, 15); m.Title != "听雨" {
		t.Errorf("rain by day is %q, want 听雨", m.Title)
	}
	if m := wxMoodOf(wxSun, true, 10); m.Title != "星夜微凉" {
		t.Errorf("clear by night is %q, want 星夜微凉", m.Title)
	}
	if m := wxMoodOf(wxRain, false, -3); m.Title != "落雪围炉" {
		t.Errorf("rain below freezing is %q, want 落雪围炉", m.Title)
	}
	if m := wxMoodOf(wxSun, false, 25); len(m.Keywords) == 0 {
		t.Error("a mood with no keywords asks the catalogue for nothing")
	}
}

// Every opening of the page wears another photograph, never the one
// that showed before.
func TestWeatherShotRolls(t *testing.T) {
	s := &wxState{rng: rand.New(rand.NewSource(3)), washed: map[string]bool{}}
	s.shots = []wxShot{{URL: "a"}, {URL: "b"}, {URL: "c"}}
	s.pickShot()
	for i := 0; i < 12; i++ {
		before := s.shot.URL
		s.otherShot()
		if s.shot.URL == before {
			t.Fatalf("the photograph stayed %q", before)
		}
		if s.shot.URL == "" {
			t.Fatal("a photograph was lost")
		}
	}
}

// What the page heard is kept on the disk, so that it is written the
// moment it opens: the sky, the mood, the playlist, its songs, and the
// day's photographs.
func TestWeatherCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weather.json")
	s := &wxState{rng: rand.New(rand.NewSource(1)), washed: map[string]bool{}, path: path}
	s.data = &weatherNow{City: "银川", Region: "宁夏", Desc: "小雨", Kind: wxRain, Temp: 9, At: time.Now()}
	s.at = time.Now()
	s.mood = wxMoodOf(wxRain, false, 9)
	s.playlists = []netease.Playlist{{ID: 7, Name: "雨天 · 窗边"}}
	s.picked = &s.playlists[0]
	s.tracks = []netease.Track{{ID: 1, Name: "雨"}, {ID: 2, Name: "小雨"}}
	s.shots = []wxShot{{URL: "https://example/a.jpg", Credit: "西湖"}}
	s.shot = s.shots[0]
	s.save()

	// Saved on a goroutine: wait for it to land.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the weather was never saved")
		}
		time.Sleep(10 * time.Millisecond)
	}

	got := &wxState{rng: rand.New(rand.NewSource(2)), washed: map[string]bool{}, path: path}
	got.load()
	if got.data == nil || got.data.City != "银川" || got.data.Kind != wxRain {
		t.Fatalf("the sky came back as %+v", got.data)
	}
	if got.mood.Title != "听雨" {
		t.Errorf("the mood came back as %q", got.mood.Title)
	}
	if got.picked == nil || got.picked.ID != 7 || len(got.tracks) != 2 {
		t.Errorf("the playlist came back as %+v with %d songs", got.picked, len(got.tracks))
	}
	if len(got.shots) != 1 || got.shown != "https://example/a.jpg" {
		t.Errorf("the photographs came back as %+v, showing %q", got.shots, got.shown)
	}
}

// The weather page frame: the strip's sky, the mood board, and the
// songs, with stand-in weather and a stand-in playlist.
func TestWeatherFrame(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.wx = newWx()
	a.wx.data = &weatherNow{
		City: "Hangzhou", Desc: "小雨", Kind: wxRain, Temp: 14, Feels: 12,
		Humidity: 86, Wind: "ne 9 km/h", Night: false, At: time.Now(),
	}
	a.wx.mood = wxMoodOf(wxRain, false, 14)
	a.wx.seekKeyword = "雨天 轻音乐"
	a.wx.shot = wxShot{URL: "shot", Credit: "西湖，杭州，浙江，中国"}
	if path := os.Getenv("NM_PHOTO"); path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			if bmp, img, err := decodeArt(raw); err == nil {
				a.art.art["shot"] = &artwork{bmp: bmp, lum: artLum(img)}
			}
		}
	}
	a.wx.playlists = []netease.Playlist{
		{ID: 1, Name: "雨天 · 窗边的一杯热茶", TrackCount: 46, PlayCount: 1200000, CoverURL: "p1"},
		{ID: 2, Name: "听雨 睡前钢琴", TrackCount: 30, PlayCount: 800000, CoverURL: "p2"},
	}
	a.wx.picked = &a.wx.playlists[0]
	a.wx.tracks = []netease.Track{
		{ID: 1, Name: "Rain After", Album: netease.Album{PicURL: "p1"}},
		{ID: 2, Name: "Sixty Seconds of Silence", Album: netease.Album{PicURL: "p2"}},
		{ID: 3, Name: "Umbrella Sketch", Album: netease.Album{PicURL: "p3"}},
	}
	covers := map[string]int{"p1": 0, "p2": 1, "p3": 2}
	for pic, i := range covers {
		aw := &artwork{bmp: ui.NewBitmap(stripedCover()), tint: stripeColors[i%len(stripeColors)]}
		a.art.art[netease.Sized(pic, 160)] = aw
		a.art.art[netease.Sized(pic, 300)] = aw
	}
	os.MkdirAll(dir, 0o755)
	a.route = route{page: pageWeather}
	name := "weather.png"
	if os.Getenv("NM_DARK") != "" {
		a.wx.data.Kind = wxSnow
		a.wx.mood = wxMoodOf(wxSnow, true, -2)
		a.wx.data.Desc, a.wx.data.Night, a.wx.data.Temp = "小雪", true, -2
		name = "weather-dark.png"
	}
	tt := ui.NewTester(a.view, 1100, 760)
	tt.SetDark(os.Getenv("NM_DARK") != "")
	tt.Frame()
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
	_ = fmt.Sprint()
}
