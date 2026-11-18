package app

import (
	"testing"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// TestQualityLevelsAreUniqueAndLabelled: the picker offers exactly the
// five levels this player can decode, each named and none repeated.
func TestQualityLevelsAreUniqueAndLabelled(t *testing.T) {
	seen := map[string]bool{}
	for _, q := range qualityLevels {
		if q.level == "" || q.label == "" || q.short == "" {
			t.Errorf("an unnamed quality: %+v", q)
		}
		if seen[q.level] {
			t.Errorf("the level %q is offered twice", q.level)
		}
		seen[q.level] = true
		if label, short := qualityLabel(q.level); label != q.label || short != q.short {
			t.Errorf("qualityLabel(%q) = %q, %q, want %q, %q", q.level, label, short, q.label, q.short)
		}
	}
	want := []string{"standard", "higher", "exhigh", "lossless", "hires"}
	if len(qualityLevels) != len(want) {
		t.Errorf("the picker has %d levels, want %d", len(qualityLevels), len(want))
	}
	// Premium levels in codecs the player cannot read must stay out.
	for _, level := range []string{"jymaster", "dolby", "qishui-high", "qishui-lossless", "qishui-hires"} {
		if seen[level] {
			t.Errorf("the unplayable level %q is still in the picker", level)
		}
	}
}

// TestBridgeQualityMapsLevels: every pick plays a 汽水 song at the best
// quality that source offers.
func TestBridgeQualityMapsLevels(t *testing.T) {
	cases := map[string]string{
		"standard": "standard",
		"higher":   "high",
		"exhigh":   "high",
		"lossless": "lossless",
		"hires":    "hires",
	}
	for level, want := range cases {
		if got := bridgeQuality(provQishui, level); got != want {
			t.Errorf("bridgeQuality(qishui, %q) = %q, want %q", level, got, want)
		}
	}
}

// TestNetEaseAcceptsEveryPickerLevel: every pick must be a real level of
// NetEase's API, and the lossless badge must follow what it streams.
func TestNetEaseAcceptsEveryPickerLevel(t *testing.T) {
	known := map[string]bool{}
	for _, l := range netease.Levels {
		known[l] = true
	}
	for _, q := range qualityLevels {
		if !known[q.level] {
			t.Errorf("the level %q is not one NetEase offers", q.level)
		}
	}
	if !netease.Lossless("lossless") || !netease.Lossless("hires") || netease.Lossless("exhigh") {
		t.Error("the lossless badge does not follow the two FLAC levels")
	}
}

// TestLevelNameCoversPicker: the download list must not show a raw id for
// any level the picker can produce.
func TestLevelNameCoversPicker(t *testing.T) {
	for _, q := range qualityLevels {
		if got := levelName(q.level); got == q.level {
			t.Errorf("levelName(%q) is still the raw id", q.level)
		}
	}
}

// TestQualityLevelIsValidatedOnLoad: a level left in settings.json by an
// older build — one no longer offered because it cannot be decoded — must
// be replaced by the default, or playback would ask the API for a name it
// does not know.
func TestQualityLevelIsValidatedOnLoad(t *testing.T) {
	for _, q := range qualityLevels {
		if !isQualityLevel(q.level) {
			t.Errorf("a valid level %q is rejected", q.level)
		}
	}
	for _, stale := range []string{"dolby", "jymaster", "qishui-hires", "", "garbage"} {
		if isQualityLevel(stale) {
			t.Errorf("stale level %q is accepted", stale)
		}
	}
}
