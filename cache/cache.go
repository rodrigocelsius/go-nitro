// Package cache implements Nitro's cache abstraction.
//
// The cache supports two levels:
//   - L1: in-memory cache (fast, process-local)
//   - L2: distributed cache (Redis, Memcached, etc.)
//   - Hybrid: combines L1 + L2 with automatic invalidation
//
// Basic usage:
//
//	c := cache.NewMemory(cache.Config{
//	    MaxSize: 1000,
//	    TTL:     5 * time.Minute,
//	})
//	c.Set(ctx, "user:123", user, 0)
//	var user User
//	c.Get(ctx, "user:123", &user)
package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Cache is the interface for a cache.
type Cache interface {
	// Get retrieves a value from the cache.
	Get(ctx context.Context, key string, dest interface{}) error

	// Set stores a value in the cache.
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a value from the cache.
	Delete(ctx context.Context, key string) error

	// Exists reports whether a key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)

	// Clear clears the entire cache.
	Clear(ctx context.Context) error

	// Stats returns cache statistics.
	Stats(ctx context.Context) (*Stats, error)
}

// Stats holds cache statistics.
type Stats struct {
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Sets      int64 `json:"sets"`
	Deletes   int64 `json:"deletes"`
	Evictions int64 `json:"evictions"`
	Size      int64 `json:"size"`
}

// HitRate returns the cache hit rate.
func (s *Stats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// Config holds cache settings.
type Config struct {
	MaxSize int           `json:"max_size"`
	TTL     time.Duration `json:"ttl"`
}

// DefaultConfig returns default settings.
func DefaultConfig() Config {
	return Config{
		MaxSize: 1000,
		TTL:     5 * time.Minute,
	}
}

// entry represents a cache entry.
type entry struct {
	value     interface{}
	createdAt time.Time
	ttl       time.Duration
	expiresAt time.Time
}

func (e *entry) isExpired() bool {
	if e.ttl == 0 {
		return false
	}
	return time.Now().After(e.expiresAt)
}

// Memory implements an L1 in-memory cache.
type Memory struct {
	config Config
	items  map[string]*entry
	order  []string // LRU order
	stats  Stats
	mu     sync.RWMutex
}

// NewMemory creates a new in-memory cache.
func NewMemory(config Config) *Memory {
	if config.MaxSize <= 0 {
		config = DefaultConfig()
	}
	return &Memory{
		config: config,
		items:  make(map[string]*entry),
		order:  make([]string, 0),
	}
}

// Get retrieves a value from the in-memory cache.
func (c *Memory) Get(ctx context.Context, key string, dest interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.items[key]
	if !exists {
		c.stats.Misses++
		return ErrCacheMiss
	}

	if entry.isExpired() {
		c.delete(key)
		c.stats.Misses++
		return ErrCacheMiss
	}

	// Update LRU position
	c.updateLRU(key)

	c.stats.Hits++

	// Serialize the value
	if err := serializeValue(entry.value, dest); err != nil {
		return fmt.Errorf("cache: failed to deserialize: %w", err)
	}

	return nil
}

// Set stores a value in the in-memory cache.
func (c *Memory) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.config.TTL
	}

	// Avoid overflow
	if len(c.items) >= c.config.MaxSize {
		c.evict()
	}

	now := time.Now()
	c.items[key] = &entry{
		value:     value,
		createdAt: now,
		ttl:       ttl,
		expiresAt: now.Add(ttl),
	}

	c.updateLRU(key)
	c.stats.Sets++

	return nil
}

// Delete removes a value from the in-memory cache.
func (c *Memory) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.items[key]; !exists {
		return nil
	}

	c.delete(key)
	c.stats.Deletes++

	return nil
}

// Exists reports whether a key exists in the in-memory cache.
func (c *Memory) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.items[key]
	if !exists {
		return false, nil
	}

	if entry.isExpired() {
		return false, nil
	}

	return true, nil
}

// Clear clears the entire in-memory cache.
func (c *Memory) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*entry)
	c.order = make([]string, 0)

	return nil
}

// Stats returns in-memory cache statistics.
func (c *Memory) Stats(ctx context.Context) (*Stats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	stats.Size = int64(len(c.items))
	return &stats, nil
}

// delete removes a key from the cache (must be called with mu.Lock()).
func (c *Memory) delete(key string) {
	delete(c.items, key)
	c.removeFromLRU(key)
}

// updateLRU updates a key's position in the LRU.
func (c *Memory) updateLRU(key string) {
	c.removeFromLRU(key)
	c.order = append(c.order, key)
}

// removeFromLRU removes a key from the LRU list.
func (c *Memory) removeFromLRU(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

// evict removes the oldest cache entry.
func (c *Memory) evict() {
	if len(c.order) == 0 {
		return
	}

	oldest := c.order[0]
	c.delete(oldest)
	c.stats.Evictions++
}

// serializeValue copies the value to dest using simple serialization.
func serializeValue(src, dest interface{}) error {
	// For simple types, use a direct conversion
	switch d := dest.(type) {
	case *interface{}:
		*d = src
	case *string:
		if v, ok := src.(string); ok {
			*d = v
		}
	case *int:
		if v, ok := src.(int); ok {
			*d = v
		}
	case *int64:
		if v, ok := src.(int64); ok {
			*d = v
		}
	case *float64:
		if v, ok := src.(float64); ok {
			*d = v
		}
	case *bool:
		if v, ok := src.(bool); ok {
			*d = v
		}
	default:
		// For structs, try assigning directly
		// Note: this does not work for pointers, but is enough for simple examples
		return fmt.Errorf("cache: unsupported destination type %T", dest)
	}
	return nil
}

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = fmt.Errorf("cache: miss")
