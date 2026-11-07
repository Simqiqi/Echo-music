package netease

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// The lengths of songs the catalogue gave none for, learned from their
// streams as they played: by the song's id, in milliseconds. A list shows
// the length from then on, and the app keeps it to the next it opens with,
// in lengths.json in the user data directory.
type lengthBook struct {
	mu    sync.RWMutex
	known map[int64]int64
	path  string
	once  sync.Once
}

var lengths = &lengthBook{known: map[int64]int64{}}

// load reads what earlier sessions learned, once, as an id is first asked
// about, so that an app that never plays anything touches no file.
func (b *lengthBook) load() {
	b.once.Do(func() {
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			return
		}
		b.path = filepath.Join(dir, "lengths.json")
		data, err := os.ReadFile(b.path)
		if err != nil {
			return
		}
		var kept map[int64]int64
		if json.Unmarshal(data, &kept) != nil {
			return
		}
		b.mu.Lock()
		for id, ms := range kept {
			if ms > 0 {
				b.known[id] = ms
			}
		}
		b.mu.Unlock()
	})
}

// lengthOf returns the length learned for a song, if one was.
func lengthOf(id int64) (time.Duration, bool) {
	lengths.load()
	lengths.mu.RLock()
	defer lengths.mu.RUnlock()
	ms, ok := lengths.known[id]
	if !ok || ms <= 0 {
		return 0, false
	}
	return time.Duration(ms) * time.Millisecond, true
}

// RememberLength keeps the length of a song whose stream has told it: the
// catalogue had none, as for a new release, an episode, or a song the
// service holds its details back for.
func RememberLength(id int64, d time.Duration) {
	ms := int64(d / time.Millisecond)
	if id == 0 || ms <= 0 {
		return
	}
	b := lengths
	b.load()
	b.mu.Lock()
	if b.known[id] == ms {
		b.mu.Unlock()
		return
	}
	b.known[id] = ms
	data, err := json.Marshal(b.known)
	path := b.path
	b.mu.Unlock()
	if err != nil || path == "" {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	// Small, and written but once a song: no need to debounce it.
	os.WriteFile(path, data, 0o600)
}

// KnownLengths is how many lengths were learned, for a test.
func KnownLengths() int {
	lengths.mu.RLock()
	defer lengths.mu.RUnlock()
	return len(lengths.known)
}
