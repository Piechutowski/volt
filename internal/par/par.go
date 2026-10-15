// Package par runs independent work on every CPU (roadmap PERF-7). The
// caller keeps results in per-index slots, so the output is assembled
// in index order and never depends on the schedule.
package par

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// For calls fn(i) for every i in [0, n), on every CPU. With one CPU, or
// one item, it runs inline.
func For(n int, fn func(i int)) {
	workers := min(n, runtime.GOMAXPROCS(0))
	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				fn(i)
			}
		}()
	}
	wg.Wait()
}
