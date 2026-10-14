package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemory_SetGet(t *testing.T) {
	c := NewMemory(Config{
		MaxSize: 100,
		TTL:     5 * time.Minute,
	})

	ctx := context.Background()

	// Set
	if err := c.Set(ctx, "key1", "value1", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	var result string
	if err := c.Get(ctx, "key1", &result); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if result != "value1" {
		t.Errorf("expected 'value1', got %q", result)
	}
}

func TestMemory_CacheMiss(t *testing.T) {
	c := NewMemory(Config{
		MaxSize: 100,
		TTL:     5 * time.Minute,
	})

	ctx := context.Background()

	var result string
	err := c.Get(ctx, "nonexistent", &result)
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss, got %v", err)
	}
}

func TestMemory_Expiration(t *testing.T) {
	c := NewMemory(Config{
		MaxSize: 100,
		TTL:     100 * time.Millisecond,
	})

	ctx := context.Background()

	if err := c.Set(ctx, "key1", "value1", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Espera expirar
	time.Sleep(150 * time.Millisecond)

	var result string
	err := c.Get(ctx, "key1", &result)
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after expiration, got %v", err)
	}
}

func TestMemory_Delete(t *testing.T) {
	c := NewMemory(Config{
		MaxSize: 100,
		TTL:     5 * time.Minute,
	})

	ctx := context.Background()

	if err := c.Set(ctx, "key1", "value1", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if err := c.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	var result string
	err := c.Get(ctx, "key1", &result)
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestMemory_Exists(t *testing.T) {
	c := NewMemory(Config{
		MaxSize: 100,
		TTL:     5 * time.Minute,
	})

	ctx := context.Background()

	exists, err := c.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected key1 to not exist")
	}

	if err := c.Set(ctx, "key1", "value1", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	exists, err = c.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected key1 to exist")
	}
}

func TestMemory_LRUEviction(t *testing.T) {
	c := NewMemory(Config{
		MaxSize: 3,
		TTL:     5 * time.Minute,
	})

	ctx := context.Background()

	// Fill the cache
	c.Set(ctx, "key1", "value1", 0)
	c.Set(ctx, "key2", "value2", 0)
	c.Set(ctx, "key3", "value3", 0)

	// Access key1 to update LRU
	c.Get(ctx, "key1", nil)

	// Add key4; should evict key2 (oldest not accessed)
	c.Set(ctx, "key4", "value4", 0)

	// key1 should exist (accessed recently)
	exists, _ := c.Exists(ctx, "key1")
	if !exists {
		t.Error("expected key1 to exist")
	}

	// key2 should have been evicted
	exists, _ = c.Exists(ctx, "key2")
	if exists {
		t.Error("expected key2 to be evicted")
	}
}

func TestMemory_Stats(t *testing.T) {
	c := NewMemory(Config{
		MaxSize: 100,
		TTL:     5 * time.Minute,
	})

	ctx := context.Background()

	c.Set(ctx, "key1", "value1", 0)
	c.Get(ctx, "key1", nil)
	c.Get(ctx, "nonexistent", nil)

	stats, err := c.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.Sets != 1 {
		t.Errorf("expected 1 set, got %d", stats.Sets)
	}

	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}

	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestMemory_Clear(t *testing.T) {
	c := NewMemory(Config{
		MaxSize: 100,
		TTL:     5 * time.Minute,
	})

	ctx := context.Background()

	c.Set(ctx, "key1", "value1", 0)
	c.Set(ctx, "key2", "value2", 0)

	if err := c.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	exists, _ := c.Exists(ctx, "key1")
	if exists {
		t.Error("expected key1 to not exist after clear")
	}

	exists, _ = c.Exists(ctx, "key2")
	if exists {
		t.Error("expected key2 to not exist after clear")
	}
}

func TestStats_HitRate(t *testing.T) {
	stats := &Stats{
		Hits:   7,
		Misses: 3,
	}

	rate := stats.HitRate()
	if rate != 0.7 {
		t.Errorf("expected hit rate 0.7, got %f", rate)
	}
}

func TestStats_HitRateEmpty(t *testing.T) {
	stats := &Stats{}

	rate := stats.HitRate()
	if rate != 0 {
		t.Errorf("expected hit rate 0 for empty stats, got %f", rate)
	}
}
