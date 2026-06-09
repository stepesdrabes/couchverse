package api

import (
	"sync"
	"time"
)

// fixed-window in-memory rate limiter, good enough for login throttling
// on a single-instance deployment.
type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*rateEntry
}

type rateEntry struct {
	count int
	start time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, entries: map[string]*rateEntry{}}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if len(rl.entries) > 10000 {
		for k, e := range rl.entries {
			if now.Sub(e.start) > rl.window {
				delete(rl.entries, k)
			}
		}
	}

	e, ok := rl.entries[key]
	if !ok || now.Sub(e.start) > rl.window {
		rl.entries[key] = &rateEntry{count: 1, start: now}
		return true
	}
	e.count++
	return e.count <= rl.limit
}
