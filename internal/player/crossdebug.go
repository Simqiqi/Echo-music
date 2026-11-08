package player

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// debugCross records what the handover does, for tracking one that does
// not begin: CROSS_DEBUG names the file to write it to. Nothing is
// written when it is unset.
var (
	debugMu   sync.Mutex
	debugLast = time.Now()
)

func debugCross(format string, args ...any) {
	path := os.Getenv("CROSS_DEBUG")
	if path == "" {
		return
	}
	debugMu.Lock()
	defer debugMu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	now := time.Now()
	fmt.Fprintf(f, "%s (+%6dms) %s\n", now.Format("15:04:05.000"), now.Sub(debugLast).Milliseconds(), fmt.Sprintf(format, args...))
	debugLast = now
}
