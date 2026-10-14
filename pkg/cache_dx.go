package nitro

import (
	"log"
	"time"
)

// Bust invalidates cache key patterns (nil-safe). Prefer patterns over Delete+Invalidate.
//
//	nitro.Bust(cache, "drivers:*")
func Bust(c Cache, patterns ...string) {
	if c == nil {
		return
	}
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if err := c.Invalidate(p); err != nil {
			log.Printf("nitro: cache bust %q: %v", p, err)
		}
	}
}

// Remember returns a cached value or runs fn and stores the result with ttl.
// If c is nil, fn always runs. ttl <= 0 lets the Cache implementation use its default.
//
//	drivers, err := nitro.Remember(cache, "drivers:all", 10*time.Minute, func() ([]Driver, error) {
//	    return store.Find(ctx, q)
//	})
func Remember[T any](c Cache, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	var zero T
	if c != nil && key != "" {
		var cached T
		if err := c.Get(key, &cached); err == nil {
			return cached, nil
		}
	}
	v, err := fn()
	if err != nil {
		return zero, err
	}
	if c != nil && key != "" {
		if err := c.Set(key, v, ttl); err != nil {
			log.Printf("nitro: cache set %q: %v", key, err)
		}
	}
	return v, nil
}
