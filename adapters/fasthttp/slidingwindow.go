package fasthttp

import (
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

// DefaultMaxRateLimitKeys bounds how many distinct keys (IPs, API keys...) a
// per-key limiter tracks at once. Keys can be attacker-controlled (headers),
// so an unbounded map would be a memory-exhaustion vector.
const DefaultMaxRateLimitKeys = 100_000

// newSlidingWindowAdapter returns a per-key sliding-window limiter.
//
// Earlier versions adapted a single usecases.SlidingWindow and ignored the
// key, so every client shared one global bucket: one abusive client could
// exhaust the budget for everybody. Each key now has its own window.
func newSlidingWindowAdapter(max int, window time.Duration) domain.RateLimiter {
	return NewPerKeySlidingWindow(max, window)
}

type keyedWindow struct {
	w    *usecases.SlidingWindow
	last time.Time
}

// perKeySlidingWindow supports per-key rate limiting with a bounded key set.
type perKeySlidingWindow struct {
	mu       sync.Mutex
	windows  map[string]*keyedWindow
	overflow *usecases.SlidingWindow // shared by keys that do not fit in the table
	max      int
	window   time.Duration
	maxKeys  int
}

// NewPerKeySlidingWindow creates a sliding window that keeps counters per key.
// At most DefaultMaxRateLimitKeys keys are tracked; expired keys are swept
// when the table is full, and keys that still do not fit share one overflow
// window (fail-closed: a key flood cannot reset or evict other clients).
func NewPerKeySlidingWindow(max int, window time.Duration) *perKeySlidingWindow {
	return newPerKeySlidingWindow(max, window, DefaultMaxRateLimitKeys)
}

func newPerKeySlidingWindow(max int, window time.Duration, maxKeys int) *perKeySlidingWindow {
	if maxKeys <= 0 {
		maxKeys = DefaultMaxRateLimitKeys
	}
	return &perKeySlidingWindow{
		windows:  make(map[string]*keyedWindow),
		overflow: usecases.NewSlidingWindow(max, window),
		max:      max,
		window:   window,
		maxKeys:  maxKeys,
	}
}

// get returns the window for key. With create=false it never inserts.
func (p *perKeySlidingWindow) get(key string, create bool) *usecases.SlidingWindow {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	if kw, ok := p.windows[key]; ok {
		kw.last = now
		return kw.w
	}
	if !create {
		return nil
	}
	if len(p.windows) >= p.maxKeys {
		p.sweepLocked(now)
		if len(p.windows) >= p.maxKeys {
			return p.overflow
		}
	}
	kw := &keyedWindow{w: usecases.NewSlidingWindow(p.max, p.window), last: now}
	p.windows[key] = kw
	return kw.w
}

func (p *perKeySlidingWindow) sweepLocked(now time.Time) {
	for k, kw := range p.windows {
		if now.Sub(kw.last) > p.window {
			delete(p.windows, k)
		}
	}
}

// Allow reports whether the request is allowed for the key.
func (p *perKeySlidingWindow) Allow(key string) bool {
	return p.get(key, true).Allow()
}

// Remaining returns how many requests remain for the key.
func (p *perKeySlidingWindow) Remaining(key string) int {
	if w := p.get(key, false); w != nil {
		return w.Remaining()
	}
	return p.max
}

// Reset restarts the counter for the key.
func (p *perKeySlidingWindow) Reset(key string) {
	p.mu.Lock()
	delete(p.windows, key)
	p.mu.Unlock()
}
