package amqp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/queue/v2"
)

func TestSafeHandler_ConvertsPanicToError(t *testing.T) {
	h := func(ctx context.Context, m *queue.Message) error {
		panic("boom")
	}
	msg := &queue.Message{ID: "m1", Queue: "q1"}
	err := safeHandler(context.Background(), msg, h)
	if err == nil {
		t.Fatal("expected error from panicking handler")
	}
	if !strings.Contains(err.Error(), "handler panic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSafeHandler_PassesThrough(t *testing.T) {
	h := func(ctx context.Context, m *queue.Message) error { return nil }
	msg := &queue.Message{ID: "m1", Queue: "q1"}
	if err := safeHandler(context.Background(), msg, h); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNextBackoff_GrowsAndCaps(t *testing.T) {
	base := 100 * time.Millisecond
	max := time.Second
	cur := base
	for i := 0; i < 10; i++ {
		next := nextBackoff(cur, max)
		if next <= 0 || next > max {
			t.Fatalf("backoff out of bounds: %v", next)
		}
		// With ±20% jitter, expected growth is ~2x minus jitter tolerance.
		if i < 3 && next < base {
			t.Fatalf("backoff shrank too early: %v", next)
		}
		cur = next
	}
	// At the cap the ±20% jitter is still applied, so the value lands in
	// [80%, 100%] of max rather than exactly on it.
	if cur > max || cur < max*8/10 {
		t.Fatalf("expected backoff near cap %v (within 20%%), got %v", max, cur)
	}
}

func TestNextBackoff_RespectsMax(t *testing.T) {
	for i := 0; i < 50; i++ {
		got := nextBackoff(time.Hour, 30*time.Second)
		// Jitter keeps the result within [80%, 100%] of the cap.
		if got > 30*time.Second || got < 24*time.Second {
			t.Fatalf("expected backoff within [24s,30s], got %v", got)
		}
	}
}

func TestSleepCtx_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepCtx(ctx, time.Hour) {
		t.Fatal("expected sleepCtx to return false when ctx cancelled")
	}
}

func TestSleepCtx_Elapses(t *testing.T) {
	if !sleepCtx(context.Background(), time.Millisecond) {
		t.Fatal("expected sleepCtx to return true after elapsing")
	}
}
