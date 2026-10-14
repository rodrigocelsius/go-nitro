package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	q, err := nitro.NewRabbitQueueFromEnv()
	if err != nil {
		log.Printf("rabbitmq unavailable (%v) — falling back to internal queue", err)
		runInternal(ctx)
		return
	}
	defer q.Close()

	_ = nitro.SubscribeRoute(q, "demo_queue", "demo.event", func(ctx context.Context, msg *nitro.QueueMessage) error {
		var body map[string]any
		_ = json.Unmarshal(msg.Payload, &body)
		log.Printf("amqp received id=%s tenant=%s body=%v", msg.ID, msg.Metadata["tenant_id"], body)
		return nil
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = nitro.PublishWithOptions(ctx, q, "demo.event", map[string]any{"hello": "nitro"},
			nitro.WithTenantID("tenant-1"))
	}()

	log.Println("worker-amqp listening (RABBITMQ_URL) — Ctrl+C to stop")
	if err := q.Start(ctx); err != nil {
		log.Fatal(err)
	}
	<-ctx.Done()
	_ = q.Stop()
}

func runInternal(ctx context.Context) {
	q := nitro.NewInternalQueue()
	_ = nitro.Subscribe(q, "demo.event", func(ctx context.Context, msg *nitro.QueueMessage) error {
		log.Printf("internal received %s %s", msg.ID, string(msg.Payload))
		return nil
	})
	_ = q.Start(ctx)
	_ = nitro.PublishWithOptions(ctx, q, "demo.event", map[string]any{"hello": "nitro"},
		nitro.WithTenantID("tenant-1"))
	r := nitro.NewResilience()
	res := nitro.ExecuteNamed(r, ctx, "demo:UF", func(ctx context.Context) (string, error) {
		return "ok", nil
	}, nitro.WithTimeout(2*time.Second), nitro.WithCircuitBreaker(3, 5*time.Second))
	fmt.Println("resilience:", res.Value, res.Err)
	time.Sleep(200 * time.Millisecond)
	_ = q.Stop()
}
