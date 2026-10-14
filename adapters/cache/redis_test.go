package cache

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func newTestRedis(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisCache(client, WithDefaultTTL(time.Minute)), mr
}

func TestRedisCache_SetGetDelete(t *testing.T) {
	c, _ := newTestRedis(t)

	type payload struct {
		Name string `json:"name"`
	}
	if err := c.Set("driver:1", payload{Name: "Ana"}, 0); err != nil {
		t.Fatal(err)
	}

	var got payload
	if err := c.Get("driver:1", &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ana" {
		t.Fatalf("got %#v", got)
	}

	if err := c.Delete("driver:1"); err != nil {
		t.Fatal(err)
	}
	if err := c.Get("driver:1", &got); err == nil {
		t.Fatal("expected miss")
	} else if _, ok := err.(domain.ErrCacheMiss); !ok {
		t.Fatalf("want ErrCacheMiss, got %T %v", err, err)
	}
}

func TestRedisCache_InvalidateAndClear(t *testing.T) {
	c, _ := newTestRedis(t)

	_ = c.Set("users:list:0", []string{"a"}, time.Minute)
	_ = c.Set("users:list:1", []string{"b"}, time.Minute)
	_ = c.Set("events:all", []string{"e"}, time.Minute)

	if err := c.Invalidate("users:list:*"); err != nil {
		t.Fatal(err)
	}

	var dest []string
	if err := c.Get("users:list:0", &dest); err == nil {
		t.Fatal("users:list:0 should be gone")
	}
	if err := c.Get("events:all", &dest); err != nil {
		t.Fatalf("events:all should remain: %v", err)
	}

	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := c.Get("events:all", &dest); err == nil {
		t.Fatal("clear should remove events:all")
	}
}

func TestRedisCache_TTL(t *testing.T) {
	c, mr := newTestRedis(t)
	if err := c.Set("temp", "x", 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(100 * time.Millisecond)

	var s string
	if err := c.Get("temp", &s); err == nil {
		t.Fatal("expected miss after TTL")
	}
}
