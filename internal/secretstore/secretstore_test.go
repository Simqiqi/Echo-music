package secretstore

import (
	"bytes"
	"testing"
)

// TestBigSecretRoundTrip keeps a session the size of 汽水's: too big for
// the keychain's per-item limit, so it goes to the private file, and comes
// back whole.
func TestBigSecretRoundTrip(t *testing.T) {
	big := bytes.Repeat([]byte(`{"guardCert":"-----BEGIN CERTIFICATE-----`), 80)
	if len(big) < 3000 {
		t.Fatalf("the sample is %d bytes, too small to test the file path", len(big))
	}
	name := "test-big-session"
	t.Cleanup(func() { Delete(name) })
	if err := Set(name, big); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := Get(name)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, big) {
		t.Fatalf("got %d bytes back, want %d", len(got), len(big))
	}
	Delete(name)
	if got, err := Get(name); err != nil || got != nil {
		t.Fatalf("after Delete: got (%d bytes, %v), want nothing", len(got), err)
	}
}
