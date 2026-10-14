package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestInternal_PublishSubscribe(t *testing.T) {
	q := NewInternal(Config{
		Workers:    2,
		QueueSize:  10,
		RetryMax:   3,
		RetryDelay: 10 * time.Millisecond,
	})

	var received int32
	handler := func(ctx context.Context, msg *Message) error {
		atomic.AddInt32(&received, 1)
		return nil
	}

	if err := q.Subscribe("test", handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Publish 5 messages
	for i := 0; i < 5; i++ {
		if err := q.Publish(ctx, "test", map[string]string{"index": string(rune('A' + i))}); err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	if err := q.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if atomic.LoadInt32(&received) != 5 {
		t.Errorf("expected 5 messages received, got %d", atomic.LoadInt32(&received))
	}
}

func TestInternal_RetryOnFailure(t *testing.T) {
	q := NewInternal(Config{
		Workers:    1,
		QueueSize:  10,
		RetryMax:   3,
		RetryDelay: 10 * time.Millisecond,
	})

	var attempts int32
	handler := func(ctx context.Context, msg *Message) error {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			return context.DeadlineExceeded // Simula erro
		}
		return nil
	}

	if err := q.Subscribe("retry-test", handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := q.Publish(ctx, "retry-test", "test-payload"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for retries
	time.Sleep(200 * time.Millisecond)

	if err := q.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestInternal_Stats(t *testing.T) {
	q := NewInternal(Config{
		Workers:    1,
		QueueSize:  10,
		RetryMax:   3,
		RetryDelay: 10 * time.Millisecond,
	})

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	if err := q.Subscribe("stats-test", handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Publish 3 messages
	for i := 0; i < 3; i++ {
		if err := q.Publish(ctx, "stats-test", "test"); err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	stats, err := q.Stats(ctx, "stats-test")
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.Total != 3 {
		t.Errorf("expected total 3, got %d", stats.Total)
	}

	if stats.Completed != 3 {
		t.Errorf("expected completed 3, got %d", stats.Completed)
	}

	if err := q.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestInternal_MessageStates(t *testing.T) {
	q := NewInternal(Config{
		Workers:    1,
		QueueSize:  10,
		RetryMax:   3,
		RetryDelay: 10 * time.Millisecond,
	})

	var lastState atomic.Value // State; handler roda em worker goroutine
	handler := func(ctx context.Context, msg *Message) error {
		lastState.Store(msg.State)
		return nil
	}

	if err := q.Subscribe("state-test", handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := q.Publish(ctx, "state-test", "test"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	if got, _ := lastState.Load().(State); got != StateProcessing {
		t.Errorf("expected state Processing, got %v", got)
	}

	if err := q.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestInternal_RecoverFromPanic(t *testing.T) {
	q := NewInternal(Config{
		Workers:    1,
		QueueSize:  10,
		RetryMax:   3,
		RetryDelay: 10 * time.Millisecond,
	})

	var attempts int32
	handler := func(ctx context.Context, msg *Message) error {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			panic("boom") // handler com bug não pode derrubar o processo
		}
		return nil
	}

	if err := q.Subscribe("panic-test", handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := q.Publish(ctx, "panic-test", "test-payload"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := q.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected panic to follow retry path (3 attempts), got %d", atomic.LoadInt32(&attempts))
	}
}
