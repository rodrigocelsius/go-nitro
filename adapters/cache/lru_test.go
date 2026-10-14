package cache

import (
	"testing"
	"time"
)

func TestLRUCache_BasicSetGet(t *testing.T) {
	c := NewLRUCache(10)

	err := c.Set("key1", "value1", time.Minute)
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	var result string
	err = c.Get("key1", &result)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if result != "value1" {
		t.Errorf("result = %q, want value1", result)
	}
}

func TestLRUCache_Miss(t *testing.T) {
	c := NewLRUCache(10)

	var result string
	err := c.Get("nonexistent", &result)
	if err == nil {
		t.Error("expected error for cache miss")
	}

	stats := c.Stats()
	if stats.Misses != 1 {
		t.Errorf("misses = %d, want 1", stats.Misses)
	}
}

func TestLRUCache_Hit(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("key1", "value1", time.Minute)

	var result string
	c.Get("key1", &result)

	stats := c.Stats()
	if stats.Hits != 1 {
		t.Errorf("hits = %d, want 1", stats.Hits)
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	c := NewLRUCache(3) // Only 3 items

	// Fill cache
	c.Set("a", "1", time.Minute)
	c.Set("b", "2", time.Minute)
	c.Set("c", "3", time.Minute)

	// Add one more - should evict "a" (least recently used)
	c.Set("d", "4", time.Minute)

	var result string
	err := c.Get("a", &result)
	if err == nil {
		t.Error("expected 'a' to be evicted")
	}

	// "b" should still be there
	err = c.Get("b", &result)
	if err != nil {
		t.Errorf("expected 'b' to still be in cache: %v", err)
	}
}

func TestLRUCache_LRUPolicy(t *testing.T) {
	c := NewLRUCache(3)

	c.Set("a", "1", time.Minute)
	c.Set("b", "2", time.Minute)
	c.Set("c", "3", time.Minute)

	// Access "a" to make it recent
	c.Get("a", nil)

	// Add "d" - should evict "b" (oldest not accessed)
	c.Set("d", "4", time.Minute)

	var result string
	err := c.Get("a", &result)
	if err != nil {
		t.Error("'a' should still be in cache after access")
	}

	err = c.Get("b", &result)
	if err == nil {
		t.Error("'b' should have been evicted")
	}
}

func TestLRUCache_Expiration(t *testing.T) {
	c := NewLRUCache(10)

	c.Set("key1", "value1", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	var result string
	err := c.Get("key1", &result)
	if err == nil {
		t.Error("expected error for expired key")
	}
}

func TestLRUCache_Delete(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("key1", "value1", time.Minute)

	err := c.Delete("key1")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	var result string
	err = c.Get("key1", &result)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestLRUCache_Clear(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("a", "1", time.Minute)
	c.Set("b", "2", time.Minute)

	err := c.Clear()
	if err != nil {
		t.Fatalf("Clear error: %v", err)
	}

	stats := c.Stats()
	if stats.Size != 0 {
		t.Errorf("size = %d after clear, want 0", stats.Size)
	}
}

func TestLRUCache_InvalidatePattern(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("user:1", "data", time.Minute)
	c.Set("user:2", "data", time.Minute)
	c.Set("post:1", "data", time.Minute)

	c.Invalidate("user:*")

	var result string
	err := c.Get("user:1", &result)
	if err == nil {
		t.Error("expected 'user:1' to be invalidated")
	}

	err = c.Get("post:1", &result)
	if err != nil {
		t.Error("'post:1' should still exist")
	}
}

func TestLRUCache_Stats(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("a", "1", time.Minute)
	c.Get("a", nil)
	c.Get("b", nil)

	stats := c.Stats()
	if stats.Hits != 1 {
		t.Errorf("hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("misses = %d, want 1", stats.Misses)
	}
	if stats.Size != 1 {
		t.Errorf("size = %d, want 1", stats.Size)
	}
	if stats.MaxSize != 10 {
		t.Errorf("maxSize = %d, want 10", stats.MaxSize)
	}
}

func TestLRUCache_Update(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("a", "1", time.Minute)
	c.Set("a", "2", time.Minute)

	var result string
	c.Get("a", &result)
	if result != "2" {
		t.Errorf("result = %q, want 2 (updated)", result)
	}

	stats := c.Stats()
	if stats.Size != 1 {
		t.Errorf("size = %d, want 1 (no duplicate)", stats.Size)
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	c := NewLRUCache(1000)
	for i := 0; i < 1000; i++ {
		c.Set(string(rune(i)), i, time.Minute)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result int
		c.Get(string(rune(i%1000)), &result)
	}
}

func BenchmarkLRUCache_Set(b *testing.B) {
	c := NewLRUCache(1000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set("key", i, time.Minute)
	}
}

func BenchmarkLRUCache_Concurrent(b *testing.B) {
	c := NewLRUCache(1000)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				c.Set("key", i, time.Minute)
			} else {
				var result int
				c.Get("key", &result)
			}
			i++
		}
	})
}
