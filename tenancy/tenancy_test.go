package tenancy

import (
	"sync"
	"testing"
	"time"
)

type modelA struct {
	ID   string `gorm:"primaryKey"`
	Name string
}

type modelARenamedField struct {
	ID    string `gorm:"primaryKey"`
	Title string
}

type modelARetagged struct {
	ID   string `gorm:"primaryKey;index"`
	Name string
}

type modelB struct {
	ID string
}

func TestComputeTargetVersion(t *testing.T) {
	base := ComputeTargetVersion([]any{modelA{}, modelB{}})

	if base == "" {
		t.Fatal("a version is expected for a non-empty model set")
	}
	if same := ComputeTargetVersion([]any{modelA{}, modelB{}}); same != base {
		t.Fatalf("the same models must give the same version: %s vs %s", base, same)
	}
	// Registration order is an implementation detail of app wiring, not a
	// schema change.
	if reordered := ComputeTargetVersion([]any{modelB{}, modelA{}}); reordered != base {
		t.Fatalf("order changed the version: %s vs %s", base, reordered)
	}
	// Pointers and values describe the same table.
	if ptr := ComputeTargetVersion([]any{&modelA{}, &modelB{}}); ptr != base {
		t.Fatalf("a pointer model changed the version: %s vs %s", base, ptr)
	}

	for name, models := range map[string][]any{
		"a renamed field": {modelARenamedField{}, modelB{}},
		"a changed tag":   {modelARetagged{}, modelB{}},
		"a dropped model": {modelA{}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := ComputeTargetVersion(models); got == base {
				t.Fatalf("%s must change the version, still %s", name, got)
			}
		})
	}
}

func TestRetryDelay(t *testing.T) {
	if d := retryDelay(0); d != retryBaseDelay {
		t.Fatalf("the first retry should use the base delay, got %s", d)
	}
	previous := time.Duration(0)
	for i := 1; i <= 12; i++ {
		d := retryDelay(i)
		if d < previous {
			t.Fatalf("delay went backwards at failure %d: %s after %s", i, d, previous)
		}
		if d > retryMaxDelay {
			t.Fatalf("delay %s at failure %d exceeds the cap %s", d, i, retryMaxDelay)
		}
		previous = d
	}
	if retryDelay(50) != retryMaxDelay {
		t.Fatal("a tenant that keeps failing should settle at the cap, not grow forever")
	}
}

func TestWarmup_RunsOnceAtATime(t *testing.T) {
	var mu sync.Mutex
	running, runs := 0, 0
	release := make(chan struct{})

	w := NewWarmup(func() {
		mu.Lock()
		running++
		runs++
		if running > 1 {
			mu.Unlock()
			t.Error("two warmups ran at once")
			return
		}
		mu.Unlock()
		<-release
		mu.Lock()
		running--
		mu.Unlock()
	})

	for i := 0; i < 20; i++ {
		w.TriggerAsync()
	}
	close(release)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := runs > 0 && running == 0
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if runs == 0 {
		t.Fatal("the warmup never ran")
	}
	if runs > 1 {
		t.Fatalf("20 triggers started %d scans; one at a time was the point", runs)
	}
}

// A request naming a namespace this service does not have used to keep the
// warmup running back to back (audit E6).
func TestWarmup_HonoursTheCooldown(t *testing.T) {
	var mu sync.Mutex
	runs := 0
	w := NewWarmup(func() {
		mu.Lock()
		runs++
		mu.Unlock()
	})

	w.TriggerAsync()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := runs == 1
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	for i := 0; i < 10; i++ {
		w.TriggerAsync()
	}
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if runs != 1 {
		t.Fatalf("expected the cooldown to hold further scans, got %d runs", runs)
	}
}

func TestWarmup_SurvivesAPanicInTheScan(t *testing.T) {
	done := make(chan struct{})
	w := NewWarmup(func() {
		defer close(done)
		panic("boom")
	})

	w.TriggerAsync()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the scan never ran")
	}
	time.Sleep(20 * time.Millisecond) // let the recover run

	// The process is still here, and the warmup is usable again.
	w.mu.Lock()
	stillRunning := w.running
	w.mu.Unlock()
	if stillRunning {
		t.Fatal("a panicked scan left the warmup marked as running")
	}
}

func TestWarmup_NilIsSafe(t *testing.T) {
	var w *Warmup
	w.TriggerAsync()
	NewWarmup(nil).TriggerAsync()
}
