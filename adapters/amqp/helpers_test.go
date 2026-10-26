package amqp

import (
	"context"
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rodrigocelsius/go-nitro/queue/v2"
)

func TestNextBackoff_BoundedAndGrowing(t *testing.T) {
	max := 10 * time.Second
	for i := 0; i < 200; i++ {
		d := nextBackoff(time.Second, max)
		if d <= 0 || d > max {
			t.Fatalf("backoff %v out of (0, %v]", d, max)
		}
		if d < 1600*time.Millisecond || d > 2400*time.Millisecond {
			t.Fatalf("backoff %v not ~2x±20%% of 1s", d)
		}
	}
	if d := nextBackoff(8*time.Second, max); d > max {
		t.Fatalf("cap exceeded: %v", d)
	}
}

func TestDeliveryToMessage(t *testing.T) {
	ts := time.Now().Add(-time.Minute)
	d := amqp.Delivery{
		MessageId: "broker-id",
		Body:      []byte(`{"a":1}`),
		Timestamp: ts,
		Headers: amqp.Table{
			headerAttempts: int32(2),
			headerMaxRetry: "5",
			headerMsgID:    "nitro-id",
			"tenant_id":    "t1",
		},
	}
	m := deliveryToMessage("orders", d)
	if m.ID != "nitro-id" || m.Queue != "orders" || m.Attempts != 2 || m.MaxRetry != 5 ||
		m.Metadata["tenant_id"] != "t1" || string(m.Payload) != `{"a":1}` || !m.CreatedAt.Equal(ts) {
		t.Fatalf("message = %+v", m)
	}
	if _, ok := m.Metadata[headerAttempts]; ok {
		t.Fatal("internal headers must not leak into metadata")
	}
	if deliveryToMessage("q", amqp.Delivery{}).ID == "" {
		t.Fatal("missing id must be generated")
	}
}

func TestHelpers(t *testing.T) {
	for v, want := range map[any]int{3: 3, int32(4): 4, int64(5): 5, 6.9: 6, "7": 7, "x": 0, true: 0} {
		if got := asInt(v); got != want {
			t.Errorf("asInt(%v) = %d, want %d", v, got, want)
		}
	}
	if cloneMeta(nil) != nil {
		t.Fatal("cloneMeta(nil)")
	}
	src := map[string]string{"a": "1"}
	cp := cloneMeta(src)
	cp["a"] = "2"
	if src["a"] != "1" {
		t.Fatal("cloneMeta must copy")
	}
	o := queue.ApplyPublishOptions(withAttempt(3))
	if o.Metadata[headerAttempts] != "3" {
		t.Fatalf("withAttempt: %v", o.Metadata)
	}
}

func TestClosedClientPublishFails(t *testing.T) {
	c := &Client{cfg: DefaultConfig(), stats: map[string]*queue.Stats{}}
	err := c.Publish(context.Background(), "k", map[string]int{"a": 1})
	if !errors.Is(err, errClosed) {
		t.Fatalf("err = %v, want errClosed", err)
	}
	if c.publisherDead() {
		t.Fatal("a closed client must not try to reconnect")
	}
	if err := c.Publish(context.Background(), "k", make(chan int)); err == nil {
		t.Fatal("unmarshalable payload must fail")
	}
}
