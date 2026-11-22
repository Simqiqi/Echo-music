package player

import (
	"math"
	"testing"
)

// 流行那套预设：低mid 抬了 +4dB 上下。用它是因为它正好能把峰值推出满刻度。
var popGains = [10]float64{-1, 1, 3, 4, 3, 1, -1, -1, -1, -2}

// 抬了增益之后，满刻度的声音也不该越界——越界就是削波，听起来是录音里
// 没有的破音。
func TestEQDoesNotClipALoudSine(t *testing.T) {
	const sr, ch = 44100, 2
	e := NewEQ()
	e.Set(popGains)
	e.SetStream(sr, ch)

	// 250Hz（被抬得最高的那一段）满刻度的正弦，半秒。
	buf := make([]float32, sr/2*ch)
	for i := 0; i < len(buf); i += ch {
		v := float32(math.Sin(2 * math.Pi * 250 * float64(i/ch) / sr))
		buf[i], buf[i+1] = v, v
	}
	e.Process(buf)

	peak := 0.0
	for _, v := range buf[ch*2000:] { // 跳过起振
		peak = math.Max(peak, math.Abs(float64(v)))
	}
	if peak > 1.0001 {
		t.Errorf("峰值 %v 越界：均衡器把信号推出满刻度了（会削波）", peak)
	}
	if peak < 0.4 {
		t.Errorf("峰值只有 %v：预放大压得过头，声音会明显变小", peak)
	}
}

// 峰值增益是照滤波器自己的响应算的，不是把各段增益加起来。十段相隔一个八度
// 会互相叠加，所以这个数会略高于最大的那一段、但远低于总和。
func TestEQPeakIsTheCascadeResponse(t *testing.T) {
	e := NewEQ()
	e.Set(popGains)
	e.SetStream(44100, 2)
	peak := e.peak()
	t.Logf("「流行」这套的峰值增益 = %.2f dB → 预放大 %.2f dB（整体会小这么多）", peak, -peak)
	if peak < 3.5 || peak > 9 {
		t.Errorf("峰值增益 %v dB，看着不像这套预设该有的（+4dB 那一段及其邻居叠加）", peak)
	}
}

// 全部归零时完全不动样本（照旧跳过滤波器）——否则"关闭"也染色了。
func TestEQFlatLeavesSamplesAlone(t *testing.T) {
	e := NewEQ()
	e.Set([10]float64{})
	e.SetStream(44100, 2)
	buf := []float32{0.5, -0.25, 0.1, -0.9}
	before := append([]float32(nil), buf...)
	e.Process(buf)
	for i := range buf {
		if buf[i] != before[i] {
			t.Fatalf("归零时样本被改了：%v → %v", before, buf)
		}
	}
}

// 抬起来的频段应当比没动过的频段更响——否则预放大就是把整首歌压小了。
func TestEQStillLiftsTheBandItBoosts(t *testing.T) {
	const sr, ch = 44100, 2
	level := func(gains [10]float64, hz float64) float64 {
		e := NewEQ()
		e.Set(gains)
		e.SetStream(sr, ch)
		buf := make([]float32, sr/4*ch)
		for i := 0; i < len(buf); i += ch {
			v := float32(math.Sin(2 * math.Pi * hz * float64(i/ch) / sr))
			buf[i], buf[i+1] = v, v
		}
		e.Process(buf)
		sum := 0.0
		for _, v := range buf[ch*4000:] {
			sum += float64(v) * float64(v)
		}
		return math.Sqrt(sum / float64(len(buf[ch*4000:])))
	}
	lifted := level(popGains, 250) // 被抬的
	flat := level([10]float64{}, 250)
	if lifted <= flat {
		t.Errorf("被抬的 250Hz 没有比不抬时更响：%v vs %v", lifted, flat)
	}
}
