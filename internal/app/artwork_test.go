package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"
)

// An animated WebP cover, as some playlists have, shows its first frame,
// in the bitmap too, which decodes its data again once it drops its pixels.
func TestAnimatedWebPCover(t *testing.T) {
	b, err := os.ReadFile("testdata/animated.webp")
	if err != nil {
		t.Fatal(err)
	}
	bmp, img, err := decodeArt(b)
	if err != nil {
		t.Fatal(err)
	}
	if r := img.Bounds(); r.Dx() != 24 || r.Dy() != 24 {
		t.Errorf("first frame is %v, want 24×24", r)
	}
	if w, h := bmp.Size(); w != 24 || h != 24 {
		t.Errorf("bitmap is %d×%d, want 24×24", w, h)
	}
}

// Covers not asked for in a while go, so that a long session does not
// keep every cover it showed; the ones still showing stay.
func TestArtCacheDropsUnusedCovers(t *testing.T) {
	now := time.Unix(1e9, 0)
	c := newArtCache(func(func()) {})
	c.now, c.swept = func() time.Time { return now }, now
	c.art["shown"] = &artwork{used: now}
	c.art["gone"] = &artwork{used: now}
	for range 7 {
		now = now.Add(time.Minute)
		c.get("shown", false)
	}
	if _, ok := c.art["gone"]; ok {
		t.Error("kept a cover not asked for in 7 minutes")
	}
	if _, ok := c.art["shown"]; !ok {
		t.Error("dropped a cover asked for every minute")
	}
}

// A cover whose CDN ignores the size asked for, as 汽水's does, is
// brought down to the size it shows at as it loads: 1600×1600 decoded
// whole for a 22-pixel row cost 19 ms on every scroll back into view.
func TestCoverBroughtDownToItsSize(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1600, 1600))
	for y := range 1600 {
		for x := range 1600 {
			src.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()

	full, img, err := decodeArt(b)
	if err != nil {
		t.Fatal(err)
	}
	if r := img.Bounds(); r.Dx() != 1600 || r.Dy() != 1600 {
		t.Errorf("unsized cover is %v, want 1600×1600", r)
	}
	if w, h := full.Size(); w != 1600 || h != 1600 {
		t.Errorf("unsized bitmap is %d×%d, want 1600×1600", w, h)
	}

	for _, side := range []int{88, 220} {
		bmp, img, err := decodeArtAt(b, side)
		if err != nil {
			t.Fatal(err)
		}
		if r := img.Bounds(); r.Dx() != side || r.Dy() != side {
			t.Errorf("cover at %d is %v, want %d×%d", side, r, side, side)
		}
		if w, h := bmp.Size(); w != side || h != side {
			t.Errorf("bitmap at %d is %d×%d, want the same size", side, w, h)
		}
	}

	// A picture already small enough loads as it is.
	small, _, err := decodeArtAt(b, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if w, _ := small.Size(); w != 1600 {
		t.Errorf("cover asked at 4000 is %d wide, want it left at 1600", w)
	}
}

// The covers showing when the window went idle stay for its next frame.
// They went: nothing asks for covers while the window draws nothing, so
// after 5 minutes away the first frame, as the window was focused again,
// dropped every cover on the page, which flashed as they loaded again.
func TestArtCacheKeepsCoversWhileIdle(t *testing.T) {
	now := time.Unix(1e9, 0)
	c := newArtCache(func(func()) {})
	c.now, c.swept = func() time.Time { return now }, now
	c.art["shown"] = &artwork{used: now}
	c.get("shown", false)
	now = now.Add(10 * time.Minute)
	if c.get("shown", false) == nil {
		t.Error("dropped the covers showing after 10 minutes idle")
	}
}

// A cover that came as a JPEG decodes to YCbCr, whose pixels fitSquare
// reads through a copy: it comes out the shape asked for all the same.
func TestFitSquareOnAnOblongCover(t *testing.T) {
	src := image.NewYCbCr(image.Rect(0, 0, 1600, 900), image.YCbCrSubsampleRatio420)
	for i := range src.Y {
		src.Y[i] = uint8(i)
	}
	out := fitSquare(src, 88)
	if out == nil {
		t.Fatal("fitSquare returned nil for a 1600×900 cover at 88")
	}
	if r := out.Bounds(); r.Dx() != 88 || r.Dy() != 49 {
		t.Errorf("the cover came out %v, want 88×49", r)
	}
	// A square that fits is left alone.
	if out := fitSquare(src, 2000); out != nil {
		t.Errorf("fitSquare made a %v of a cover that fits", out.Bounds())
	}
}
