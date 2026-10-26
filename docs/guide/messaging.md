# Messaging (Queue)

**Language:** [English](messaging.md) · [Português](../pt/guide/messaging.md)

Nitro provides a `queue.Queue` abstraction with:

- **Internal** — in-memory channels (`nitro.NewInternalQueue`)
- **RabbitMQ** — topic exchange (`nitro.NewRabbitQueue` / `NewRabbitQueueFromEnv`)
- **NATS JetStream** / **Kafka** — `nitro.NewNATSQueue` / `nitro.NewKafkaQueue`
- **`nitro.QueueFromEnv()`** — RabbitMQ, NATS or Kafka when configured, in-memory otherwise, with automatic fallback

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

## Convention: `QueueFromEnv` (with fallback)

```go
q := nitro.QueueFromEnv()           // no if/else in main
_ = q.Subscribe("emails", sendEmail)
_ = q.Start(ctx)
defer q.Stop()
_ = q.Publish(ctx, "emails", msg)
```

| Situation | Behaviour |
|-----------|-----------|
| No broker variable | in-process queue (workers in this process) |
| `RABBITMQ_URL` / `NATS_URL` / `KAFKA_BROKERS` (first set wins, in this order; `NITRO_QUEUE=rabbitmq\|nats\|kafka\|memory` forces one) | publish to that broker (acks / publisher confirms, retries, DLQ) |
| Broker down at boot or later | queues with a subscriber **in this process** are processed in memory; other messages are buffered (bounded, 10 000) and re-published in order when the broker is back; the connection is retried every 5 s |
| In-memory buffer full | `Publish` waits up to `PublishTimeout` (1 s, or the ctx deadline) for a worker to free a slot — backpressure instead of an immediate error — then returns `nitro.ErrQueueFull` |

Buffered messages live in memory: they are lost if the process stops before the broker returns (logged on `Stop`). For guaranteed delivery, write the event to a table inside the same transaction (outbox) and publish it from `nitro.AfterCommit` or a job. A broker restart is healed on the next `Publish` (publish-only processes reconnect too).

Publish only after the data is committed:

```go
nitro.AfterCommit(ctx, func(ctx context.Context) {
    _ = q.Publish(ctx, "sale.created", sale)
})
```

Wrap an explicit client yourself with `nitro.NewFallbackQueue(rabbit, nitro.FallbackQueueConfig{BufferSize: 50_000})`. In-memory delivery matches the queue/routing-key name exactly (topic wildcards like `orders.*` only apply on RabbitMQ).

## NATS JetStream and Kafka (zero configuration)

```bash
nitrogen install nats    # NATS_URL + docker-compose (nats -js)
nitrogen install kafka   # KAFKA_BROKERS + docker-compose (Kafka KRaft)
```

`nitro.QueueFromEnv()` then uses the broker with the same code — nothing to declare on the server:

| | NATS JetStream (`NATS_URL`) | Kafka (`KAFKA_BROKERS`) |
|-|-----------------------------|-------------------------|
| Created on first use | stream `NITRO_<QUEUE>` (work-queue retention, file storage, 7 days) + durable consumer per queue | topic (3 partitions, `KAFKA_PARTITIONS`) + consumer group `<service>.<queue>` |
| Several instances, same queue | share the messages (one durable consumer) | share the partitions (one group) |
| Two queues on one subject/topic (`SubscribeRoute`) | not supported (work-queue streams: one consumer per subject) | each queue gets every message (one group per queue) |
| Retries | redelivery with exponential backoff (`NATS_RETRY_MAX`, default 3) | in-process retries with backoff (`KAFKA_RETRY_MAX`, default 3); offset committed after success |
| Exhausted | published to `<queue>.dlq` with `x-nitro-error` | published to `<topic>.dlq` with `x-nitro-error` |
| Ordering | per subject | per partition — set `metadata["key"]` (e.g. `nitro.WithQueueMetadata(map[string]string{"key": customerID})`) |
| Metadata | NATS headers | record headers |

Explicit clients: `nitro.NewNATSQueue(nitro.NATSConfig{…})` / `NewNATSQueueFromEnv()`, `nitro.NewKafkaQueue(nitro.KafkaConfig{…})` / `NewKafkaQueueFromEnv()`. Postgres-only or RabbitMQ-only binaries can drop the clients with `-tags nitro_nonats,nitro_nokafka`.

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
