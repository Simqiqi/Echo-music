package netease

import "testing"

func TestSized(t *testing.T) {
	cases := []struct {
		name string
		pic  string
		n    int
		want string
	}{
		{"empty", "", 96, ""},
		{"netease plain", "http://p1.music.126.net/a/b.jpg", 96, "https://p1.music.126.net/a/b.jpg?param=96y96"},
		{"netease already has query", "https://p1.music.126.net/a/b.jpg?foo=1", 96, "https://p1.music.126.net/a/b.jpg?foo=1&param=96y96"},
		// 汽水音乐等第三方：保持原样，不能加网易云的 ?param=，否则既多余又会在已有
		// query 时拼出第二个 '?' 导致封面加载失败。
		{"qishui cover", "https://p1.qishui.com/abc~template-image.image", 240, "https://p1.qishui.com/abc~template-image.image"},
		{"third-party cover with size", "https://img.example.com/song/abc_240.jpg", 240, "https://img.example.com/song/abc_240.jpg"},
		{"qishui url with query", "https://p1.qishui.com/cover?token=xyz", 96, "https://p1.qishui.com/cover?token=xyz"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Sized(c.pic, c.n); got != c.want {
				t.Fatalf("Sized(%q, %d) = %q, want %q", c.pic, c.n, got, c.want)
			}
		})
	}
}
