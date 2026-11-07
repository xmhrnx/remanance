package dupes

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// throttle lets a report through at most every 150ms, and the last one.
type throttle struct {
	mu   sync.Mutex
	last time.Time
}

// parallel runs fn on every index below n on all cores, and reports
// each one done.
func parallel(ctx context.Context, n, workers int, fn func(i int), done func(k int)) {
	var next, count atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				fn(i)
				done(int(count.Add(1)))
			}
		}()
	}
	wg.Wait()
}

func (t *throttle) ok(final bool) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !final && time.Since(t.last) < 150*time.Millisecond {
		return false
	}
	t.last = time.Now()
	return true
}
