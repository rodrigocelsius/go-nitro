package nats

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/rodrigocelsius/go-nitro/queue/v2"
)

func runServer(t *testing.T) string {
	t.Helper()
	opts := &server.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatal(err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}
	t.Cleanup(s.Shutdown)
	return s.ClientURL()
}

func open(t *testing.T, url string, mod func(*Config)) *Client {
	t.Helper()
	cfg := Config{URL: url, RetryDelay: 20 * time.Millisecond}
	if mod != nil {
		mod(&cfg)
	}
	c, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func wait(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for %s", what)
	}
}

func TestNATS_PublishSubscribeWithMetadata(t *testing.T) {
	c := open(t, runServer(t), nil)
	got := make(chan *queue.Message, 1)
	if err := c.Subscribe("emails", func(_ context.Context, m *queue.Message) error { got <- m; return nil }); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.PublishWithOptions(ctx, "emails", map[string]string{"to": "a@b.c"}, queue.WithTenantID("t1")); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-got:
		var body map[string]string
		_ = json.Unmarshal(m.Payload, &body)
		if body["to"] != "a@b.c" || m.Metadata["tenant_id"] != "t1" || m.Queue != "emails" || m.ID == "" {
			t.Fatalf("message = %+v meta=%v", m, m.Metadata)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not delivered")
	}
}

func TestNATS_RetryThenDeadLetter(t *testing.T) {
	url := runServer(t)
	c := open(t, url, func(c *Config) { c.RetryMax = 3 })
	var calls atomic.Int32
	dead := make(chan struct{})
	_ = c.Subscribe("jobs", func(context.Context, *queue.Message) error {
		calls.Add(1)
		return errors.New("boom")
	})
	_ = c.Subscribe("jobs.dlq", func(_ context.Context, m *queue.Message) error {
		if m.Metadata["x-nitro-error"] == "" {
			t.Errorf("dlq message without error metadata: %v", m.Metadata)
		}
		close(dead)
		return nil
	})
	ctx := context.Background()
	_ = c.Start(ctx)
	if err := c.Publish(ctx, "jobs", 1); err != nil {
		t.Fatal(err)
	}
	wait(t, dead, "dead letter")
	if calls.Load() != 3 {
		t.Fatalf("handler calls = %d, want 3 (RetryMax)", calls.Load())
	}
	st, _ := c.Stats(ctx, "jobs")
	if st.Failed != 1 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestNATS_RetrySucceeds(t *testing.T) {
	c := open(t, runServer(t), nil)
	var calls atomic.Int32
	done := make(chan struct{})
	_ = c.Subscribe("flaky", func(_ context.Context, m *queue.Message) error {
		if calls.Add(1) == 1 {
			return errors.New("transient")
		}
		if m.Attempts != 1 {
			t.Errorf("Attempts = %d, want 1 on redelivery", m.Attempts)
		}
		close(done)
		return nil
	})
	_ = c.Start(context.Background())
	_ = c.Publish(context.Background(), "flaky", "x")
	wait(t, done, "redelivery")
}

func TestNATS_InstancesShareWork(t *testing.T) {
	url := runServer(t)
	a, b := open(t, url, nil), open(t, url, nil)
	var total atomic.Int32
	all := make(chan struct{})
	h := func(context.Context, *queue.Message) error {
		if total.Add(1) == 20 {
			close(all)
		}
		return nil
	}
	_ = a.Subscribe("orders", h)
	_ = b.Subscribe("orders", h)
	ctx := context.Background()
	_ = a.Start(ctx)
	_ = b.Start(ctx)
	for i := 0; i < 20; i++ {
		if err := a.Publish(ctx, "orders", i); err != nil {
			t.Fatal(err)
		}
	}
	wait(t, all, "20 messages")
	time.Sleep(200 * time.Millisecond)
	if total.Load() != 20 {
		t.Fatalf("processed %d, want exactly 20 (each message once)", total.Load())
	}
}

func TestNATS_OpenFailsFastWhenDown(t *testing.T) {
	start := time.Now()
	if _, err := Open(Config{URL: "nats://127.0.0.1:1"}); err == nil {
		t.Fatal("expected connect error")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("Open must fail fast")
	}
}

func TestNames(t *testing.T) {
	if streamName("orders.created") != "NITRO_ORDERS_CREATED" || consumerName("a.b-c") != "A_B-C" {
		t.Fatal(streamName("orders.created"), consumerName("a.b-c"))
	}
	t.Setenv("NATS_URL", "nats://x:4222")
	t.Setenv("NATS_RETRY_MAX", "7")
	if c := ConfigFromEnv(); c.URL != "nats://x:4222" || c.RetryMax != 7 {
		t.Fatalf("ConfigFromEnv = %+v", c)
	}
}
