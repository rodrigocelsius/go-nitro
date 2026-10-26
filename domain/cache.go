package domain

import (
	"context"
	"time"
)

// Cache is the primary cache interface.
type Cache interface {
	Get(key string, dest any) error
	Set(key string, value any, ttl time.Duration) error
	Delete(key string) error
	Invalidate(pattern string) error
	Clear() error
}

type bypassCacheKey struct{}

// BypassCache marks ctx so cached repositories read from the source of truth
// (Resource write paths load the row they are about to change this way).
func BypassCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, bypassCacheKey{}, true)
}

// CacheBypassed reports whether reads with ctx must skip caches: marked with
// BypassCache or running inside a transaction.
func CacheBypassed(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	if v, _ := ctx.Value(bypassCacheKey{}).(bool); v {
		return true
	}
	return InTx(ctx)
}
