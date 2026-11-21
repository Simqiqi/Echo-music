package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/player"
)

// 十段增益是均衡器的真相：设置里存的就是它，预设只是往它里面填一套数。
func TestEqGainsAreTheTruth(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.EQ = player.NewEQ()

	// 一条老设置：只记了预设的名字，没有十段增益。
	a.settings.EqPreset = "摇滚"
	if got, want := a.settings.eqShape(), "摇滚"; got != want {
		t.Errorf("eqShape = %q, want %q", got, want)
	}
	rock, ok := eqPresetGains("摇滚")
	if !ok {
		t.Fatal("找不到「摇滚」这个预设")
	}
	if a.settings.eqGains() != rock {
		t.Error("只记了名字的老设置没有还原成那个预设的十段")
	}

	// 选一个预设，就是把它的十段灌进来；玩家那边也跟着变。
	a.setEqGains(rock)
	if a.pl.EQ.Gains() != rock {
		t.Error("setEqGains 没有把十段交给播放器")
	}
	if got, want := a.settings.eqShape(), "摇滚"; got != want {
		t.Errorf("eqShape = %q, want %q", got, want)
	}

	// 动一段，形状就是自己的了。
	custom := rock
	custom[3] += 2
	a.setEqGains(custom)
	if got, want := a.settings.eqShape(), eqCustom; got != want {
		t.Errorf("eqShape = %q, want %q", got, want)
	}
	if a.pl.EQ.Gains() != custom {
		t.Error("自定义的十段没有交给播放器")
	}

	// 归零：所有段落回 0，形状是「关闭」。
	a.setEqGains([10]float64{})
	if got, want := a.settings.eqShape(), "关闭"; got != want {
		t.Errorf("eqShape = %q, want %q", got, want)
	}
}

// 超出范围的增益会被收回来，短一截的列表会补齐——设置文件可能被手改过。
func TestEqGainsClampedAndPadded(t *testing.T) {
	s := settings{EqGains: []float64{99, -99, 3}}
	got := s.eqGains()
	if got[0] != 12 || got[1] != -12 || got[2] != 3 {
		t.Errorf("前两段没有收到 ±12 内：%v", got[:3])
	}
	for i := 3; i < 10; i++ {
		if got[i] != 0 {
			t.Errorf("第 %d 段应当是 0，得到 %v", i, got[i])
		}
	}
}

// 频段标签跟播放器的十段一一对应，界面上写的就是滤波器用的。
func TestEqBandLabelsMatchTheFilters(t *testing.T) {
	want := []string{"31", "62", "125", "250", "500", "1k", "2k", "4k", "8k", "16k"}
	for i, w := range want {
		if got := eqBandLabel(i); got != w {
			t.Errorf("第 %d 段标签 = %q, want %q", i, got, w)
		}
	}
	if float64(len(want)) != float64(len(player.EQFreqs)) {
		t.Errorf("标签 %d 个，频段 %d 个", len(want), len(player.EQFreqs))
	}
}

// 拖一段滑杆，设置里的十段与播放器都要跟着动，形状也随之变成自定义。
func TestEqBandDragChangesTheBands(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.EQ = player.NewEQ()
	a.settings.EqGains = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	a.settings.EqPreset = "关闭"

	tt := ui.NewTester(func(c *ui.Context) { a.settingsTestView(c) }, 620, 2000)
	tt.Frame()

	box, ok := tt.Find("均衡器 31")
	if !ok {
		t.Fatal("找不到 31Hz 那段滑杆")
	}
	mid := box.Y + box.H/2
	tt.Press(box.X+2, mid) // 按在最左：-12 dB 那头
	tt.Frame()
	tt.Move(box.X+box.W*0.8, mid) // 一路拖到右边
	tt.Frame()
	tt.Release(box.X+box.W*0.8, mid)
	tt.Frame()

	if got := a.settings.eqGains()[0]; got <= 0 {
		t.Errorf("31Hz 段 = %v，拖到右边之后应当为正", got)
	}
	if a.pl.EQ.Gains() != a.settings.eqGains() {
		t.Error("播放器拿到的十段与设置里的不一致")
	}
	if got := a.settings.eqShape(); got != eqCustom {
		t.Errorf("eqShape = %q，拖动之后应当是 %q", got, eqCustom)
	}
}
