package ratelimit

import (
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

func TestSlidingWindowAllow(t *testing.T) {
	sw := usecases.NewSlidingWindow(5, time.Second)

	for i := 0; i < 5; i++ {
		if !sw.Allow() {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	if sw.Allow() {
		t.Fatal("6th request should be denied")
	}
}

func TestSlidingWindowRemaining(t *testing.T) {
	sw := usecases.NewSlidingWindow(3, time.Second)

	sw.Allow()
	sw.Allow()

	remaining := sw.Remaining()
	if remaining != 1 {
		t.Fatalf("expected 1 remaining, got %d", remaining)
	}
}

func TestSlidingWindowReset(t *testing.T) {
	sw := usecases.NewSlidingWindow(2, time.Second)

	sw.Allow()
	sw.Allow()

	if sw.Allow() {
		t.Fatal("should be denied after max")
	}

	sw.Reset()

	if !sw.Allow() {
		t.Fatal("should be allowed after reset")
	}
}

func TestSlidingWindowExpiry(t *testing.T) {
	sw := usecases.NewSlidingWindow(2, 50*time.Millisecond)

	sw.Allow()
	sw.Allow()

	if sw.Allow() {
		t.Fatal("should be denied")
	}

	time.Sleep(100 * time.Millisecond)

	if !sw.Allow() {
		t.Fatal("should be allowed after window expires")
	}
}

func TestMemoryLimiterSlidingWindow(t *testing.T) {
	limiter := NewSlidingWindowLimiter(3, time.Second)

	limiter.Allow("a")
	limiter.Allow("a")
	limiter.Allow("a")

	if limiter.Allow("a") {
		t.Fatal("should be denied")
	}

	if !limiter.Allow("b") {
		t.Fatal("different key should be allowed")
	}
}

func BenchmarkSlidingWindow(b *testing.B) {
	sw := usecases.NewSlidingWindow(1000, time.Minute)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sw.Allow()
	}
}

func BenchmarkTokenBucket(b *testing.B) {
	tb := usecases.NewTokenBucket(1000, time.Minute)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Allow()
	}
}
