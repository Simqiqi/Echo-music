//go:build darwin

package dj

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// available reports whether the system's synthesizer is there to speak
// with.
func available() bool {
	_, err := os.Stat(sayPath)
	return err == nil
}

const sayPath = "/usr/bin/say"

var voicePick struct {
	sync.Once
	name string
}

// voiceName picks the voice: a Mandarin one, Tingting the system's own,
// unless the listener has downloaded a Premium or Enhanced one of the
// same language, which the synthesizer names as such.
func voiceName() string {
	voicePick.Do(func() {
		voicePick.name = "Tingting"
		out, err := exec.Command(sayPath, "-v", "?").Output()
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(out), "\n") {
			// "Tingting            zh_CN    # 你好！我叫婷婷。"
			i := strings.Index(line, "zh_CN")
			if i <= 0 {
				continue
			}
			name := strings.TrimSpace(line[:i])
			if name == "" {
				continue
			}
			low := strings.ToLower(name)
			if strings.Contains(low, "premium") || strings.Contains(low, "enhanced") {
				voicePick.name = name
				return
			}
		}
	})
	return voicePick.name
}

// synthesize has the system say text, at rate and pitch, and returns the
// WAV it wrote: mono, 16-bit, 22.05 kHz.
func synthesize(text string, rate, pitch int) ([]byte, error) {
	dir, err := os.MkdirTemp("", "echo-dj-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "voice.wav")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// The pitch goes in the text, as the synthesizer's own command, a
	// touch below the voice's middle for a deeper, closer cast.
	msg := fmt.Sprintf("[[pbas %d]] %s", pitch, text)
	cmd := exec.CommandContext(ctx, sayPath,
		"-v", voiceName(),
		"-r", strconv.Itoa(rate),
		"-o", path,
		"--data-format=LEI16@22050",
		msg,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("dj: say: %v: %s", err, strings.TrimSpace(string(out)))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < 44 {
		return nil, errors.New("dj: the voice said nothing")
	}
	return b, nil
}
