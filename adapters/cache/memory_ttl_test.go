package cache

import (
	"testing"
	"time"
)

func TestMemoryCache_ZeroTTLUsesDefault(t *testing.T) {
	for name, c := range map[string]interface {
		Set(string, any, time.Duration) error
		Get(string, any) error
	}{"memory": NewMemoryCache(10), "lru": NewLRUCache(10)} {
		if err := c.Set("k", "v", 0); err != nil {
			t.Fatal(err)
		}
		var got string
		if err := c.Get("k", &got); err != nil || got != "v" {
			t.Fatalf("%s: ttl 0 must use the default TTL, got %q err=%v", name, got, err)
		}
	}
}

func TestMemoryCache_SweepsExpiredWhenUnbounded(t *testing.T) {
	c := NewMemoryCache(0)
	for i := 0; i < sweepEvery; i++ {
		_ = c.Set(string(rune('a'+i%26))+time.Now().String(), i, time.Nanosecond)
	}
	time.Sleep(time.Millisecond)
	_ = c.Set("live", 1, time.Minute) // triggers the sweep
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	if n > 2 {
		t.Fatalf("expired entries not swept: %d items", n)
	}
}

func TestMemoryCache_BoundedOverwriteDoesNotEvict(t *testing.T) {
	c := NewMemoryCache(2)
	_ = c.Set("a", 1, time.Minute)
	_ = c.Set("b", 2, time.Minute)
	_ = c.Set("b", 3, time.Minute) // overwrite: "a" must stay
	var v int
	if err := c.Get("a", &v); err != nil {
		t.Fatal("overwriting an existing key must not evict another one")
	}
}
