package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The weather outside the window, as the sky itself reports it. The app
// asks where the listener is by the network they come from — and hears
// it in Chinese, the city's own name — then asks the open sky over that
// place what it is doing, and turns what it hears into a mood of music
// to match. The strip at the head of the page wears a photograph of the
// same weather, picked from the day's wallpapers.

// wxKind is what the sky is up to, as the music reads it.
type wxKind int

const (
	wxSun wxKind = iota
	wxPartly
	wxCloud
	wxFog
	wxRain
	wxThunder
	wxSnow
	wxWind
)

// weatherNow is what the sky reports: where the listener is, what the
// sky does there, how warm it is, and whether the sun is up.
type weatherNow struct {
	City     string
	Region   string
	Desc     string
	Kind     wxKind
	Temp     float64
	Feels    float64
	High     float64
	Low      float64
	Humidity int
	Wind     string
	WindKmh  float64
	Sunrise  string
	Sunset   string
	Night    bool
	At       time.Time
}

// wxPlace is where the listener is, as the network says: the city's name
// in Chinese, and where it lies on the earth.
type wxPlace struct {
	City   string
	Region string
	Lat    float64
	Lon    float64
}

// weatherIn fetches the sky over where the listener is.
func weatherIn() (*weatherNow, error) {
	p, err := locate()
	if err != nil {
		// Without a place, the sky is asked blind, as once it was.
		return wttrWeather("")
	}
	if w, err2 := skyOver(p); err2 == nil {
		return w, nil
	}
	// The place is known even where the open sky is not reached: ask
	// the same report by where it is, and keep the name in Chinese.
	return wttrWeather(fmt.Sprintf("%.3f,%.3f", p.Lat, p.Lon))
}

