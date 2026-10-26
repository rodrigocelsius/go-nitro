# Queue

Events published **after COMMIT** (`ResourceConfig.AfterCommit`) and consumed with retries — the broker comes from the environment, the code does not change:

| Env | Broker |
|-----|--------|
| none | in-process |
| `RABBITMQ_URL` | RabbitMQ |
| `NATS_URL` | NATS JetStream (streams / consumers created automatically) |
| `KAFKA_BROKERS` | Kafka (topics / consumer groups created automatically) |

While the broker is down, messages are processed in memory or buffered and re-sent. Guide: [`docs/guide/messaging.md`](../../../docs/guide/messaging.md).

```bash
docker run -p 4222:4222 nats -js   # optional
NATS_URL=nats://localhost:4222 go run .
```

```bash
cd examples/v2/queue
go test .
go run .   # :8080, SQLite in ./data unless DATABASE_URL is set
```
