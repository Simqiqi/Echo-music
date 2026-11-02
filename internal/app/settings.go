package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// settings are the listener's choices, kept in settings.json in the user
// data directory.
type settings struct {
	// Notch shows the quick player around the notch: the listener turns
	// it on.
	Notch bool `json:"notch"`
	// NotchActivity shows the cover and the music's levels beside the
	// notch while music plays, which hides the menu bar there.
	NotchActivity bool `json:"notchActivity"`
	// NotchVirtual keeps a notch of the app's own showing at the top of a
	// display without one; else it shows only while the pointer asks.
	NotchVirtual bool `json:"notchVirtual"`
	// NotchGreeted says the notch showed itself, once, as the first song
	// played (greet).
	NotchGreeted bool `json:"notchGreeted"`
	// Quality is the level of the streams: standard, higher, exhigh,
	// lossless or hires.
	Quality string `json:"quality"`
	// Appearance is light, dark, or system to follow the system.
	Appearance string `json:"appearance"`
	// Smoke makes the progress bars comets trailing smoke as music plays,
	// for a few percent of a core: the listener turns it on.
	Smoke bool `json:"smoke"`
	// LyricsStage shows lyrics on the stage, Folia's style, in Now
	// Playing, rather than scrolling.
	LyricsStage bool `json:"lyricsStage"`
	// StageStyle is how the stage sets its lyrics (stageStyles): empty for
	// Folia's classic, or the id of another of its visualizers.
	StageStyle string `json:"stageStyle,omitempty"`
	// StageSparks throws sparks off each word on the stage as it is sung,
	// whatever its style.
	StageSparks bool `json:"stageSparks,omitempty"`
	// ParticleLyricsOff keeps the line being sung off the particles page,
	// where the record stands in the middle of the pane: the switch in its
	// corner turns it back on, and it shows by default.
	ParticleLyricsOff bool `json:"particleLyricsOff,omitempty"`
	// ParticleLyricMid stands the line being sung across the middle of the
	// particles page, over the record, rather than under it.
	ParticleLyricMid bool `json:"particleLyricMid,omitempty"`
	// ParticleShape is the shape of the specks of the particles page: 0
	// round, 1 square, 2 a star of four points.
	ParticleShape int `json:"particleShape,omitempty"`
	// EqPreset is the equalizer's shape, as eqPresets names it, "" being
	// that it stands aside.
	EqPreset string `json:"eqPreset,omitempty"`
	// Volume is where the volume was left, from 0 to 1.
	Volume float64 `json:"volume"`
	// Normalize evens out the loudness of tracks, by what NetEase
	// measured of each.
	Normalize bool `json:"normalize"`
	// HotComments shows a song's hot comments in its quiet parts, in
	// place of the lyrics (quietComment): the listener turns it on.
	HotComments bool `json:"hotComments"`
	// WxAutoPlay starts the weather's own songs as the weather radio
	// opens, where nothing plays yet: the listener turns it on.
	WxAutoPlay bool `json:"wxAutoPlay"`
	// Fog fills Now Playing's backdrop with fog of the cover's color,
	// rising and swelling with the bass: an experiment the listener turns
	// on. Backdrop holds the same answer, and what it says wins.
	Fog bool `json:"fog"`
	// Backdrop is what fills Now Playing's backdrop: empty for the cover's
	// gradient, "fog" for fog rising through it, "stars" for a sky of the
	// cover's hue with stars drifting up in it (backdrops).
	Backdrop string `json:"backdrop,omitempty"`
	// Transition fades a song out before it ends and the next one in as
	// it begins, and fades each page in as it takes the place of another.
	// It holds no answer for the listener who never said: the default is
	// on, so nil means yes.
	Transition *bool `json:"transition,omitempty"`
	// Glass floats the sidebar and the player bar as panes of Liquid
	// Glass over the page, what scrolls under them frosted and bent, as
	// macOS 26 draws it. The default is on, so nil means yes.
	Glass *bool `json:"glass,omitempty"`
	// GlassClarity is how clear the panes of glass are, from
	// clarityLeast, a fog, to clarityMost, hardly there. It holds no
	// answer until the listener sets it: each pane keeps its own until
	// then.
	GlassClarity *float64 `json:"glassClarity,omitempty"`
	// Vinyl turns Now Playing's cover into a record on a platter, its
	// label the cover itself, spinning while the music plays and freezing,
	// mid-spin, when it stops: the listener turns it on.
	Vinyl bool `json:"vinyl,omitempty"`
	// Accent is the color the app lights with, by its id in accents: empty
	// for the red it has always been.
	Accent string `json:"accent,omitempty"`
	// AppIcon is where the app's icon shows on macOS (appIcons); elsewhere
	// it is always in the notification area.
	AppIcon string `json:"appIcon"`
	// Captions writes down episodes as they play, for their captions,
	// once the listener has downloaded the model.
	Captions bool `json:"captions"`
	// CacheMB is how many megabytes the songs kept on disk may take; 0
	// keeps none.
	CacheMB int `json:"cacheMB"`
	// SearchHistory keeps submitted keywords, most recent first.
	SearchHistory []string `json:"searchHistory,omitempty"`

	path string // where they were loaded from, and are saved to
}

