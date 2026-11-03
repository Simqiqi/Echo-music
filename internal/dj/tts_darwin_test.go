//go:build darwin

package dj

import (
	"testing"
	"time"
)

func TestSay(t *testing.T) {
	if !Available() {
		t.Skip("no voice on this system")
	}
	dir := t.TempDir()
	v := New(dir)
	text := "接下来这首《晴天》，周杰伦。收录在两千零三年的专辑《叶惠美》里。"

	s, err := v.Say(text)
	if err != nil {
		t.Fatal(err)
	}
	if s.Duration() < 500*time.Millisecond {
		t.Fatalf("the voice is too short: %v", s.Duration())
	}
	// The same words come back as the same sound, from memory.
	again, err := v.Say(text)
	if err != nil {
		t.Fatal(err)
	}
	if again != s {
		t.Fatal("the line was not kept in memory")
	}
	// And from disk, for the app's next run.
	other := New(dir)
	fromDisk, err := other.Say(text)
	if err != nil {
		t.Fatal(err)
	}
	if d := fromDisk.Duration(); d != s.Duration() {
		t.Fatalf("the kept line is %v, the made one %v", d, s.Duration())
	}
}
