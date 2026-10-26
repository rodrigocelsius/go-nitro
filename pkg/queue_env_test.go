package nitro

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestQueueFromEnv_InMemoryWithoutRabbit(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "")
	q := QueueFromEnv()
	if q.Primary() != nil {
		t.Fatal("without RABBITMQ_URL the queue must be in-memory")
	}
	done := make(chan struct{})
	_ = q.Subscribe("jobs", func(context.Context, *QueueMessage) error { close(done); return nil })
	_ = q.Start(context.Background())
	defer q.Stop()
	if err := q.Publish(context.Background(), "jobs", 1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("not delivered")
	}
}

func TestQueueFromEnv_UnreachableRabbitFallsBack(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:1/") // nothing listens on port 1
	q := QueueFromEnv()
	if q.Primary() == nil {
		t.Fatal("with RABBITMQ_URL the primary must be the broker")
	}
	got := make(chan struct{}, 1)
	_ = q.Subscribe("orders", func(context.Context, *QueueMessage) error { got <- struct{}{}; return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = q.Start(ctx)
	defer q.Stop()

	if err := q.Publish(ctx, "orders", "o-1"); err != nil {
		t.Fatalf("publish with broker down must fall back: %v", err)
	}
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("fallback did not deliver locally")
	}
	// No local subscriber: buffered for the broker.
	if err := q.Publish(ctx, "audit", 1); err != nil || q.Buffered() != 1 {
		t.Fatalf("err=%v buffered=%d", err, q.Buffered())
	}
}

func TestLazyBroker_ThrottlesRedial(t *testing.T) {
	calls := 0
	l := &lazyBroker{open: func() (Queue, error) { calls++; return nil, errors.New("down") }}
	_, _ = l.get()
	_, err := l.get()
	if calls != 1 || err == nil {
		t.Fatalf("calls=%d err=%v: redial must be throttled", calls, err)
	}
}

func TestSelectBroker(t *testing.T) {
	for _, k := range []string{"RABBITMQ_URL", "NATS_URL", "KAFKA_BROKERS", "NITRO_QUEUE"} {
		t.Setenv(k, "")
	}
	if selectBroker() != nil {
		t.Fatal("no env → memory")
	}
	t.Setenv("KAFKA_BROKERS", "k:9092")
	t.Setenv("NATS_URL", "nats://n:4222")
	if b := selectBroker(); b == nil || b.name != "nats" {
		t.Fatalf("NATS has priority over Kafka, got %v", b)
	}
	t.Setenv("NITRO_QUEUE", "kafka")
	if b := selectBroker(); b == nil || b.name != "kafka" {
		t.Fatalf("NITRO_QUEUE must win, got %v", b)
	}
	t.Setenv("NITRO_QUEUE", "memory")
	if selectBroker() != nil {
		t.Fatal("NITRO_QUEUE=memory")
	}
}

func TestQueueFromEnv_NATSDownFallsBack(t *testing.T) {
	for _, k := range []string{"RABBITMQ_URL", "KAFKA_BROKERS", "NITRO_QUEUE"} {
		t.Setenv(k, "")
	}
	t.Setenv("NATS_URL", "nats://127.0.0.1:1")
	q := QueueFromEnv()
	got := make(chan struct{}, 1)
	_ = q.Subscribe("jobs", func(context.Context, *QueueMessage) error { got <- struct{}{}; return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = q.Start(ctx)
	defer q.Stop()
	if err := q.Publish(ctx, "jobs", 1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("fallback did not deliver")
	}
}
