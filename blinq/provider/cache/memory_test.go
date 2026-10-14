package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/cache"
)

func TestMemoryInvalidateTag(t *testing.T) {
	ctx := context.Background()
	m := cache.New()
	m.Set(ctx, "users|k1", []map[string]any{{"a": 1}}, time.Minute)
	m.Set(ctx, "users|k2", []map[string]any{{"a": 2}}, time.Minute)
	m.Set(ctx, "other|k3", []map[string]any{{"a": 3}}, time.Minute)
	m.InvalidateTag(ctx, "users")
	if _, ok := m.Get(ctx, "users|k1"); ok {
		t.Fatal("expected miss")
	}
	if _, ok := m.Get(ctx, "other|k3"); !ok {
		t.Fatal("other tag should remain")
	}
}
