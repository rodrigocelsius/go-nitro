package cache

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type cacheItem struct {
	Value     []byte
	ExpiresAt time.Time
}

// MemoryCache implements domain.Cache with per-item expiration.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
	max   int
	sets  int // Set calls since the last expired-entry sweep
}

// sweepEvery bounds memory when maxSize is 0 (unbounded): expired entries
// that are never read again are purged every sweepEvery writes.
const sweepEvery = 1024

// defaultMemoryTTL applies when Set receives ttl <= 0 (matches RedisCache).
const defaultMemoryTTL = 5 * time.Minute

// NewMemoryCache creates a new MemoryCache.
func NewMemoryCache(maxSize int) *MemoryCache {
	return &MemoryCache{
		items: make(map[string]cacheItem),
		max:   maxSize,
	}
}

// Get retrieves a value from the cache.
func (c *MemoryCache) Get(key string, dest any) error {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return domain.ErrCacheMiss{}
	}
	if time.Now().After(item.ExpiresAt) {
		c.mu.Lock()
		if cur, still := c.items[key]; still && time.Now().After(cur.ExpiresAt) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		return domain.ErrCacheMiss{}
	}

	return json.Unmarshal(item.Value, dest)
}

// Set stores a value in the cache.
func (c *MemoryCache) Set(key string, value any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = defaultMemoryTTL // same contract as RedisCache: 0 = default TTL
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.sets++
	if c.sets >= sweepEvery || (c.max > 0 && len(c.items) >= c.max) {
		c.sweepExpired()
	}
	if _, exists := c.items[key]; !exists && c.max > 0 && len(c.items) >= c.max {
		c.evictOldest()
	}

	c.items[key] = cacheItem{
		Value:     data,
		ExpiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Delete removes a value from the cache.
func (c *MemoryCache) Delete(key string) error {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
	return nil
}

// Invalidate removes values that match the pattern.
func (c *MemoryCache) Invalidate(pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.items {
		if matchPattern(key, pattern) {
			delete(c.items, key)
		}
	}
	return nil
}

// Clear removes all values from the cache.
func (c *MemoryCache) Clear() error {
	c.mu.Lock()
	c.items = make(map[string]cacheItem)
	c.mu.Unlock()
	return nil
}

// sweepExpired drops expired entries (caller holds c.mu).
func (c *MemoryCache) sweepExpired() {
	c.sets = 0
	now := time.Now()
	for k, it := range c.items {
		if now.After(it.ExpiresAt) {
			delete(c.items, k)
		}
	}
}

func (c *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

func matchPattern(key, pattern string) bool {
	if pattern == "" {
		return false
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		return len(key) >= len(pattern)-1 && key[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	return key == pattern
}
