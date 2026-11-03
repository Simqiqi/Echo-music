package dj

import "math"

// master makes the voice sit close and warm, as a late-night host's does
// on the radio: it takes the rumble out, lifts the low end a little for
// body, gives the consonants presence, evens the loud and the quiet with
// a gentle compressor, and brings it all to a steady level without
// clipping, fading in and out so it never clicks.
func master(rate int, s []float32) {
	if len(s) == 0 || rate <= 0 {
		return
	}
	fs := float64(rate)
	rumble := highpass(fs, 72, 0.707)
	body := lowShelf(fs, 190, 2.8, 0.72)
	presence := peaking(fs, 2900, 1.6, 0.9)
	for i, x := range s {
		v := rumble.run(float64(x))
		v = body.run(v)
		v = presence.run(v)
		s[i] = float32(v)
	}
	compress(s, rate)
	normalize(s, 0.89)
	fade(s, int(fs*0.014))
}

// masterNatural masters what the speech service's neural voice made:
// the voice arrives finished, so it is only brought to a steady level
// and eased at the ends so it never clicks.
func masterNatural(rate int, s []float32) {
	if len(s) == 0 || rate <= 0 {
		return
	}
	normalize(s, 0.89)
	fade(s, rate*14/1000)
}

// normalize brings the loudest sample to peak.
func normalize(s []float32, peak float64) {
	var top float64
	for _, x := range s {
		if a := math.Abs(float64(x)); a > top {
			top = a
		}
	}
	if top <= 0 {
		return
	}
	g := peak / top
	for i := range s {
		s[i] = float32(float64(s[i]) * g)
	}
}

// fade eases the first and last n samples in and out, along a half cosine.
func fade(s []float32, n int) {
	if n < 1 || 2*n > len(s) {
		n = len(s) / 2
	}
	for i := 0; i < n; i++ {
		g := float32(0.5 - 0.5*math.Cos(math.Pi*float64(i)/float64(n)))
		s[i] *= g
		s[len(s)-1-i] *= g
	}
}

// compress evens the loud and the quiet: a soft knee above -18 dB, at
// about 2.6 to 1, with a quick attack and a slow release, so that the
// voice stays up front without pumping.
func compress(s []float32, rate int) {
	const (
		threshold = 0.126 // -18 dBFS
		ratio     = 2.6
	)
	attack := math.Exp(-1 / (0.008 * float64(rate)))
	release := math.Exp(-1 / (0.160 * float64(rate)))
	env := 0.0
	for i, x := range s {
		a := math.Abs(float64(x))
		c := release
		if a > env {
			c = attack
		}
		env = c*env + (1-c)*a
		if env <= threshold {
			continue
		}
		over := math.Pow(env/threshold, 1/ratio-1)
		s[i] = float32(float64(x) * over)
	}
}

// biquad is a second-order filter, in its direct form.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

func (b *biquad) run(x float64) float64 {
	y := b.b0*x + b.b1*b.x1 + b.b2*b.x2 - b.a1*b.y1 - b.a2*b.y2
	b.x2, b.x1 = b.x1, x
	b.y2, b.y1 = b.y1, y
	return y
}

func (b *biquad) set(b0, b1, b2, a0, a1, a2 float64) {
	b.b0, b.b1, b.b2 = b0/a0, b1/a0, b2/a0
	b.a1, b.a2 = a1/a0, a2/a0
}

// highpass is a second-order high-pass at f0, as the cookbook has it.
func highpass(fs, f0, q float64) biquad {
	w := 2 * math.Pi * f0 / fs
	c, s := math.Cos(w), math.Sin(w)
	alpha := s / (2 * q)
	var b biquad
	b.set((1+c)/2, -(1 + c), (1+c)/2, 1+alpha, -2*c, 1-alpha)
	return b
}

// lowShelf lifts or drops what is below f0 by gainDB.
func lowShelf(fs, f0, gainDB, q float64) biquad {
	A := math.Pow(10, gainDB/40)
	w := 2 * math.Pi * f0 / fs
	c, s := math.Cos(w), math.Sin(w)
	alpha := s / (2 * q)
	beta := 2 * math.Sqrt(A) * alpha
	var b biquad
	b.set(
		A*((A+1)-(A-1)*c+beta),
		2*A*((A-1)-(A+1)*c),
		A*((A+1)-(A-1)*c-beta),
		(A+1)+(A-1)*c+beta,
		-2*((A-1)+(A+1)*c),
		(A+1)+(A-1)*c-beta,
	)
	return b
}

// peaking lifts or drops a band around f0 by gainDB.
func peaking(fs, f0, gainDB, q float64) biquad {
	A := math.Pow(10, gainDB/40)
	w := 2 * math.Pi * f0 / fs
	c, s := math.Cos(w), math.Sin(w)
	alpha := s / (2 * q)
	var b biquad
	b.set(
		1+alpha*A,
		-2*c,
		1-alpha*A,
		1+alpha/A,
		-2*c,
		1-alpha/A,
	)
	return b
}

// encodeWAV writes mono samples as 16-bit PCM WAV, the form the cache
// keeps them in.
func encodeWAV(rate int, s []float32) ([]byte, error) {
	if rate <= 0 {
		return nil, ErrNothing
	}
	b := make([]byte, 44+len(s)*2)
	copy(b[0:], "RIFF")
	put32(b[4:], uint32(36+len(s)*2))
	copy(b[8:], "WAVEfmt ")
	put32(b[16:], 16)
	put16(b[20:], 1) // PCM
	put16(b[22:], 1) // mono
	put32(b[24:], uint32(rate))
	put32(b[28:], uint32(rate*2))
	put16(b[32:], 2)
	put16(b[34:], 16)
	copy(b[36:], "data")
	put32(b[40:], uint32(len(s)*2))
	for i, x := range s {
		v := math.Max(-1, math.Min(1, float64(x)))
		put16(b[44+i*2:], uint16(int16(math.Round(v*32767))))
	}
	return b, nil
}

func put16(b []byte, v uint16) {
	b[0], b[1] = byte(v), byte(v>>8)
}

func put32(b []byte, v uint32) {
	b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}
