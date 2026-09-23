package tenancy

import (
	"log/slog"
	"sync"
	"time"
)

// warmupCooldown keeps a request path from re-running the warmup back to
// back. Without it, traffic naming a namespace this service does not have
// kept a full re-scan of every tenant running continuously (audit E6).
const warmupCooldown = 30 * time.Second

// Warmup runs a tenant re-scan in the background, at most one at a time and
// no more often than the cooldown. A panic inside the scan is recovered:
// this runs detached, so an unrecovered one takes the process with it.
type Warmup struct {
	run func()

	mu       sync.Mutex
	running  bool
	lastDone time.Time
}

func NewWarmup(run func()) *Warmup {
	return &Warmup{run: run}
}

// TriggerAsync starts a scan unless one is running or one finished within
// the cooldown. It never blocks the caller.
func (w *Warmup) TriggerAsync() {
	if w == nil || w.run == nil {
		return
	}

	w.mu.Lock()
	if w.running || time.Since(w.lastDone) < warmupCooldown {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go func() {
		defer func() {
			w.mu.Lock()
			w.running = false
			w.lastDone = time.Now()
			w.mu.Unlock()
		}()
		defer func() {
			if r := recover(); r != nil {
				slog.Default().Error("tenant warmup panicked", slog.Any("panic", r))
			}
		}()
		w.run()
	}()
}
