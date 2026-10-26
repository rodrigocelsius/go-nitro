package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/rodrigocelsius/go-nitro/queue/v2"
)

// Integration tests need a broker: KAFKA_TEST_BROKERS=localhost:9092
// (`docker run -p 9092:9092 apache/kafka`). Unit tests always run.
func brokers(t *testing.T) []string {
	t.Helper()
	v := os.Getenv("KAFKA_TEST_BROKERS")
	if v == "" {
		t.Skip("set KAFKA_TEST_BROKERS to run Kafka integration tests")
	}
	return strings.Split(v, ",")
}

func uniq(name string) string { return name + "-" + queue.NewMessageID()[:8] }

func open(t *testing.T, mod func(*Config)) *Client {
	t.Helper()
	cfg := Config{Brokers: brokers(t), RetryDelay: 10 * time.Millisecond, Partitions: 1, GroupID: uniq("g")}
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
	case <-time.After(30 * time.Second):
		t.Fatalf("timeout waiting for %s", what)
	}
}

func TestKafka_PublishSubscribe(t *testing.T) {
	c := open(t, nil)
	topic := uniq("emails")
	got := make(chan *queue.Message, 1)
	_ = c.Subscribe(topic, func(_ context.Context, m *queue.Message) error { got <- m; return nil })
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.PublishWithOptions(ctx, topic, map[string]string{"to": "a@b.c"}, queue.WithTenantID("t1")); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-got:
		var body map[string]string
		_ = json.Unmarshal(m.Payload, &body)
		if body["to"] != "a@b.c" || m.Metadata["tenant_id"] != "t1" || m.ID == "" {
			t.Fatalf("message = %+v", m)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("not delivered")
	}
}

func TestKafka_RetryThenDeadLetter(t *testing.T) {
	c := open(t, func(c *Config) { c.RetryMax = 3 })
	topic := uniq("jobs")
	var calls atomic.Int32
	dead := make(chan struct{})
	_ = c.Subscribe(topic, func(context.Context, *queue.Message) error {
		calls.Add(1)
		return errors.New("boom")
	})
	_ = c.Subscribe(topic+".dlq", func(_ context.Context, m *queue.Message) error {
		if m.Metadata["x-nitro-error"] != "boom" {
			t.Errorf("dlq metadata = %v", m.Metadata)
		}
		close(dead)
		return nil
	})
	ctx := context.Background()
	_ = c.Start(ctx)
	if err := c.Publish(ctx, topic, 1); err != nil {
		t.Fatal(err)
	}
	wait(t, dead, "dead letter")
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

func TestOpen_FailsFastWhenDown(t *testing.T) {
	start := time.Now()
	if _, err := Open(Config{Brokers: []string{"127.0.0.1:1"}}); err == nil {
		t.Fatal("expected connect error")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("Open must fail fast")
	}
}

func TestConfigFromEnvAndMessage(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "a:9092, b:9092")
	t.Setenv("KAFKA_GROUP_ID", "svc")
	t.Setenv("KAFKA_RETRY_MAX", "5")
	cfg := ConfigFromEnv()
	if len(cfg.Brokers) != 2 || cfg.Brokers[1] != "b:9092" || cfg.GroupID != "svc" || cfg.RetryMax != 5 {
		t.Fatalf("cfg = %+v", cfg)
	}
	m := toMessage("q", kafkago.Message{Value: []byte(`1`), Headers: []kafkago.Header{
		{Key: headerMsgID, Value: []byte("id-1")}, {Key: headerMaxRetry, Value: []byte("4")}, {Key: "tenant_id", Value: []byte("t")},
	}})
	if m.ID != "id-1" || m.MaxRetry != 4 || m.Metadata["tenant_id"] != "t" || string(m.Payload) != "1" {
		t.Fatalf("message = %+v", m)
	}
}