// locate asks the network where the listener is. The Chinese address
// book knows the city in Chinese, and hands over the coordinates the
// sky is asked by.
func locate() (wxPlace, error) {
	const u = "http://ip-api.com/json/?lang=zh-CN&fields=status,message,country,regionName,city,lat,lon"
	var raw struct {
		Status  string  `json:"status"`
		Message string  `json:"message"`
		Country string  `json:"country"`
		Region  string  `json:"regionName"`
		City    string  `json:"city"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
	}
	if err := getJSON(u, &raw); err != nil {
		return wxPlace{}, err
	}
	if raw.Status != "success" {
		return wxPlace{}, fmt.Errorf("定位失败：%s", raw.Message)
	}
	if raw.City == "" {
		return wxPlace{}, fmt.Errorf("定位不到城市")
	}
	return wxPlace{
		City:   tidyCity(raw.City),
		Region: tidyRegion(raw.Region),
		Lat:    raw.Lat,
		Lon:    raw.Lon,
	}, nil
}

// tidyCity drops the suffix a city's official name carries, which a
// report of the weather has no use for: 银川市 is 银川.
func tidyCity(s string) string {
	s = strings.TrimSpace(s)
	for _, suffix := range []string{"特别行政区", "自治州", "地区", "市", "盟", "县", "区"} {
		if strings.HasSuffix(s, suffix) && len([]rune(s)) > len([]rune(suffix)) {
			return strings.TrimSuffix(s, suffix)
		}
	}
	return s
}

// tidyRegion keeps a province's own short name: 宁夏 rather than 宁夏
// 回族自治区, which is what the address book gives.
func tidyRegion(s string) string {
	s = strings.TrimSpace(s)
	for _, suffix := range []string{"回族自治区", "维吾尔自治区", "壮族自治区", "自治区", "省", "市"} {
		if strings.HasSuffix(s, suffix) && len([]rune(s)) > len([]rune(suffix)) {
			return strings.TrimSuffix(s, suffix)
		}
	}
	return s
}

// skyOver asks the open sky over a place what it is doing now, and what
// the day will do: the warmest and the coldest it comes to, and when the
// sun rises and sets there.
func skyOver(p wxPlace) (*weatherNow, error) {
	f := "https://api.open-meteo.com/v1/forecast?latitude=%.3f&longitude=%.3f" +
		"&current=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code," +
		"wind_speed_10m,wind_direction_10m,is_day" +
		"&daily=temperature_2m_max,temperature_2m_min,sunrise,sunset" +
		"&timezone=auto&forecast_days=1&wind_speed_unit=kmh"
	var raw struct {
		Current struct {
			Time  string  `json:"time"`
			Temp  float64 `json:"temperature_2m"`
			Feels float64 `json:"apparent_temperature"`
			Hum   int     `json:"relative_humidity_2m"`
			Code  int     `json:"weather_code"`
			Wind  float64 `json:"wind_speed_10m"`
			Dir   float64 `json:"wind_direction_10m"`
			Day   int     `json:"is_day"`
		} `json:"current"`
		Daily struct {
			Max     []float64 `json:"temperature_2m_max"`
			Min     []float64 `json:"temperature_2m_min"`
			Sunrise []string  `json:"sunrise"`
			Sunset  []string  `json:"sunset"`
		} `json:"daily"`
	}
	if err := getJSON(fmt.Sprintf(f, p.Lat, p.Lon), &raw); err != nil {
		return nil, err
	}
	if raw.Current.Time == "" {
		return nil, fmt.Errorf("天气为空")
	}
	w := &weatherNow{
		City:     p.City,
		Region:   p.Region,
		Temp:     raw.Current.Temp,
		Feels:    raw.Current.Feels,
		Humidity: raw.Current.Hum,
		WindKmh:  raw.Current.Wind,
		Night:    raw.Current.Day == 0,
		At:       time.Now(),
		Kind:     wxKindWMO(raw.Current.Code),
	}
	w.Desc = wxDescOf(w)
	w.Wind = windWords(raw.Current.Dir, raw.Current.Wind)
	if len(raw.Daily.Max) > 0 {
		w.High = raw.Daily.Max[0]
	}
	if len(raw.Daily.Min) > 0 {
		w.Low = raw.Daily.Min[0]
	}
	if len(raw.Daily.Sunrise) > 0 {
		w.Sunrise = clockText(raw.Daily.Sunrise[0])
	}
	if len(raw.Daily.Sunset) > 0 {
		w.Sunset = clockText(raw.Daily.Sunset[0])
	}
	return w, nil
}

// windWords names the wind the way a report does: where it comes from,
// and how hard it blows.
func windWords(deg, kmh float64) string {
	dirs := []string{"北风", "东北风", "东风", "东南风", "南风", "西南风", "西风", "西北风"}
	d := int((deg+22.5)/45) % 8
	if d < 0 {
		d += 8
	}
	return fmt.Sprintf("%s %d km/h", dirs[d], int(kmh+0.5))
}

// clockText reads the hour out of a time the sky writes as
// "2026-11-02T07:12": just the clock, 07:12.
func clockText(s string) string {
	if i := strings.IndexByte(s, 'T'); i >= 0 {
		s = s[i+1:]
	}
	if len(s) >= 5 {
		return s[:5]
	}
	return s
}

// getJSON asks url for a report and reads it into v.
func getJSON(url string, v any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Echo云音乐/1.0")
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

// wttrWeather asks wttr.in what the sky does at query — a place it knows
// by name, or by where it lies — and reads the old report the app began
// with. It stands behind the open sky, for the networks that cannot
// reach it.
func wttrWeather(query string) (*weatherNow, error) {
	u := "https://wttr.in/" + query + "?format=j1&lang=zh"
	var raw struct {
		Cur []struct {
			TempC       string `json:"temp_C"`
			FeelsC      string `json:"FeelsLikeC"`
			Humidity    string `json:"humidity"`
			Code        string `json:"weatherCode"`
			WindKmh     string `json:"windspeedKmph"`
			WindDir     string `json:"winddir16Point"`
			Observation string `json:"localObsDateTime"`
			LangZh      []struct {
				Value string `json:"value"`
			} `json:"lang_zh"`
			Desc []struct {
				Value string `json:"value"`
			} `json:"weatherDesc"`
		} `json:"current_condition"`
		Area []struct {
			Name []struct {
				Value string `json:"value"`
			} `json:"areaName"`
			Region []struct {
				Value string `json:"value"`
			} `json:"region"`
		} `json:"nearest_area"`
		Days []struct {
			Astro []struct {
				Sunrise string `json:"sunrise"`
				Sunset  string `json:"sunset"`
			} `json:"astronomy"`
			MaxC []struct {
				Value string `json:"value"`
			} `json:"maxtempC"`
			MinC []struct {
				Value string `json:"value"`
			} `json:"mintempC"`
		} `json:"weather"`
	}
	if err := getJSON(u, &raw); err != nil {
		return nil, err
	}
	if len(raw.Cur) == 0 {
		return nil, fmt.Errorf("wttr.in: 天气为空")
	}
	cur := raw.Cur[0]
	w := &weatherNow{At: time.Now()}
	if len(raw.Area) > 0 {
		if len(raw.Area[0].Name) > 0 {
			w.City = raw.Area[0].Name[0].Value
		}
		if len(raw.Area[0].Region) > 0 {
			w.Region = raw.Area[0].Region[0].Value
		}
	}
	// The sky speaks Chinese where it can, English where it cannot.
	if len(cur.LangZh) > 0 && cur.LangZh[0].Value != "" {
		w.Desc = cur.LangZh[0].Value
	} else if len(cur.Desc) > 0 {
		w.Desc = cur.Desc[0].Value
	}
	w.Temp, _ = strconv.ParseFloat(cur.TempC, 64)
	w.Feels, _ = strconv.ParseFloat(cur.FeelsC, 64)
	w.Humidity, _ = strconv.Atoi(cur.Humidity)
	w.Kind = wxKindOf(cur.Code)
	kmh, _ := strconv.ParseFloat(cur.WindKmh, 64)
	w.WindKmh = kmh
	if kmh >= 30 && (w.Kind == wxSun || w.Kind == wxPartly || w.Kind == wxCloud) {
		w.Kind = wxWind
	}
	w.Wind = fmt.Sprintf("%s %d km/h", strings.ToLower(cur.WindDir), int(kmh+0.5))
	if len(raw.Days) > 0 {
		d := raw.Days[0]
		if len(d.Astro) > 0 {
			a := d.Astro[0]
			w.Night = afterSunset(cur.Observation, a.Sunrise, a.Sunset)
			w.Sunrise, w.Sunset = clockOf12(a.Sunrise), clockOf12(a.Sunset)
		} else {
			h := time.Now().Hour()
			w.Night = h < 6 || h >= 19
		}
		if len(d.MaxC) > 0 {
			w.High, _ = strconv.ParseFloat(d.MaxC[0].Value, 64)
		}
		if len(d.MinC) > 0 {
			w.Low, _ = strconv.ParseFloat(d.MinC[0].Value, 64)
		}
	} else {
		h := time.Now().Hour()
		w.Night = h < 6 || h >= 19
	}
	return w, nil
}

// clockOf12 writes a clock as "7:12 am" the way the page reads it, with
// two digits and the day's own 24 hours.
func clockOf12(s string) string {
	m, ok := clockOf(s)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

// afterSunset asks whether observed falls between the sun's setting and
// its next rising. The times come as the sky writes them: "07:12" or
// "7:12 am" and "6:45 pm".
func afterSunset(observed, sunrise, sunset string) bool {
	sunUp, ok1 := clockOf(sunrise)
	sunDown, ok2 := clockOf(sunset)
	now, ok3 := clockOf(observed)
	if !ok1 || !ok2 || !ok3 {
		h := time.Now().Hour()
		return h < 6 || h >= 19
	}
	return now >= sunDown || now < sunUp
}

// clockOf reads a clock, as "07:12", "7:12 am", "6:45 pm" or the tail
// of "2026-11-02 20:00", into the minutes since midnight.
func clockOf(s string) (int, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	meridiem := 0
	if strings.Contains(s, "am") {
		meridiem = -1
		s = strings.ReplaceAll(s, "am", "")
	} else if strings.Contains(s, "pm") {
		meridiem = 1
		s = strings.ReplaceAll(s, "pm", "")
	}
	s = strings.TrimSpace(s)
	// A date in front of the clock is no business of the clock's.
	if fields := strings.Fields(s); len(fields) > 1 {
		s = fields[len(fields)-1]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, false
	}
	if meridiem == 1 && h < 12 {
		h += 12
	}
	if meridiem == -1 && h == 12 {
		h = 0
	}
	return h*60 + m, true
}

// wxKindOf reads the sky's own code — the WWO number wttr.in speaks —
// into what the music understands.
func wxKindOf(code string) wxKind {
	n, err := strconv.Atoi(code)
	if err != nil {
		return wxCloud
	}
	switch n {
	case 113:
		return wxSun
	case 116:
		return wxPartly
	case 143, 248, 260:
		return wxFog
	case 200, 386, 389:
		return wxThunder
	case 179, 182, 185, 227, 230, 320, 323, 325, 326, 332, 338, 350, 362, 365, 368, 371, 374, 377:
		return wxSnow
	case 119, 122:
		return wxCloud
	default:
		// Everything else the sky does, it does with rain.
		return wxRain
	}
}

// wxKindWMO reads the open sky's code — the WMO number it speaks — into
// what the music understands.
func wxKindWMO(code int) wxKind {
	switch code {
	case 0, 1:
		return wxSun
	case 2:
		return wxPartly
	case 3:
		return wxCloud
	case 45, 48:
		return wxFog
	case 51, 53, 55, 56, 57, 61, 63, 65, 66, 67, 80, 81, 82:
		return wxRain
	case 71, 73, 75, 77, 85, 86:
		return wxSnow
	case 95, 96, 99:
		return wxThunder
	}
	return wxCloud
}

// wxDescOf names the sky, as the page reads it.
func wxDescOf(w *weatherNow) string {
	if w == nil {
		return ""
	}
	if w.Desc != "" {
		return w.Desc
	}
	if w.Night && (w.Kind == wxSun || w.Kind == wxPartly) {
		return "晴夜"
	}
	names := map[wxKind]string{
		wxSun: "晴", wxPartly: "多云", wxCloud: "阴", wxFog: "雾",
		wxRain: "雨", wxThunder: "雷雨", wxSnow: "雪", wxWind: "大风",
	}
	return names[w.Kind]
}

// wxMood is what the music makes of the sky: what it calls itself, what
// it says of itself, what to ask the catalogue for, and the colors its
// strip wears.
type wxMood struct {
	Title    string
	Sub      string
	Keywords []string
	From, To ui.Color
	Emoji    string
}

// wxMoodOf reads the sky into a mood of music. The night turns the
// sun's mood down to a hush, and the cold turns the rain's up to snow.
func wxMoodOf(k wxKind, night bool, temp float64) wxMood {
	if k == wxRain && temp <= 0 {
		k = wxSnow
	}
	switch k {
	case wxSun:
		if night {
			return wxMood{Title: "星夜微凉", Sub: "晴天的夜，月亮很好，听点安静的",
				Keywords: []string{"华语 夜晚 民谣", "华语 星空 深夜 安静", "华语 月色 治愈"},
				From:     ui.Hex("#26305c"), To: ui.Hex("#8a6fa8"), Emoji: "🌙"}
		}
		return wxMood{Title: "阳光满格", Sub: "大晴天，把音量调大一点",
			Keywords: []string{"华语 晴天 好心情", "华语 夏日 阳光 流行", "华语 元气 欢快"},
			From:     ui.Hex("#f2a03c"), To: ui.Hex("#e2603f"), Emoji: "☀️"}
	case wxPartly:
		return wxMood{Title: "云间散步", Sub: "云不多，风刚好，适合出门",
			Keywords: []string{"华语 小清新 独立", "华语 午后 轻快", "华语 晴天 民谣"},
			From:     ui.Hex("#3d76a8"), To: ui.Hex("#8fb6cf"), Emoji: "⛅"}
	case wxCloud:
		return wxMood{Title: "云层低垂", Sub: "阴天，声音也放轻一点",
			Keywords: []string{"华语 阴天 情绪", "华语 慢歌 抒情", "华语 安静 放松"},
			From:     ui.Hex("#4b5563"), To: ui.Hex("#8b93a1"), Emoji: "☁️"}
	case wxFog:
		return wxMood{Title: "雾里看花", Sub: "雾很大，听点看不清的东西",
			Keywords: []string{"华语 氛围 空灵", "华语 梦境 缥缈", "华语 治愈 轻音乐"},
			From:     ui.Hex("#5a6672"), To: ui.Hex("#a9b4bd"), Emoji: "🌫️"}
	case wxThunder:
		return wxMood{Title: "雷雨围炉", Sub: "外面打雷，屋里正好",
			Keywords: []string{"华语 治愈 钢琴", "华语 雨天 室内 温暖", "华语 舒缓 慢歌"},
			From:     ui.Hex("#2b3242"), To: ui.Hex("#5d6b83"), Emoji: "⛈️"}
	case wxSnow:
		return wxMood{Title: "落雪围炉", Sub: "下雪了，听点温暖的",
			Keywords: []string{"华语 冬日 温暖", "华语 雪天 钢琴", "华语 民谣 炉火"},
			From:     ui.Hex("#4a6b8c"), To: ui.Hex("#c3d5e6"), Emoji: "❄️"}
	case wxWind:
		return wxMood{Title: "风起时节", Sub: "风很大，跟着风走",
			Keywords: []string{"华语 旅行 民谣", "华语 自由 出发", "华语 公路 上路"},
			From:     ui.Hex("#2f6b60"), To: ui.Hex("#8fc6ae"), Emoji: "🍃"}
	default:
		return wxMood{Title: "听雨", Sub: "下雨天，音乐和雨声很配",
			Keywords: []string{"华语 雨天 轻音乐", "华语 下雨天 钢琴", "华语 雨声 治愈"},
			From:     ui.Hex("#33506e"), To: ui.Hex("#7f97ad"), Emoji: "🌧️"}
	}
}

// wxShot is a photograph the strip can wear: where it is, and whose it
// is, for the credit along its foot.
type wxShot struct {
	URL    string
	Credit string
}

// wxShots asks Bing for the day's wallpapers: a week of real
// photographs, one of which the strip picks at random each time it is
// opened.
func wxShots() ([]wxShot, error) {
	const u = "https://cn.bing.com/HPImageArchive.aspx?format=js&idx=0&n=8&mkt=zh-CN"
	var raw struct {
		Images []struct {
			URL       string `json:"url"`
			Copyright string `json:"copyright"`
		} `json:"images"`
	}
	if err := getJSON(u, &raw); err != nil {
		return nil, err
	}
	out := make([]wxShot, 0, len(raw.Images))
	for _, im := range raw.Images {
		if im.URL == "" {
			continue
		}
		link := im.URL
		if strings.HasPrefix(link, "/") {
			link = "https://cn.bing.com" + link
		}
		out = append(out, wxShot{URL: link, Credit: creditOf(im.Copyright)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("没有照片")
	}
	return out, nil
}

// creditOf keeps the place a photograph was taken, dropping the
// photographer's mark that follows it.
func creditOf(s string) string {
	for _, mark := range []string{"©", "("} {
		if i := strings.Index(s, mark); i > 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(strings.Trim(s, "，, "))
}

// hasHan reports whether s is written at least partly in Chinese.
func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// wxChinese keeps the playlists sung in Chinese where the catalogue
// holds enough of them: the weather radio is a Chinese one.
func wxChinese(pls []netease.Playlist) []netease.Playlist {
	var zh []netease.Playlist
	for _, p := range pls {
		if hasHan(p.Name) {
			zh = append(zh, p)
		}
	}
	if len(zh) >= 3 {
		return zh
	}
	return pls
}

// wxChineseTracks puts the songs sung in Chinese first, dropping the
// rest where enough of them fill a page.
func wxChineseTracks(ts []netease.Track) []netease.Track {
	var zh, other []netease.Track
	for _, t := range ts {
		if hasHan(t.Name) || hasHan(t.ArtistNames()) {
			zh = append(zh, t)
		} else {
			other = append(other, t)
		}
	}
	if len(zh) >= 8 {
		return zh
	}
	return append(zh, other...)
}