func defaultSettings() settings {
	return settings{NotchActivity: true, Quality: "exhigh", Appearance: "system", Volume: 0.8, Normalize: true, AppIcon: "dock-and-menubar", CacheMB: 2048, Captions: true}
}

// transitions reports whether the smart transitions are on: they are,
// until the listener says otherwise.
func (s settings) transitions() bool {
	return s.Transition == nil || *s.Transition
}

// glassOn reports whether Liquid Glass is on: it is, until the listener
// says otherwise.
func (s settings) glassOn() bool {
	return s.Glass == nil || *s.Glass
}

// backdropChoice is a backdrop Now Playing may wear: its id in the
// settings, and what the settings call it.
type backdropChoice struct {
	id, label string
}

// backdrops are the backdrops Now Playing may wear, the cover's gradient
// first: the settings show them in this order.
var backdrops = []backdropChoice{
	{"", "渐变"},
	{"fog", "烟雾"},
	{"stars", "星野"},
	{"terrain", "地形"},
	{"ripple", "涟漪"},
	{"vinyl", "唱片"},
	{"water", "水彩"},
}

// backdropStyle is what Now Playing's backdrop is: the cover's gradient,
// the fog, the stars, the hills, the water, the record or the washes. Fog,
// set before the listener could choose, still answers for the fog.
func (s settings) backdropStyle() string {
	style := s.Backdrop
	if style == "" && s.Fog {
		style = "fog"
	}
	// A backdrop no longer offered, from a version that had it, makes for
	// the gradient rather than for nothing.
	for _, b := range backdrops {
		if b.id == style {
			return style
		}
	}
	return ""
}

// setBackdrop makes style the backdrop, and the fog flag agree with it,
// so that neither speaks for the other.
func (s *settings) setBackdrop(style string) {
	s.Backdrop = style
	s.Fog = style == "fog"
}

// Where the clarity slider ends: below, the glass is a fog, and above,
// hardly there at all.
const (
	clarityLeast = 0.15
	clarityMost  = 0.95
)

// clarity is how clear the listener set the glass, or the middle of the
// range, as macOS draws it, until they do.
func (s settings) clarity() float64 {
	if s.GlassClarity == nil {
		return 0.5
	}
	return min(clarityMost, max(clarityLeast, *s.GlassClarity))
}

// glassClarity is how clear a pane of glass is: what the listener set, or
// base, the pane's own, until they do.
func (a *app) glassClarity(base float32) float32 {
	if a.settings.GlassClarity == nil {
		return base
	}
	return float32(a.settings.clarity())
}

