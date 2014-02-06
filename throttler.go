package tb

import (
	"math"
	"sync"
	"time"
)

// Throttler is a thread-safe wrapper around a map of buckets and an easy to
// use API for generic throttling.
type Throttler struct {
	mu      sync.RWMutex
	buckets map[string]*Bucket
	closing chan struct{}
}

// NewThrottler returns a Throttler with a single filler go-routine for all
// its Buckets which ticks every hz.
// The number of tokens added on each tick for each bucket is computed
// dynamically to be even accross the duration of a second.
//
// If hz <= 0, the filling go-routine won't be started.
func NewThrottler(hz time.Duration) *Throttler {
	th := &Throttler{
		buckets: map[string]*Bucket{},
		closing: make(chan struct{}),
	}

	if hz > 0 {
		go th.fill(hz)
	}

	return th
}

// Throttle throttles a quantity 'in' to the specified 'rate' per second,
// with a Bucket keyed by key, returning the permitted quantity.
// This method is thread-safe, locks are used only to synchronize access to
// the bucket map.
//
// If hz < 1/rate seconds, the effective throttling rate won't be correct.
//
// You must call Close when you're done with the Throttler in order to not leak
// a go-routine and a system-timer.
func (t *Throttler) Throttle(key string, in, rate int64) (out int64) {
	t.mu.RLock()
	b := t.buckets[key]
	t.mu.RUnlock()

	if b == nil {
		b = NewBucket(rate, 0)
		t.mu.Lock()
		t.buckets[key] = b
		t.mu.Unlock()
	}

	return b.Take(in)
}

// Close stops filling the Buckets
func (t *Throttler) Close() error {
	close(t.closing)
	return nil
}

func (t Throttler) fill(hz time.Duration) {
	ticker := time.NewTicker(hz)
	defer ticker.Stop()

	for _ = range ticker.C {
		select {
		case <-t.closing:
			return
		default:
		}
		t.mu.RLock()
		for _, b := range t.buckets {
			b.Put(int64(math.Floor(.5 + (float64(b.capacity) * hz.Seconds()))))
		}
		t.mu.RUnlock()
	}
}
