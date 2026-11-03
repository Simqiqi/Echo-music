package dj

// The speech service asks for a proof token, Sec-MS-GEC, that ties the
// request to the clock: Windows file time, rounded down to a five-minute
// window, put through a custom digest pair. This is a faithful port of
// the token the Edge browser makes, and it is pinned by tests against
// reference values.

import (
	"encoding/binary"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	drmToken     = "6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	drmWinEpoch  = 11644473600
	drmTicksPerS = 10_000_000
	drmAdjust    = uint32(0xC9DC06A7) // swapped into the first block's constants
	drmSigma1Adj = uint32(0x000B92C0) // nudges two schedule words in drmInit
	drmSigma0Adj = uint32(0x059C0B80)
)

// secMSGEC is the token for now.
func secMSGEC() string { return secMSGECAt(float64(time.Now().UTC().Unix())) }

// secMSGECAt is the token for a moment, in seconds. The arithmetic is
// floating point, as the reference's is, because the tick count passes
// through a double on its way to the string that is hashed.
func secMSGECAt(ts float64) string {
	ticks := ts + drmWinEpoch
	ticks -= math.Mod(ticks, 300)
	ticks *= drmTicksPerS
	data := strconv.FormatFloat(ticks, 'f', 0, 64) + drmToken
	cs, sum := drmTokenHash([]byte(data))
	return cs + "-" + sum
}

// The first digest is SHA-256 with a few constants moved: the rounds
// start from a custom working state derived from the input's first seven
// bytes, the schedule's words 16 to 29 are fixed, and the fourteenth
// round constant is nudged in the first block.

var (
	// The standard round constants.
	drmK = [64]uint32{
		0x428A2F98, 0x71374491, 0xB5C0FBCF, 0xE9B5DBA5,
		0x3956C25B, 0x59F111F1, 0x923F82A4, 0xAB1C5ED5,
		0xD807AA98, 0x12835B01, 0x243185BE, 0x550C7DC3,
		0x72BE5D74, 0x80DEB1FE, 0x9BDC06A7, 0xC19BF174,
		0xE49B69C1, 0xEFBE4786, 0x0FC19DC6, 0x240CA1CC,
		0x2DE92C6F, 0x4A7484AA, 0x5CB0A9DC, 0x76F988DA,
		0x983E5152, 0xA831C66D, 0xB00327C8, 0xBF597FC7,
		0xC6E00BF3, 0xD5A79147, 0x06CA6351, 0x14292967,
		0x27B70A85, 0x2E1B2138, 0x4D2C6DFC, 0x53380D13,
		0x650A7354, 0x766A0ABB, 0x81C2C92E, 0x92722C85,
		0xA2BFE8A1, 0xA81A664B, 0xC24B8B70, 0xC76C51A3,
		0xD192E819, 0xD6990624, 0xF40E3585, 0x106AA070,
		0x19A4C116, 0x1E376C08, 0x2748774C, 0x34B0BCB5,
		0x391C0CB3, 0x4ED8AA4A, 0x5B9CCA4F, 0x682E6FF3,
		0x748F82EE, 0x78A5636F, 0x84C87814, 0x8CC70208,
		0x90BEFFFA, 0xA4506CEB, 0xBEF9A3F7, 0xC67178F2,
	}

	// The message schedule's words 16 to 29, fixed.
	drmSchedule = [14]uint32{
		0xEB24BF9E, 0x1558691F, 0xA0C09EE2, 0x0CFA32BA,
		0xA622EE8C, 0xD1DD9491, 0xC0DD25A0, 0xF9E70AB6,
		0xF0C31573, 0x7058CE4E, 0x7188E606, 0x00E5AB33,
		0xD258B060, 0x72766564,
	}

	// The rounds' starting working state.
	drmState = [8]uint32{
		0xFD9FBEA2, 0x91243CE7, 0xBE499094, 0xCD2E1E0A,
		0x6C006F49, 0x04BD35AB, 0xFEB0A7E4, 0x03EFFBD2,
	}

	// The standard initial state.
	drmInitial = [8]uint32{
		0x6A09E667, 0xBB67AE85, 0x3C6EF372, 0xA54FF53A,
		0x510E527F, 0x9B05688C, 0x1F83D9AB, 0x5BE0CD19,
	}
)

func rotr(x uint32, n uint) uint32 { return x>>n | x<<(32-n) }

func sig0(x uint32) uint32 { return rotr(x, 7) ^ rotr(x, 18) ^ (x >> 3) }
func sig1(x uint32) uint32 { return rotr(x, 17) ^ rotr(x, 19) ^ (x >> 10) }
func bs0(x uint32) uint32  { return rotr(x, 2) ^ rotr(x, 13) ^ rotr(x, 22) }
func bs1(x uint32) uint32  { return rotr(x, 6) ^ rotr(x, 11) ^ rotr(x, 25) }

