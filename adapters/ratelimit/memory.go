package ratelimit

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

// rateLimiter is the internal interface for limiters (TokenBucket or SlidingWindow).
type rateLimiter interface {
	Allow() bool
	Remaining() int
	Reset()
}

// maxBuckets bounds the bucket map; beyond it, buckets idle for more than
// two windows are swept (memory exhaustion guard for attacker-controlled keys).
const maxBuckets = 8192

type bucketEntry struct {
	limiter rateLimiter
	// lastSeen is unix-nano, updated on every access (atomic: read under
	// RLock, written without extra locking on the hot Allow path).
	lastSeen atomic.Int64
}

// MemoryLimiter implements domain.RateLimiter in memory.
type MemoryLimiter struct {
	mu       sync.RWMutex
	buckets  map[string]*bucketEntry
	max      int
	window   time.Duration
	strategy domain.RateLimitStrategy
}

// NewMemoryLimiter creates a MemoryLimiter with token bucket (default).
func NewMemoryLimiter(max int, window time.Duration) *MemoryLimiter {
	return &MemoryLimiter{
		buckets:  make(map[string]*bucketEntry),
		max:      max,
		window:   window,
		strategy: domain.StrategyTokenBucket,
	}
}

// NewSlidingWindowLimiter creates a MemoryLimiter with sliding window.
func NewSlidingWindowLimiter(max int, window time.Duration) *MemoryLimiter {
	return &MemoryLimiter{
		buckets:  make(map[string]*bucketEntry),
		max:      max,
		window:   window,
		strategy: domain.StrategySlidingWindow,
	}
}

func (m *MemoryLimiter) getOrCreate(key string) rateLimiter {
	m.mu.RLock()
	entry, ok := m.buckets[key]
	if ok {
		entry.lastSeen.Store(time.Now().UnixNano())
	}
	m.mu.RUnlock()
	if ok {
		return entry.limiter
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok = m.buckets[key]
	if !ok {
		switch m.strategy {
		case domain.StrategySlidingWindow:
			b := usecases.NewSlidingWindow(m.max, m.window)
			entry = &bucketEntry{limiter: b}
		default:
			b := usecases.NewTokenBucket(m.max, m.window)
			entry = &bucketEntry{limiter: b}
		}
		m.buckets[key] = entry
		if len(m.buckets) > maxBuckets {
			m.sweepLocked(time.Now())
		}
	}
	entry.lastSeen.Store(time.Now().UnixNano())
	return entry.limiter
}

// sweepLocked removes buckets idle for more than two windows.
// Caller holds the write lock.
func (m *MemoryLimiter) sweepLocked(now time.Time) {
	cutoff := now.Add(-2 * m.window).UnixNano()
	for k, e := range m.buckets {
		if e.lastSeen.Load() < cutoff {
			delete(m.buckets, k)
		}
	}
}

// Allow reports whether the request is allowed.
func (m *MemoryLimiter) Allow(key string) bool {
	return m.getOrCreate(key).Allow()
}

// Remaining returns how many tokens remain.
func (m *MemoryLimiter) Remaining(key string) int {
	return m.getOrCreate(key).Remaining()
}

// Reset restarts the bucket.
func (m *MemoryLimiter) Reset(key string) {
	m.getOrCreate(key).Reset()
}
