package ratelimit

import (
	"strconv"
	"testing"
	"time"
)

// With every bucket still live, the map used to grow without bound.
func TestMemoryLimiter_HardCapSharesOverflowBucket(t *testing.T) {
	m := NewMemoryLimiter(1, time.Hour)
	for i := 0; i < hardMaxBuckets; i++ {
		m.Allow(strconv.Itoa(i))
	}
	if !m.Allow("over-1") {
		t.Fatal("first overflow request should pass")
	}
	if m.Allow("over-2") {
		t.Fatal("overflow bucket is shared and fail-closed")
	}
	if len(m.buckets) > hardMaxBuckets {
		t.Fatalf("map grew past the hard cap: %d", len(m.buckets))
	}
}
