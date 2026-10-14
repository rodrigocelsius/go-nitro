package usecases

import (
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Remember runs fn and caches the typed result (prefer nitro.Remember[T] in pkg).
// Deprecated: use nitro.Remember — this version serializes twice with RedisCache.
func Remember(c domain.Cache, key string, ttl time.Duration, fn func() (any, error), dest any) error {
	if err := c.Get(key, dest); err == nil {
		return nil
	}

	value, err := fn()
	if err != nil {
		return err
	}
	if err := c.Set(key, value, ttl); err != nil {
		return err
	}
	return c.Get(key, dest)
}

// cacheEntry represents a cache entry.
type cacheEntry struct {
	Value     []byte
	ExpiresAt time.Time
}

// CacheMiddleware returns middleware that caches the response.
func CacheMiddleware(c domain.Cache, ttl time.Duration) domain.Handler {
	return func(ctx *domain.Ctx) error {
		key := ctx.Method() + ":" + ctx.Path()
		var cached []byte
		if err := c.Get(key, &cached); err == nil {
			ctx.SetHeader("Content-Type", "application/json")
			ctx.SetBody(cached)
			ctx.SetHeader("X-Cache", "HIT")
			return nil
		}

		ctx.SetHeader("X-Cache", "MISS")
		return nil
	}
}
