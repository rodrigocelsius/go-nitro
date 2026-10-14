# worker-amqp

Queue worker: RabbitMQ when env is set, otherwise the in-memory queue.

```bash
# optional
export RABBITMQ_URL='amqp://guest:guest@localhost:5672/'
go run .
```

See [`docs/guide/messaging.md`](../../../docs/guide/messaging.md).
