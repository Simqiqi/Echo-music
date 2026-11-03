package dj

import (
	"os"
	"testing"
	"time"
)

// TestEdgeSpeak asks the service for a line, when the network has it:
// the result is decoded, mastered, and left as a WAV to listen to.
func TestEdgeSpeak(t *testing.T) {
	if os.Getenv("NM_EDGE") == "" {
		t.Skip("NM_EDGE not set: the network is not on trial here")
	}
	start := time.Now()
	rate, samples, engine, err := New("").make("接下来这首《晴天》，是周杰伦两千零三年的作品，来自专辑《叶惠美》。")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("engine=%s rate=%d samples=%d dur=%.1fs in %s",
		engine, rate, len(samples), float64(len(samples))/float64(rate), time.Since(start).Round(time.Millisecond))
	if engine != "edge" {
		t.Fatalf("the service should have answered, but the line fell back to %s", engine)
	}
	if len(samples) < rate { // under a second means it said nothing
		t.Fatalf("the clip is too short: %d samples", len(samples))
	}
	if dir := os.Getenv("NM_EDGE_DIR"); dir != "" {
		if b, err := encodeWAV(rate, samples); err == nil {
			p := dir + "/edge-dj.wav"
			if err := os.WriteFile(p, b, 0o644); err == nil {
				t.Logf("wrote %s", p)
			}
		}
	}
}
