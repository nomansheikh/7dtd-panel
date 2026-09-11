package api

import (
	"sync"
	"time"
)

// rateLimiter is a fixed-window counter keyed by string, used to slow password
// guessing on the login endpoint.
//
// A fixed window allows a burst at a boundary, which is fine here: the goal is
// to make brute force impractical, not to meter traffic precisely.
type rateLimiter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	count     int
	windowEnd time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string]*bucket),
	}
}

// Allow records an attempt for key and reports whether it is within the limit.
func (r *rateLimiter) Allow(key string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Opportunistic sweep so a long-running panel does not accumulate a bucket
	// per address that ever tried to log in.
	if len(r.buckets) > 1024 {
		for k, b := range r.buckets {
			if now.After(b.windowEnd) {
				delete(r.buckets, k)
			}
		}
	}

	b, ok := r.buckets[key]
	if !ok || now.After(b.windowEnd) {
		r.buckets[key] = &bucket{count: 1, windowEnd: now.Add(r.window)}
		return true
	}
	if b.count >= r.limit {
		return false
	}
	b.count++
	return true
}

// Reset clears a key, called after a successful login so a legitimate operator
// who fumbled their password a few times is not left throttled.
func (r *rateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.buckets, key)
}
