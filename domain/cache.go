package domain

import "time"

// Cache is the primary cache interface.
type Cache interface {
	Get(key string, dest any) error
	Set(key string, value any, ttl time.Duration) error
	Delete(key string) error
	Invalidate(pattern string) error
	Clear() error
}
