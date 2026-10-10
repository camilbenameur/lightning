// Package ratelimit implements a per-key token-bucket rate limiter.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// sweepInterval is how often idle buckets are evicted.
const sweepInterval = time.Minute

// Limiter allows perMinute events per key with bursts of up to burst events.
// It is safe for concurrent use. Buckets of idle keys are evicted, so memory
// stays proportional to the number of recently active keys.
type Limiter struct {
	limit rate.Limit
	burst int
	// idleTTL is how long a bucket takes to refill completely; evicting it
	// afterwards is indistinguishable from keeping it.
	idleTTL time.Duration
	now     func() time.Time

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// New returns a limiter; perMinute and burst must be positive.
func New(perMinute, burst int) *Limiter {
	limit := rate.Limit(float64(perMinute) / 60)
	return &Limiter{
		limit:   limit,
		burst:   burst,
		idleTTL: time.Duration(float64(burst) / float64(limit) * float64(time.Second)),
		now:     time.Now,
		buckets: make(map[string]*bucket),
	}
}

// Allow consumes a token for key. When none is available it returns false
// and the time until the next token.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastSweep) >= sweepInterval {
		l.sweep(now)
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.buckets[key] = b
	}
	b.lastSeen = now

	r := b.limiter.ReserveN(now, 1)
	if delay := r.DelayFrom(now); delay > 0 {
		r.CancelAt(now)
		return false, delay
	}
	return true, 0
}

func (l *Limiter) sweep(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.lastSeen) > l.idleTTL {
			delete(l.buckets, key)
		}
	}
	l.lastSweep = now
}

// Len returns the number of tracked keys.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