// accentChoice is a color the app may light with: its id in the settings,
// what the settings call it, and its hue, which the palette turns into the
// red of a progress bar or the ring of a heart.
type accentChoice struct {
	id, label string
	hue       float32
}

// accents are the colors the app may light with, the first its own red:
// the settings show them in this order.
var accents = []accentChoice{
	{"red", "红", 24},
	{"orange", "橙", 54},
	{"yellow", "黄", 88},
	{"green", "绿", 148},
	{"teal", "青", 196},
	{"blue", "蓝", 252},
	{"purple", "紫", 300},
	{"pink", "粉", 338},
}

// accentHue returns the hue of an accent by its id: the red's until the
// listener picks another.
func accentHue(id string) float32 {
	for _, ac := range accents {
		if ac.id == id {
			return ac.hue
		}
	}
	return accents[0].hue
}

// accentLabel returns what the settings call an accent by its id.
func accentLabel(id string) string {
	for _, ac := range accents {
		if ac.id == id {
			return ac.label
		}
	}
	return accents[0].label
}

func settingsPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// oldNames are what the app was called before it took its bundle's name,
// and kept its data under: the fork's first name, and the one before it.
// The names go newest first, so the data that was written last moves.
var oldNames = []string{"MyGo 云音乐", "网易云音乐"}

// moveOldData moves the user data and the song cache kept under an old name
// to the directories of the app's name, unless those hold something
// already: the login, settings and where playback was left go on. It must
// run before anything reads them.
func moveOldData() {
	for _, name := range []mygo.PathName{mygo.PathUserData, mygo.PathCache} {
		dir, err := mygo.App.Path(name)
		if err != nil {
			continue
		}
		for _, old := range oldNames {
			moveDir(filepath.Join(filepath.Dir(dir), old), dir)
		}
	}
}

// moveDir moves the directory old to dir when dir is empty, as Path made
// it, and old exists.
func moveDir(old, dir string) {
	if old == dir {
		return
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) > 0 {
		return
	}
	if _, err := os.Stat(old); err != nil {
		return
	}
	if os.Remove(dir) == nil {
		if err := os.Rename(old, dir); err != nil {
			os.MkdirAll(dir, 0o700)
		}
	}
}

// loadSettings reads the settings, or returns the defaults.
func loadSettings() settings {
	s := defaultSettings()
	if path, err := settingsPath(); err == nil {
		if b, err := os.ReadFile(path); err == nil {
			json.Unmarshal(b, &s)
		}
		s.path = path
	}
	return s
}

// save keeps the settings where they were loaded from; defaults that were
// never loaded, as in tests, are not saved.
func (s settings) save() error {
	path := s.path
	if path == "" {
		return nil
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// setThemeSource applies an appearance to the app's windows: a seam for
// tests, where no app runs to apply it to.
var setThemeSource = func(src mygo.ThemeSource) { mygo.Theme.SetSource(src) }

// applyAppearance makes the windows light, dark, or follow the system.
func (s settings) applyAppearance() {
	switch s.Appearance {
	case "light":
		setThemeSource(mygo.ThemeLight)
	case "dark":
		setThemeSource(mygo.ThemeDark)
	default:
		setThemeSource(mygo.ThemeSystem)
	}
}

// qualityLevel is the stream level NetEase names a quality by.
var qualityLevels = []struct{ level, label string }{
	{"standard", "标准（128k）"},
	{"higher", "较高（192k）"},
	{"exhigh", "极高（320k）"},
	{"lossless", "无损（FLAC）"},
	{"hires", "Hi-Res"},
}

// cacheLimits are the sizes the song cache may take, in megabytes.
var cacheLimits = []struct {
	mb    int
	label string
}{
	{0, "关闭"},
	{1024, "1 GB"},
	{2048, "2 GB"},
	{5120, "5 GB"},
	{10240, "10 GB"},
}
