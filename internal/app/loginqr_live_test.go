package app

import (
	"os"
	"testing"

	"github.com/simqiqi/simqiqi-music/internal/kugou"
	"github.com/simqiqi/simqiqi-music/internal/qishui"
	"github.com/simqiqi/simqiqi-music/internal/qq"
)

// 三家桥接音源的登录二维码要走完整条路：向桥接要图 → 解码成位图 → 交给弹窗。
// 这条链子断过一次（错误被一句"请确认已安装 Node.js"盖住，谁也看不出所以然），
// 所以整条测。LIVE=1 才跑：它会起桥接、连三家的登录页。
func TestLoginQRsDecode(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("LIVE is not set: skipping the bridge and the login pages")
	}
	cases := []struct {
		name  string
		start func() (string, string, error)
	}{
		{"汽水音乐", qishui.StartQR},
		{"酷狗音乐", kugou.StartQR},
		{"QQ音乐", func() (string, string, error) { return qq.StartQR(qq.LoginQQ) }},
	}
	for _, c := range cases {
		key, image, err := c.start()
		if err != nil {
			t.Errorf("%s：向桥接要二维码失败：%v", c.name, err)
			continue
		}
		if key == "" || image == "" {
			t.Errorf("%s：桥接给的 key 或 image 是空的", c.name)
			continue
		}
		bmp, err := decodeLoginQR(image)
		if err != nil {
			t.Errorf("%s：解码二维码失败：%v", c.name, err)
			continue
		}
		if w, h := bmp.Size(); w == 0 || h == 0 {
			t.Errorf("%s：解码出来的位图尺寸是 %dx%d", c.name, w, h)
		}
	}
}
