package player

import (
	"math"
	"sync"
)

// The equalizer: ten bands of peaking filter, from the bass to the air,
// that the listener may lift or lower as they like. It rides on the
// decoder, so that it works whatever plays, and steps aside at once when
// every band stands at nought.

// EQFreqs are the bands' middle frequencies, hertz, an octave apart. The
// app labels the sliders with them, so they are the same ten the filters
// use: a band the listener moves is a band they see.
var EQFreqs = [10]float64{31, 62, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

// EQ is the ten bands, their gains, and the filters' state.
type EQ struct {
	mu    sync.Mutex
	gains [10]float64 // decibels, -12 to 12
	sr    int
	ch    int
	f     [10]biquad
	// preamp is what the samples are multiplied by before the filters, so
	// that a lifted band cannot push them past full scale. Bands only
	// ever add — a peaking filter lifts its middle without touching the
	// rest — so without this the louder parts of a loud master would
	// clip, which is heard as a crackle that is not in the recording.
	preamp float64
}

// biquad is one peaking filter, and its state for both channels.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2             [2]float64
	y1, y2             [2]float64
}

// NewEQ returns an equalizer whose every band stands at nought.
func NewEQ() *EQ { return &EQ{} }

// Set sets the bands' gains, in decibels, and starts the filters over.
func (e *EQ) Set(gains [10]float64) {
	for i, g := range gains {
		gains[i] = min(max(g, -12), 12)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.gains = gains
	e.reset()
}

// Gains is what the bands stand at now.
func (e *EQ) Gains() [10]float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.gains
}

// setStream tells the equalizer what rate and how many channels the song
// coming next runs at, and starts its filters over.
func (e *EQ) SetStream(sr, ch int) {
	if ch < 1 || ch > 2 {
		ch = 2
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if sr == e.sr && ch == e.ch {
		return
	}
	e.sr, e.ch = sr, ch
	e.reset()
}

// reset reckons every band's coefficients over again.
func (e *EQ) reset() {
	on := false
	for _, g := range e.gains {
		if g != 0 {
			on = true
		}
	}
	for i := range e.f {
		e.f[i] = biquad{}
		if on && e.sr > 0 {
			e.f[i].peaking(EQFreqs[i], float64(e.sr), e.gains[i], 1.1)
		}
	}
	e.preamp = 1
	if on && e.sr > 0 {
		e.preamp = math.Pow(10, -e.peak()/20)
	}
}

// peak is the most the whole cascade lifts any frequency, in decibels:
// what the samples have to be scaled back by to leave room for it. It is
// the cascade's own magnitude response, swept in fine steps rather than
// guessed from the gains — ten bands an octave apart overlap, and their
// middles are not the only place the lift adds up.
func (e *EQ) peak() float64 {
	if e.sr <= 0 {
		return 0
	}
	most := 0.0
	nyquist := float64(e.sr) / 2
	for f := 10.0; f < nyquist; f *= 1.02 {
		w := 2 * math.Pi * f / float64(e.sr)
		re, im := 1.0, 0.0
		for i := range e.f {
			b := &e.f[i]
			// H(e^{jw}) for b0..b2 / 1, a1, a2.
			cos1, sin1 := math.Cos(w), math.Sin(w)
			cos2, sin2 := math.Cos(2*w), math.Sin(2*w)
			numRe := b.b0 + b.b1*cos1 + b.b2*cos2
			numIm := -(b.b1*sin1 + b.b2*sin2)
			denRe := 1 + b.a1*cos1 + b.a2*cos2
			denIm := -(b.a1*sin1 + b.a2*sin2)
			den := denRe*denRe + denIm*denIm
			if den == 0 {
				continue
			}
			hRe := (numRe*denRe + numIm*denIm) / den
			hIm := (numIm*denRe - numRe*denIm) / den
			re, im = re*hRe-im*hIm, re*hIm+im*hRe
		}
		if db := 20 * math.Log10(math.Hypot(re, im)); db > most {
			most = db
		}
	}
	return most
}

// peaking sets the filter to a peaking band of f0 hertz, of gain g
// decibels, at a song of rate sr.
func (b *biquad) peaking(f0, sr, g, q float64) {
	if g == 0 {
		*b = biquad{b0: 1}
		return
	}
	A := math.Pow(10, g/40)
	w0 := 2 * math.Pi * f0 / sr
	cw, sw := math.Cos(w0), math.Sin(w0)
	alpha := sw / (2 * q)
	a0 := 1 + alpha/A
	b.b0 = (1 + alpha*A) / a0
	b.b1 = (-2 * cw) / a0
	b.b2 = (1 - alpha*A) / a0
	b.a1 = (-2 * cw) / a0
	b.a2 = (1 - alpha/A) / a0
}

// flat is whether every band stands at nought, so that the samples need
// not pass through the filters at all.
func (e *EQ) flat() bool {
	for _, g := range e.gains {
		if g != 0 {
			return false
		}
	}
	return true
}

// process filters buf, samples interleaved, in place.
func (e *EQ) Process(buf []float32) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.flat() || e.sr == 0 || e.ch == 0 {
		return
	}
	ch := e.ch
	if e.preamp != 1 {
		for i := range buf {
			buf[i] = float32(float64(buf[i]) * e.preamp)
		}
	}
	for i := range e.f {
		f := &e.f[i]
		if f.b0 == 1 && f.b1 == 0 && f.b2 == 0 && f.a1 == 0 && f.a2 == 0 {
			continue
		}
		for n := 0; n+ch <= len(buf); n += ch {
			for c := 0; c < ch; c++ {
				x := float64(buf[n+c])
				y := f.b0*x + f.b1*f.x1[c] + f.b2*f.x2[c] - f.a1*f.y1[c] - f.a2*f.y2[c]
				f.x2[c], f.x1[c] = f.x1[c], x
				f.y2[c], f.y1[c] = f.y1[c], y
				buf[n+c] = float32(y)
			}
		}
	}
	// 预放大是照滤波器峰值算的，样本之间的过冲仍可能略微越界；这里只是
	// 别让它以别的样子响出来（负值回绕听起来是咔哒声）。
	for i, v := range buf {
		if v > 1 {
			buf[i] = 1
		} else if v < -1 {
			buf[i] = -1
		}
	}
}

// eqDecoder is a song with the equalizer over it.
type eqDecoder struct {
	stream
	eq *EQ
}

// Read decodes, and filters what it decoded.
func (d eqDecoder) Read(buf []float32) (int, error) {
	n, err := d.stream.Read(buf)
	if n > 0 {
		d.eq.Process(buf[:n])
	}
	return n, err
}

// EQPresets are the equalizer's ready shapes, as the listener knows them.
var EQPresets = []struct {
	Label string
	Gains [10]float64
}{
	{"关闭", [10]float64{}},
	{"流行", [10]float64{-1, 1, 3, 4, 3, 1, -1, -1, -1, -2}},
	{"摇滚", [10]float64{4, 3, 1, 0, -1, -1, 1, 3, 4, 4}},
	{"古典", [10]float64{2, 1, 0, 0, 0, 0, -1, -1, 1, 2}},
	{"低音增强", [10]float64{6, 5, 4, 2, 1, 0, 0, 0, 0, 0}},
	{"人声", [10]float64{-2, -1, 0, 2, 4, 4, 3, 1, 0, -1}},
}
