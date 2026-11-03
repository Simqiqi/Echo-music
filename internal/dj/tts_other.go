//go:build !darwin

package dj

// available reports whether this system has a synthesizer to speak with.
// Only macOS does, for now: elsewhere the feature keeps quiet.
func available() bool { return false }

// voiceName names no voice.
func voiceName() string { return "none" }

// synthesize makes no voice here.
func synthesize(string, int, int) ([]byte, error) { return nil, ErrUnavailable }
