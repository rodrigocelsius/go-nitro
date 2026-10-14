package usecases

import (
	"sync"
	"time"
)

// TokenBucket implements token-bucket rate limiting with continuous refill:
// fractional tokens accumulate between calls instead of being discarded until
// a whole window has elapsed.
type TokenBucket struct {
	mu     sync.Mutex
	tokens float64
	max    int
	window time.Duration
	last   time.Time
}

// NewTokenBucket creates a new TokenBucket.
func NewTokenBucket(max int, window time.Duration) *TokenBucket {
	return &TokenBucket{
		tokens: float64(max),
		max:    max,
		window: window,
		last:   time.Now(),
	}
}

// Allow reports whether the request is allowed.
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if elapsed := now.Sub(b.last); elapsed > 0 && b.max > 0 && b.window > 0 {
		refill := elapsed.Seconds() / b.window.Seconds() * float64(b.max)
		b.tokens = minFloat(float64(b.max), b.tokens+refill)
	}
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Remaining returns how many full tokens are left.
func (b *TokenBucket) Remaining() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return int(b.tokens)
}

// Reset resets the bucket.
func (b *TokenBucket) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens = float64(b.max)
	b.last = time.Now()
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// SlidingWindow implements sliding-window-log rate limiting.
// It keeps timestamps of each request and drops expired entries.
type SlidingWindow struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	requests []time.Time
}

// NewSlidingWindow creates a new SlidingWindow.
func NewSlidingWindow(max int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		max:    max,
		window: window,
	}
}

// Allow reports whether the request is allowed within the window.
func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	sw.cleanExpired(now)

	if len(sw.requests) >= sw.max {
		return false
	}

	sw.requests = append(sw.requests, now)
	return true
}

// Remaining returns how many requests remain in the current window.
func (sw *SlidingWindow) Remaining() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.cleanExpired(time.Now())
	remaining := sw.max - len(sw.requests)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Reset clears all recorded requests.
func (sw *SlidingWindow) Reset() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.requests = sw.requests[:0]
}

// cleanExpired removes entries outside the time window.
// Must be called with the mutex held.
func (sw *SlidingWindow) cleanExpired(now time.Time) {
	cutoff := now.Add(-sw.window)
	i := 0
	for i < len(sw.requests) && sw.requests[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		sw.requests = sw.requests[i:]
	}
}
