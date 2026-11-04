package cache

import (
	"strconv"
	"testing"
	"time"
)

func TestMemoryCache_FullCacheEvictsBatchAndStaysBounded(t *testing.T) {
	c := NewMemoryCache(64)
	for i := 0; i < 1000; i++ {
		_ = c.Set(strconv.Itoa(i), i, time.Duration(i+1)*time.Minute)
		if len(c.items) > 64 {
			t.Fatalf("cache grew past its cap: %d", len(c.items))
		}
	}
	var v int
	if err := c.Get("999", &v); err != nil || v != 999 {
		t.Fatalf("latest entry must be present: v=%d err=%v", v, err)
	}
	if err := c.Get("0", &v); err == nil {
		t.Fatal("soonest-to-expire entry must have been evicted")
	}
}
