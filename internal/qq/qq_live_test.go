package qq

import (
	"os"
	"strings"
	"testing"
)

// The bridge hands back a QR image for either channel, and an unknown key
// counts as an expired one. LIVE=1 runs it: it starts the Node bridge and
// talks to QQ's login page.
func TestBridgeHandsBackAQR(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("LIVE is not set: skipping the bridge and QQ's login page")
	}
	for _, mode := range []string{LoginQQ, LoginWeChat} {
		key, image, err := StartQR(mode)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if key == "" {
			t.Fatalf("%s: no key, so the login could not be polled", mode)
		}
		if !strings.HasPrefix(key, mode+":") {
			t.Errorf("%s: key %q does not name its channel", mode, key)
		}
		if !strings.HasPrefix(image, "data:image/") {
			t.Errorf("%s: image is not a data URI: %.20q", mode, image)
		}
	}
	if state, err := CheckQR("qq:not-a-login"); err != nil || state != "expired" {
		t.Errorf("CheckQR(unknown key) = %q, %v; want expired", state, err)
	}
}
// Searching QQ's own catalogue finds songs, each with the ids playback
// needs. LIVE=1 runs it: it starts the bridge and reaches QQ.
func TestBridgeSearchFindsSongs(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("LIVE is not set: skipping the bridge and QQ's search")
	}
	items, _, _, err := Search("周杰伦", 5, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("search found nothing")
	}
	first := items[0]
	if first.ID == "" || first.Name == "" {
		t.Errorf("a result came back without an id or a name: %+v", first)
	}
}