// drmRounds runs SHA-256's rounds over one block, from first to 64. The
// derived state's block starts at round 14: the reference skips its
// first rounds, so its schedule's fixed words stand in for them. The
// reference unrolls them with rotations folded into a multiply; the fold
// is the same rotation in the low 32 bits, so the plain form agrees,
// which the pinned tests check.
func drmRounds(state [8]uint32, words *[64]uint32, k *[64]uint32, first int) [8]uint32 {
	a, b, c, d, e, f, g, h := state[0], state[1], state[2], state[3],
		state[4], state[5], state[6], state[7]
	for i := first; i < 64; i++ {
		t1 := h + bs1(e) + (g ^ (e & (f ^ g))) + k[i] + words[i]
		t2 := bs0(a) + ((a & b) | (c & (a | b)))
		h, g, f, e, d, c, b, a = g, f, e, d+t1, c, b, a, t1+t2
	}
	return [8]uint32{a, b, c, d, e, f, g, h}
}

// drmInit derives the digest's starting state from the input's first
// seven bytes: a synthetic block whose schedule is fixed for words 16 to
// 29 and grows from the seed, rounded with the adjusted constants.
func drmInit(seed []byte) [8]uint32 {
	var words [64]uint32
	// The seed goes in as the integers `int.from_bytes` makes: three
	// bytes as a 24-bit number, then four as a 32-bit one.
	words[14] = uint32(seed[0])<<16 | uint32(seed[1])<<8 | uint32(seed[2])
	words[15] = uint32(seed[3])<<24 | uint32(seed[4])<<16 | uint32(seed[5])<<8 | uint32(seed[6])
	for index := 16; index <= 29; index++ {
		value := drmSchedule[index-16]
		// index-2 >= 14 always holds in this range.
		if index == 16 {
			value += sig1(words[index-2]) ^ drmSigma1Adj
		} else {
			value += sig1(words[index-2])
		}
		if index-7 >= 14 {
			value += words[index-7]
		}
		if index-15 >= 14 {
			if index == 29 {
				value += sig0(words[index-15]) ^ drmSigma0Adj
			} else {
				value += sig0(words[index-15])
			}
		}
		words[index] = value
	}
	words[30] = sig1(words[28]) + words[23] + sig0(words[15]) + words[14] +
		drmAdjust - drmK[14]
	for index := 31; index < 64; index++ {
		words[index] = sig1(words[index-2]) + words[index-7] +
			sig0(words[index-15]) + words[index-16]
	}
	var k [64]uint32
	copy(k[:], drmK[:])
	k[14] = drmAdjust
	working := drmRounds(drmState, &words, &k, 14)
	var out [8]uint32
	for i := range out {
		out[i] = drmInitial[i] + working[i]
	}
	return out
}

// drmDigest hashes data with the moved constants: the seed's seven bytes
// set the state, and the rest is padded as SHA-256 pads a message whose
// length the reference counts its own way.
func drmDigest(data []byte) [8]uint32 {
	length := 57 + len(data)
	zeros := ((55-length)%64 + 64) % 64 // the reference's floor mod
	padded := make([]byte, 0, len(data)-7+1+zeros+8)
	padded = append(padded, data[7:]...)
	padded = append(padded, 0x80)
	for i := 0; i < zeros; i++ {
		padded = append(padded, 0)
	}
	var lenBytes [8]byte
	binary.BigEndian.PutUint64(lenBytes[:], uint64(length)*8)
	padded = append(padded, lenBytes[:]...)

	state := drmInit(data[:7])
	for off := 0; off < len(padded); off += 64 {
		var words [64]uint32
		for i := 0; i < 16; i++ {
			words[i] = binary.BigEndian.Uint32(padded[off+i*4:])
		}
		for i := 16; i < 64; i++ {
			words[i] = sig1(words[i-2]) + words[i-7] + sig0(words[i-15]) + words[i-16]
		}
		working := drmRounds(state, &words, &drmK, 0)
		for i := range state {
			state[i] += working[i]
		}
	}
	return state
}

// The second digest is a checksum over the data, keyed by the first, and
// it is transcribed term for term from the reference.

var drmCS64Coefficients = [14][2]uint32{
	{0xE82E91B1, 0xB32700D0},
	{0x44DE24C6, 0x88DFF51D},
	{0x3BEB65BC, 0x29514038},
	{0xC89C5933, 0xB328CBDF},
	{0x8D1EC211, 0xD405B4D5},
	{0x314EF5D5, 0x26E55F3C},
	{0xE92491B1, 0x607801C6},
	{0xF0A5B9D5, 0x82B2C1FB},
	{0x06064CF3, 0xFA51A3DA},
	{0xDA79FA60, 0x207CC464},
	{0xBED20560, 0x3C3C74F3},
	{0x89F9C9F8, 0xD2D14B61},
	{0x35FA38C6, 0xBB04A2C1},
	{0xCB49E3C6, 0xC5626167},
}

