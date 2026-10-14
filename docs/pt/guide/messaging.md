# Mensageria (Queue)

**Idioma:** [Português](messaging.md) · [English](../../guide/messaging.md)

O Nitro oferece a abstração `queue.Queue` com:

- **Internal** — canais em memória (`nitro.NewInternalQueue`)
- **RabbitMQ** — topic exchange (`nitro.NewRabbitQueue` / `NewRabbitQueueFromEnv`)

## Início rápido (em memória)

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

Helpers de env: `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `RABBITMQ_PREFETCH`.

Mensagens esgotadas vão para `{queue}.dlq` quando a DLQ está habilitada (padrão).

## Scaffold de worker

```bash
nitrogen new myworker --worker --module github.com/acme/myworker
nitrogen install rabbitmq
go run ./cmd/worker
```

## Monorepo fleet

```bash
nitrogen new backend --fleet \
  --module github.com/acme/backend \
  --services gateway,nfe,danfe,cancel \
  --nitro-path /path/to/nitrov1
```

`gateway` é gerado como `--microservice` (HTTP); os demais nomes como `--worker`.

## Resilience

```go
r := nitro.NewResilience()
nitro.SetBreakerStore(r, nitro.NewRedisBreakerStore(rdb, "nitro:cb:", 24*time.Hour))
res := nitro.ExecuteNamed(r, ctx, tenant+":SP", func(ctx context.Context) (any, error) {
    return callSEFAZ(ctx)
}, nitro.WithTimeout(10*time.Second), nitro.WithRetry(3, time.Second), nitro.WithCircuitBreaker(5, 30*time.Second))
```

Veja também: [`examples/v1/worker-amqp`](../../examples/v1/worker-amqp/).
