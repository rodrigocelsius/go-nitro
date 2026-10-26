# Mensageria (Queue)

**Idioma:** [Português](messaging.md) · [English](../../guide/messaging.md)

O Nitro oferece a abstração `queue.Queue` com:

- **Internal** — canais em memória (`nitro.NewInternalQueue`)
- **RabbitMQ** — topic exchange (`nitro.NewRabbitQueue` / `NewRabbitQueueFromEnv`)
- **NATS JetStream** / **Kafka** — `nitro.NewNATSQueue` / `nitro.NewKafkaQueue`
- **`nitro.QueueFromEnv()`** — RabbitMQ, NATS ou Kafka quando configurados, memória caso contrário, com fallback automático

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

## Convenção: `QueueFromEnv` (com fallback)

```go
q := nitro.QueueFromEnv()           // sem if/else no main
_ = q.Subscribe("emails", sendEmail)
_ = q.Start(ctx)
defer q.Stop()
_ = q.Publish(ctx, "emails", msg)
```

| Situação | Comportamento |
|----------|---------------|
| Nenhuma variável de broker | fila em processo (workers neste processo) |
| `RABBITMQ_URL` / `NATS_URL` / `KAFKA_BROKERS` (vale a primeira definida, nesta ordem; `NITRO_QUEUE=rabbitmq\|nats\|kafka\|memory` força um) | publica nesse broker (acks / publisher confirms, retentativas, DLQ) |
| Broker fora no boot ou depois | filas com subscriber **neste processo** são processadas em memória; as demais mensagens ficam em buffer (limitado, 10 000) e são republicadas em ordem quando o broker volta; a conexão é refeita a cada 5 s |
| Buffer em memória cheio | `Publish` espera até `PublishTimeout` (1 s, ou o deadline do ctx) um worker liberar espaço — backpressure em vez de erro imediato — e então retorna `nitro.ErrQueueFull` |

Mensagens em buffer ficam em memória: são perdidas se o processo parar antes do broker voltar (logado no `Stop`). Para entrega garantida, grave o evento numa tabela na mesma transação (outbox) e publique em `nitro.AfterCommit` ou num job. Um restart do broker é recuperado no próximo `Publish` (processos que só publicam também reconectam).

Publique só depois do commit:

```go
nitro.AfterCommit(ctx, func(ctx context.Context) {
    _ = q.Publish(ctx, "sale.created", sale)
})
```

Envolva um cliente explícito com `nitro.NewFallbackQueue(rabbit, nitro.FallbackQueueConfig{BufferSize: 50_000})`. A entrega em memória casa o nome da fila/routing key exatamente (curingas de tópico como `orders.*` só valem no RabbitMQ).

## NATS JetStream e Kafka (sem configuração)

```bash
nitrogen install nats    # NATS_URL + docker-compose (nats -js)
nitrogen install kafka   # KAFKA_BROKERS + docker-compose (Kafka KRaft)
```

`nitro.QueueFromEnv()` passa a usar o broker com o mesmo código — nada a declarar no servidor:

| | NATS JetStream (`NATS_URL`) | Kafka (`KAFKA_BROKERS`) |
|-|-----------------------------|-------------------------|
| Criado no primeiro uso | stream `NITRO_<FILA>` (retenção work-queue, em disco, 7 dias) + consumer durável por fila | tópico (3 partições, `KAFKA_PARTITIONS`) + consumer group `<serviço>.<fila>` |
| Várias instâncias, mesma fila | dividem as mensagens (um consumer durável) | dividem as partições (um group) |
| Duas filas no mesmo subject/tópico (`SubscribeRoute`) | não suportado (streams work-queue: um consumer por subject) | cada fila recebe todas as mensagens (um group por fila) |
| Retentativas | reentrega com backoff exponencial (`NATS_RETRY_MAX`, padrão 3) | retentativas no processo com backoff (`KAFKA_RETRY_MAX`, padrão 3); offset confirmado após sucesso |
| Esgotadas | publicadas em `<fila>.dlq` com `x-nitro-error` | publicadas em `<tópico>.dlq` com `x-nitro-error` |
| Ordem | por subject | por partição — defina `metadata["key"]` (ex.: `nitro.WithQueueMetadata(map[string]string{"key": customerID})`) |
| Metadados | headers NATS | headers do registro |

Clientes explícitos: `nitro.NewNATSQueue(nitro.NATSConfig{…})` / `NewNATSQueueFromEnv()`, `nitro.NewKafkaQueue(nitro.KafkaConfig{…})` / `NewKafkaQueueFromEnv()`. Binários que não usam esses brokers podem removê-los com `-tags nitro_nonats,nitro_nokafka`.

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
