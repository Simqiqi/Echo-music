package app

import (
	"testing"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// Two catalogues spell the same title differently: case, spaces, brackets
// and punctuation must not keep a song from being recognised.
func TestNormalizeSong(t *testing.T) {
	cases := []struct{ in, want string }{
		{"青花瓷", "青花瓷"},
		{"青花瓷 (Live)", "青花瓷"},
		{"青花瓷（现场版）", "青花瓷"},
		{"【重制】青花瓷", "青花瓷"},
		{"Hello, World!", "helloworld"},
		{"Hello World", "helloworld"},
		{"A-Lin - 给我一个理由忘记", "alin给我一个理由忘记"},
	}
	for _, c := range cases {
		if got := normalizeSong(c.in); got != c.want {
			t.Errorf("normalizeSong(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A search's results are only taken when the name really matches; the
// artist decides between versions of it. A song that is not there must not
// be swapped in for one that is.
func TestPickMatch(t *testing.T) {
	found := []netease.Track{
		{ID: 1, Name: "青花瓷 (Live)", Artists: []netease.Artist{{Name: "周杰伦"}}},
		{ID: 2, Name: "青花瓷", Artists: []netease.Artist{{Name: "宋雅琳"}}},
		{ID: 3, Name: "青花瓷", Artists: []netease.Artist{{Name: "周杰伦"}}},
	}
	if got, ok := pickMatch("青花瓷", "周杰伦", found); !ok || got.ID != 3 {
		t.Errorf("picked %+v (ok=%v), want the version by 周杰伦", got, ok)
	}
	if got, ok := pickMatch("青花瓷", "宋雅琳", found); !ok || got.ID != 2 {
		t.Errorf("picked %+v (ok=%v), want the version by 宋雅琳", got, ok)
	}
	// 只对得上歌名、歌手对不上时，仍算找到（同一首歌的不同歌手版本很常见）。
	if got, ok := pickMatch("青花瓷", "王力宏", found); !ok || got.ID != 2 {
		t.Errorf("picked %+v (ok=%v), want a name match", got, ok)
	}
	// 歌名对不上就是没找到：宁可少一首，也不能拿别的歌顶上。
	if _, ok := pickMatch("烟花易冷", "周杰伦", found); ok {
		t.Error("a song that is not in the results was taken as a match")
	}
	if _, ok := pickMatch("", "", found); ok {
		t.Error("an empty title matched something")
	}
}
