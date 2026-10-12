package couchtv

import (
	"sync"
	"time"
)

// rateLimiter is a small sliding-window limiter: at most max events per window per
// key. It bounds guest joins per IP, so a leaked link cannot be hammered into
// guest rows.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: map[string][]time.Time{}, max: max, window: window}
}

func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hits) > 10_000 {
		for k, hits := range l.hits {
			if now.Sub(hits[len(hits)-1]) > l.window {
				delete(l.hits, k)
			}
		}
	}
	kept := recent(l.hits[key], now, l.window)
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

// recent keeps the times within window of now.
func recent(times []time.Time, now time.Time, window time.Duration) []time.Time {
	kept := times[:0]
	for _, t := range times {
		if now.Sub(t) < window {
			kept = append(kept, t)
		}
	}
	return kept
}
