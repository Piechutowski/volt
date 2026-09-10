package par

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestForVisitsEveryIndexOnce(t *testing.T) {
	for _, n := range []int{0, 1, 3, 100, 10007} {
		seen := make([]int32, n)
		For(n, func(i int) { atomic.AddInt32(&seen[i], 1) })
		for i, c := range seen {
			if c != 1 {
				t.Fatalf("n=%d: index %d visited %d times", n, i, c)
			}
		}
	}
}

// TestForFansOut proves the items run concurrently: as many items as
// workers rendezvous, each waiting for the others; a pool that ran
// them one after another would never let the first one finish.
func TestForFansOut(t *testing.T) {
	workers := min(4, runtime.GOMAXPROCS(0))
	if workers < 2 {
		t.Skip("one CPU")
	}
	var arrived atomic.Int32
	release := make(chan struct{})
	timedOut := atomic.Bool{}
	For(workers, func(int) {
		if arrived.Add(1) == int32(workers) {
			close(release)
		}
		select {
		case <-release:
		case <-time.After(5 * time.Second):
			timedOut.Store(true)
		}
	})
	if timedOut.Load() {
		t.Fatalf("%d items did not run concurrently on %d workers", workers, workers)
	}
}
