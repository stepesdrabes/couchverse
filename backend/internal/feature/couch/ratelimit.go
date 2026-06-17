package couch

import (
	"sync"
	"time"
)

// rateLimiter is a small sliding-window limiter: at most max events per window
// per key. Used to bound join attempts from one IP so a leaked link can't be
// hammered to churn participants.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]int64
	max    int
	window time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: map[string][]int64{}, max: max, window: window}
}

func (l *rateLimiter) allow(key string) bool {
	now := time.Now().UnixNano()
	cutoff := now - int64(l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t > cutoff {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
