package nitro

import (
	"context"
	"os"
	"testing"
	"time"
)

// End to end through the convention, against real brokers:
//
//	NITRO_QUEUE_E2E_NATS=nats://localhost:4222 NITRO_QUEUE_E2E_KAFKA=localhost:9092 go test -run E2E
func TestQueueFromEnv_BrokersE2E(t *testing.T) {
	cases := map[string][2]string{
		"nats":  {"NATS_URL", os.Getenv("NITRO_QUEUE_E2E_NATS")},
		"kafka": {"KAFKA_BROKERS", os.Getenv("NITRO_QUEUE_E2E_KAFKA")},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if c[1] == "" {
				t.Skip("broker address not set")
			}
			for _, k := range []string{"RABBITMQ_URL", "NATS_URL", "KAFKA_BROKERS", "NITRO_QUEUE"} {
				t.Setenv(k, "")
			}
			t.Setenv(c[0], c[1])
			q := QueueFromEnv()
			topic := "e2e-" + NewUUIDString()[:8]
			got := make(chan string, 1)
			_ = q.Subscribe(topic, func(_ context.Context, m *QueueMessage) error {
				got <- string(m.Payload)
				return nil
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := q.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer q.Stop()
			if err := q.Publish(ctx, topic, "hello"); err != nil {
				t.Fatal(err)
			}
			select {
			case p := <-got:
				if p != `"hello"` {
					t.Fatalf("payload %s", p)
				}
			case <-time.After(30 * time.Second):
				t.Fatal("not delivered")
			}
			if q.Buffered() != 0 {
				t.Fatal("broker up: nothing should be buffered")
			}
		})
	}
}