const (
	drmCS64Factor  = uint32(0x82893933)
	drmCS64Adjust0 = uint32(0xF2F90B1A)
	drmCS64Adjust1 = uint32(0x0B1A0000)
	drmCS64Mask    = uint64(0xFFFFFFFF)
)

func rol16(v uint32) uint32 { return v<<16 | v>>16 }

// drmCS64 returns the checksum's two words.
func drmCS64(data, digest []byte) (uint32, uint32) {
	keyA := binary.LittleEndian.Uint32(digest[0:4]) | 1
	keyB := binary.LittleEndian.Uint32(digest[4:8]) | 1
	factorA := uint64(keyA * drmCS64Factor)
	factorB := uint64(keyB * drmCS64Factor)

	// The coefficients, and then the data's words, each with an offset.
	var terms, offsets []uint64
	for i, co := range drmCS64Coefficients {
		f := factorA
		if i%2 == 1 {
			f = factorB
		}
		terms = append(terms, uint64(co[0])*f&drmCS64Mask)
		offsets = append(offsets, uint64(co[1]))
	}
	// The data goes in as little-endian words of a leading zero byte,
	// padded to an even count of words.
	buf := append([]byte{0}, data...)
	for len(buf)%4 != 0 {
		buf = append(buf, 0)
	}
	n := (len(buf) / 4 + 1) / 2 * 2
	for i := 0; i < n; i++ {
		var w uint32
		if i*4+4 <= len(buf) {
			w = binary.LittleEndian.Uint32(buf[i*4:])
		}
		key := uint64(keyA)
		mul := uint64(0xEF0569FB)
		if i%2 == 1 {
			key, mul = uint64(keyB), 0xC31713DB
		}
		term := uint64(w) * key
		offset := uint64(rol16(w)) * mul
		if i == 0 {
			term += uint64(drmCS64Adjust0) * factorA
			offset += uint64(drmCS64Adjust1)
		}
		terms = append(terms, term&drmCS64Mask)
		offsets = append(offsets, offset&drmCS64Mask)
	}

	var swapAcc, swapLast, state, acc uint64
	for i := 0; i+1 < len(terms); i += 2 {
		x, y := terms[i], terms[i+1]
		t := rol16(uint32((offsets[i] + x) & drmCS64Mask))
		q := (uint64(t) * 0x76F50F34) & drmCS64Mask
		swapAcc += (uint64(rol16(uint32(q)))*0xC3EFEA97 + q) & drmCS64Mask
		t = rol16(uint32((offsets[i+1] + y) & drmCS64Mask))
		q = (uint64(t) * 0x1881CE3C) & drmCS64Mask
		swapLast = (uint64(rol16(uint32(q)))*0x35BD1EC9 + q) & drmCS64Mask
		swapAcc = (swapAcc + swapLast) & drmCS64Mask

		state = (state*uint64(keyA) + x) & drmCS64Mask
		state = (uint64(rol16(uint32(state))) * 0xCF98B111) & drmCS64Mask
		state = (uint64(rol16(uint32(state))) * 0x87085B9F) & drmCS64Mask
		state = (uint64(rol16(uint32(state))) * 0x12CEB96D) & drmCS64Mask
		state = (uint64(rol16(uint32(state))) * 0x257E1D83) & drmCS64Mask
		acc += state
		u := (state*uint64(keyB) + y) & drmCS64Mask
		u = (uint64(rol16(uint32(u))) * 0xA27416F5) & drmCS64Mask
		u = (uint64(rol16(uint32(u))) * 0xD38396FF) & drmCS64Mask
		state = (uint64(rol16(uint32((uint64(rol16(uint32(u)))) * 0x7C932B89 & drmCS64Mask)))) *
			0xBFA49F69 & drmCS64Mask
		acc = (acc + state) & drmCS64Mask
	}
	return uint32(swapLast ^ state), uint32(swapAcc ^ acc)
}

// drmTokenHash returns the token's two parts, the checksum and the
// digest, as upper-case hex.
func drmTokenHash(data []byte) (string, string) {
	d := drmDigest(data)
	digest := make([]byte, 32)
	for i, w := range d {
		binary.BigEndian.PutUint32(digest[i*4:], w)
	}
	a, b := drmCS64(data, digest)
	cs := make([]byte, 8)
	binary.LittleEndian.PutUint32(cs[0:], a)
	binary.LittleEndian.PutUint32(cs[4:], b)
	return strings.ToUpper(hex.EncodeToString(cs)), strings.ToUpper(hex.EncodeToString(digest))
}
