package player

import (
	"errors"
	"fmt"
	"testing"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// Only a song the account cannot play is skipped; a failure on the way to it is not.
func TestSkipOnError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"no error", nil, false},
		{"network", errors.New("Post: dial tcp: timeout"), false},
		{"decode", errors.New("mp3: no sync found"), false},
		{"unavailable", netease.ErrUnavailable, true},
		{"wrapped unavailable", fmt.Errorf("song url: %w", netease.ErrUnavailable), true},
		{"format", netease.ErrFormat, true},
		{"wrapped format", fmt.Errorf("%w %s", netease.ErrFormat, "ogg"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := skipOnError(tc.err); got != tc.want {
				t.Errorf("skipOnError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// A failed URL request used to skip to the next song, so clicking a track in
// a playlist played one further down the list; only unplayable songs may skip.
func TestNextOnFailure(t *testing.T) {
	transient := errors.New("Post: dial tcp: timeout")
	for _, tc := range []struct {
		name    string
		err     error
		at      int
		skipped int
		n       int
		want    int
		wantOK  bool
	}{
		{"transient stays", transient, 3, 0, 10, 3, false},
		{"wrapped transient stays", fmt.Errorf("open: %w", transient), 3, 0, 10, 3, false},
		{"unavailable skips", netease.ErrUnavailable, 3, 0, 10, 4, true},
		{"wrapped unavailable skips", fmt.Errorf("open: %w", netease.ErrUnavailable), 3, 0, 10, 4, true},
		{"format skips", fmt.Errorf("%w %s", netease.ErrFormat, "ogg"), 3, 0, 10, 4, true},
		{"wraps at the end", netease.ErrUnavailable, 9, 0, 10, 0, true},
		{"whole queue tried", netease.ErrUnavailable, 5, 9, 10, 5, false},
		{"one song only", netease.ErrUnavailable, 0, 0, 1, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := nextOnFailure(tc.err, tc.at, tc.skipped, tc.n)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("nextOnFailure(%v, %d, %d, %d) = (%d, %v), want (%d, %v)",
					tc.err, tc.at, tc.skipped, tc.n, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
