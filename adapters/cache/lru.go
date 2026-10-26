package cache

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type lruNode struct {
	key       string
	Value     []byte
	ExpiresAt time.Time
	prev      *lruNode
	next      *lruNode
}

// LRUCache implements domain.Cache with real LRU eviction and statistics.
type LRUCache struct {
	mu      sync.RWMutex
	items   map[string]*lruNode
	head    *lruNode // most recent
	tail    *lruNode // least recent (eviction candidate)
	maxSize int
	size    int

	// Statistics
	hits   uint64
	misses uint64
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Hits    uint64
	Misses  uint64
	Size    int
	MaxSize int
}

// Stats returns current cache statistics.
func (c *LRUCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Hits:    c.hits,
		Misses:  c.misses,
		Size:    c.size,
		MaxSize: c.maxSize,
	}
}

// NewLRUCache creates a new LRUCache.
func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		items:   make(map[string]*lruNode),
		maxSize: maxSize,
	}
}

// Get retrieves a value from the cache.
func (c *LRUCache) Get(key string, dest any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.items[key]
	if !ok || time.Now().After(node.ExpiresAt) {
		c.misses++
		if ok {
			c.removeNode(node)
			delete(c.items, key)
		}
		return domain.ErrCacheMiss{}
	}

	// Move to head (most recent)
	c.moveToHead(node)
	c.hits++

	return json.Unmarshal(node.Value, dest)
}

// Set stores a value in the cache.
func (c *LRUCache) Set(key string, value any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = defaultMemoryTTL
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// If it already exists, update and move to head
	if node, ok := c.items[key]; ok {
		node.Value = data
		node.ExpiresAt = time.Now().Add(ttl)
		c.moveToHead(node)
		return nil
	}

	// Create a new node
	node := &lruNode{
		key:       key,
		Value:     data,
		ExpiresAt: time.Now().Add(ttl),
	}

	// If full, remove the tail (LRU)
	if c.maxSize > 0 && c.size >= c.maxSize {
		c.removeOldest()
	}

	// Add to head
	c.addToHead(node)
	c.items[key] = node
	return nil
}

// Delete removes a value from the cache.
func (c *LRUCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.items[key]; ok {
		c.removeNode(node)
		delete(c.items, key)
	}
	return nil
}

// Invalidate removes values that match the pattern.
func (c *LRUCache) Invalidate(pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, node := range c.items {
		if matchPattern(key, pattern) {
			c.removeNode(node)
			delete(c.items, key)
		}
	}
	return nil
}

// Clear removes all values from the cache.
func (c *LRUCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*lruNode)
	c.head = nil
	c.tail = nil
	c.size = 0
	return nil
}

// addToHead inserts a node at the start of the list.
func (c *LRUCache) addToHead(node *lruNode) {
	node.prev = nil
	node.next = c.head

	if c.head != nil {
		c.head.prev = node
	}
	c.head = node

	if c.tail == nil {
		c.tail = node
	}
	c.size++
}

// removeNode removes a node from the list.
func (c *LRUCache) removeNode(node *lruNode) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.head = node.next
	}

	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.tail = node.prev
	}

	node.prev = nil
	node.next = nil
	c.size--
}

// moveToHead moves an existing node to the start.
func (c *LRUCache) moveToHead(node *lruNode) {
	if c.head == node {
		return
	}
	c.removeNode(node)
	c.addToHead(node)
}

// removeOldest removes the least recently used node (tail).
func (c *LRUCache) removeOldest() {
	if c.tail == nil {
		return
	}
	node := c.tail
	c.removeNode(node)
	delete(c.items, node.key)
}
