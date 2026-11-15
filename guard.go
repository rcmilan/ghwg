package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// guard is the safety net around a person's game (D1, after a live test froze the whole computer while T
// was held): if one Update or Draw is still running after stallLimit, or the process holds more than
// memLimit, it writes why and every goroutine's stack to a log and quits, before the machine is starved.
// A dragged or unfocused window is not a stall: nothing is running then.
const (
	stallLimit = 3 * time.Second
	memLimit   = 2 << 30
)

type guard struct {
	ebiten.Game
	since atomic.Int64 // when the running Update or Draw began (UnixNano), 0 between them
}

func (w *guard) Update() error {
	w.since.Store(time.Now().UnixNano())
	defer w.since.Store(0)
	return w.Game.Update()
}

func (w *guard) Draw(screen *ebiten.Image) {
	w.since.Store(time.Now().UnixNano())
	defer w.since.Store(0)
	w.Game.Draw(screen)
}

func (w *guard) watch() {
	if runtime.GOOS == "js" { // a browser tab is its own sandbox: no temp log to write, and os.Exit would only freeze the page
		return
	}
	var m runtime.MemStats
	for range time.Tick(500 * time.Millisecond) {
		runtime.ReadMemStats(&m)
		why := ""
		if s := w.since.Load(); s != 0 && time.Since(time.Unix(0, s)) > stallLimit {
			why = fmt.Sprintf("a frame ran over %v", stallLimit)
		} else if m.Sys > memLimit {
			why = fmt.Sprintf("the game held %d MB", m.Sys>>20)
		}
		if why == "" {
			continue
		}
		stacks := make([]byte, 1<<20)
		stacks = stacks[:runtime.Stack(stacks, true)]
		path := filepath.Join(os.TempDir(), "ghosthotel-guard.log")
		os.WriteFile(path, fmt.Appendf(nil, "%s: quit, %s (heap %d MB)\n\n%s", time.Now().Format(time.RFC3339), why, m.HeapAlloc>>20, stacks), 0o644)
		fmt.Fprintf(os.Stderr, "Ghost Hotel quit to protect the computer: %s. Details: %s\n", why, path)
		os.Exit(3)
	}
}
