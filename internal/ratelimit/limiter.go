// Package ratelimit provides a per-key token-bucket rate limiter, used to
// bound how fast any single client (keyed by IP) can hit the HTTP API.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter rate-limits independently per key, each key getting its own token
// bucket. Keys idle longer than the configured idleTimeout are evicted so a
// long-running node doesn't accumulate one bucket per key forever.
type Limiter struct {
	rps   rate.Limit
	burst int
	idle  time.Duration

	mu      sync.Mutex
	buckets map[string]*bucket

	done chan struct{}
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// New returns a Limiter allowing rps requests per second, per key, with the
// given burst size. A background goroutine evicts buckets that have gone
// idle for longer than idleTimeout; call Close to stop it.
func New(rps float64, burst int, idleTimeout time.Duration) *Limiter {
	l := &Limiter{
		rps:     rate.Limit(rps),
		burst:   burst,
		idle:    idleTimeout,
		buckets: make(map[string]*bucket),
		done:    make(chan struct{}),
	}
	go l.evictLoop()
	return l
}

// Allow reports whether a request for key is allowed right now, consuming a
// token from its bucket if so.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.buckets[key] = b
	}
	b.lastSeen = time.Now()
	return b.limiter.Allow()
}

// Close stops the background eviction goroutine.
func (l *Limiter) Close() {
	close(l.done)
}

func (l *Limiter) evictLoop() {
	ticker := time.NewTicker(l.idle)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.evictIdle()
		case <-l.done:
			return
		}
	}
}

// evictIdle removes buckets that have not been used for at least idle.
// Separated from evictLoop so tests can trigger eviction deterministically
// without waiting on a real ticker.
func (l *Limiter) evictIdle() {
	cutoff := time.Now().Add(-l.idle)
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(l.buckets, k)
		}
	}
}
