package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// flakyBroker is a primary that can be taken down and brought back.
type flakyBroker struct {
	down      atomic.Bool
	mu        sync.Mutex
	published []string
}

func (b *flakyBroker) Publish(_ context.Context, queue string, _ interface{}) error {
	if b.down.Load() {
		return errors.New("dial tcp: connection refused")
	}
	b.mu.Lock()
	b.published = append(b.published, queue)
	b.mu.Unlock()
	return nil
}
func (b *flakyBroker) Subscribe(string, Handler) error               { return nil }
func (b *flakyBroker) Start(context.Context) error                   { return nil }
func (b *flakyBroker) Stop() error                                   { return nil }
func (b *flakyBroker) Stats(context.Context, string) (*Stats, error) { return &Stats{}, nil }

func (b *flakyBroker) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.published)
}

func TestFallback_BuffersAndFlushesWhenPrimaryReturns(t *testing.T) {
	b := &flakyBroker{}
	f := NewFallback(b, FallbackConfig{RetryEvery: time.Hour})
	ctx := context.Background()
	_ = f.Start(ctx)
	defer f.Stop()

	if err := f.Publish(ctx, "emails", map[string]string{"to": "a"}); err != nil || b.count() != 1 {
		t.Fatalf("healthy publish: err=%v count=%d", err, b.count())
	}

	b.down.Store(true)
	for i := 0; i < 3; i++ {
		if err := f.Publish(ctx, "emails", i); err != nil {
			t.Fatalf("publish while down must not fail: %v", err)
		}
	}
	if f.Buffered() != 3 {
		t.Fatalf("buffered = %d, want 3", f.Buffered())
	}
	if f.Flush(ctx) != 3 {
		t.Fatal("flush while down must keep the buffer")
	}

	b.down.Store(false)
	if left := f.Flush(ctx); left != 0 || b.count() != 4 {
		t.Fatalf("after recovery: left=%d published=%d", left, b.count())
	}
}

func TestFallback_DeliversLocallyWhenSubscribed(t *testing.T) {
	b := &flakyBroker{}
	b.down.Store(true)
	f := NewFallback(b, FallbackConfig{RetryEvery: time.Hour})
	got := make(chan string, 1)
	_ = f.Subscribe("orders", func(_ context.Context, m *Message) error {
		got <- string(m.Payload)
		return nil
	})
	ctx := context.Background()
	_ = f.Start(ctx)
	defer f.Stop()

	if err := f.Publish(ctx, "orders", "o-1"); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-got:
		if p != `"o-1"` {
			t.Fatalf("payload %s", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("local subscriber did not receive the fallback message")
	}
	if f.Buffered() != 0 {
		t.Fatal("subscribed queue must not be buffered")
	}
}

func TestFallback_BufferBound(t *testing.T) {
	b := &flakyBroker{}
	b.down.Store(true)
	f := NewFallback(b, FallbackConfig{BufferSize: 1, RetryEvery: time.Hour})
	ctx := context.Background()
	_ = f.Publish(ctx, "x", 1)
	if err := f.Publish(ctx, "x", 2); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("err = %v, want ErrQueueFull", err)
	}
	st, _ := f.Stats(ctx, "x")
	if st.Pending != 1 {
		t.Fatalf("stats pending = %d", st.Pending)
	}
}

func TestFallback_NilPrimaryIsInMemory(t *testing.T) {
	f := NewFallback(nil, FallbackConfig{})
	done := make(chan struct{})
	_ = f.Subscribe("jobs", func(context.Context, *Message) error { close(done); return nil })
	_ = f.Start(context.Background())
	defer f.Stop()
	_ = f.Publish(context.Background(), "jobs", 1)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("in-memory delivery failed")
	}
}

func TestInternal_BackpressureInsteadOfImmediateFailure(t *testing.T) {
	q := NewInternal(Config{Workers: 1, QueueSize: 1, RetryMax: 1, RetryDelay: time.Millisecond, PublishTimeout: 2 * time.Second})
	release := make(chan struct{})
	var handled atomic.Int32
	_ = q.Subscribe("slow", func(context.Context, *Message) error {
		<-release
		handled.Add(1)
		return nil
	})
	_ = q.Start(context.Background())
	defer q.Stop()
	ctx := context.Background()
	_ = q.Publish(ctx, "slow", 1) // taken by the worker (blocked)
	time.Sleep(20 * time.Millisecond)
	_ = q.Publish(ctx, "slow", 2) // fills the buffer

	errCh := make(chan error, 1)
	go func() { errCh <- q.Publish(ctx, "slow", 3) }() // must wait, not fail
	time.Sleep(50 * time.Millisecond)
	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("publish under backpressure: %v", err)
	}

	full := NewInternal(Config{Workers: 1, QueueSize: 1, PublishTimeout: -1})
	_ = full.Publish(ctx, "q", 1)
	if err := full.Publish(ctx, "q", 2); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("err = %v, want ErrQueueFull", err)
	}
}
