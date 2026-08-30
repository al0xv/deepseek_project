package gateway

import (
	"sync"
	"time"
)

// createLimiter is a simple fixed-window limiter for session creation.
type createLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	stamps []time.Time
}

func newCreateLimiter(limit int, window time.Duration) *createLimiter {
	return &createLimiter{limit: limit, window: window}
}

// Allow reports whether a new session creation is allowed right now.
func (l *createLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-l.window)
	kept := l.stamps[:0]
	for _, t := range l.stamps {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	l.stamps = kept
	if len(l.stamps) >= l.limit {
		return false
	}
	l.stamps = append(l.stamps, now)
	return true
}
