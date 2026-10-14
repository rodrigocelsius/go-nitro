# Messaging (Queue)

**Language:** [English](messaging.md) · [Português](../pt/guide/messaging.md)

Nitro provides a `queue.Queue` abstraction with:

- **Internal** — in-memory channels (`nitro.NewInternalQueue`)
- **RabbitMQ** — topic exchange (`nitro.NewRabbitQueue` / `NewRabbitQueueFromEnv`)

## Quick start (in-memory)

```go
q := nitro.NewInternalQueue()
_ = nitro.Subscribe(q, "emails", func(ctx context.Context, msg *nitro.QueueMessage) error {
    // handle msg.Payload / msg.Metadata["tenant_id"]
    return nil
})
_ = q.Start(ctx)
_ = nitro.Publish(ctx, q, "emails", map[string]any{"to": "a@b.c"})
```

## RabbitMQ (topic)

```go
q, err := nitro.NewRabbitQueue(nitro.RabbitConfig{
    URL:      "amqp://guest:guest@127.0.0.1:5672/",
    Exchange: "nitro.events",
    Prefetch: 5,
})
// Bind queue name to routing key (they may differ):
_ = nitro.SubscribeRoute(q, "nfe_processing_queue", "nfe.send", handler)
_ = nitro.PublishWithOptions(ctx, q, "nfe.send", payload, nitro.WithTenantID(tenant))
_ = q.Start(ctx)
```

Env helpers: `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `RABBITMQ_PREFETCH`.

Exhausted messages go to `{queue}.dlq` when DLQ is enabled (default).

## Worker scaffold

```bash
nitrogen new myworker --worker --module github.com/acme/myworker
nitrogen install rabbitmq
go run ./cmd/worker
```

## Fleet monorepo

```bash
nitrogen new backend --fleet \
  --module github.com/acme/backend \
  --services gateway,nfe,danfe,cancel \
  --nitro-path /path/to/nitrov1
```

`gateway` is generated as `--microservice` (HTTP); other names as `--worker`.

## Resilience

```go
r := nitro.NewResilience()
nitro.SetBreakerStore(r, nitro.NewRedisBreakerStore(rdb, "nitro:cb:", 24*time.Hour))
res := nitro.ExecuteNamed(r, ctx, tenant+":SP", func(ctx context.Context) (any, error) {
    return callSEFAZ(ctx)
}, nitro.WithTimeout(10*time.Second), nitro.WithRetry(3, time.Second), nitro.WithCircuitBreaker(5, 30*time.Second))
```

See also: [`examples/v1/worker-amqp`](../../examples/v1/worker-amqp/).
