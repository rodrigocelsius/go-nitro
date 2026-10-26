// queue — publish events after COMMIT and consume them, with the broker chosen
// by the environment and no code changes:
//
//	(nothing)                          in-process queue
//	RABBITMQ_URL=amqp://…              RabbitMQ
//	NATS_URL=nats://localhost:4222     NATS JetStream
//	KAFKA_BROKERS=localhost:9092       Kafka
//
// While the broker is unreachable the app keeps working: subscribed queues are
// processed in memory, the rest is buffered and re-sent when it is back.
//
//	cd examples/v2/queue && go test . && go run .
package main

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"os"
	"os/signal"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Order struct {
	nitro.Model
	CustomerEmail string  `json:"customer_email" validate:"required,email"`
	Total         float64 `json:"total" validate:"gt=0"`
	Status        string  `json:"status" readonly:"true"`
}

// OrderPlaced is the event contract published on "orders.placed".
type OrderPlaced struct {
	OrderID string  `json:"order_id"`
	Email   string  `json:"email"`
	Total   float64 `json:"total"`
}

func newApp(db *sqladapter.DB, q nitro.Queue) *nitro.App {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource(&Order{}, sqladapter.StoreFor[Order](db), &nitro.ResourceConfig{
		Public: true,
		BeforeCreate: func(ctx context.Context, e any) error {
			e.(*Order).Status = "placed"
			return nil
		},
		// Published only if the order was committed: no "ghost" events for
		// rolled-back writes, and the HTTP request never waits on a retry.
		AfterCommit: func(ctx context.Context, action string, e any) {
			if action != "create" {
				return
			}
			o := e.(*Order)
			ev := OrderPlaced{OrderID: o.ID.String(), Email: o.CustomerEmail, Total: o.Total}
			if err := nitro.PublishWithOptions(ctx, q, "orders.placed", ev, nitro.WithQueueMetadata(map[string]string{"key": o.CustomerEmail})); err != nil {
				log.Printf("publish orders.placed: %v", err)
			}
		},
	})
	return app
}

// Mailer is the consumer side (an e-mail service, an invoice worker, …).
type Mailer interface {
	Send(to, subject string) error
}

// subscribe wires the consumers. A handler error is retried with backoff and
// dead-lettered to "orders.placed.dlq" after the retries.
func subscribe(q nitro.Queue, mail Mailer) error {
	return q.Subscribe("orders.placed", func(ctx context.Context, m *nitro.QueueMessage) error {
		var ev OrderPlaced
		if err := json.Unmarshal(m.Payload, &ev); err != nil {
			return err
		}
		return mail.Send(ev.Email, "Order "+ev.OrderID+" received")
	})
}

type logMailer struct{}

func (logMailer) Send(to, subject string) error {
	log.Printf("mail to %s: %s", to, subject)
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	dsn := sqladapter.EnvDSN()
	var db *sqladapter.DB
	var err error
	if dsn == "" {
		db, err = sqladapter.Open(ctx, "sqlite", "file:data/queue.db")
	} else {
		db, err = sqladapter.OpenEnv(ctx)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.MigrateFS(ctx, migrations); err != nil {
		log.Fatal(err)
	}

	q := nitro.QueueFromEnv()
	if err := subscribe(q, logMailer{}); err != nil {
		log.Fatal(err)
	}
	if err := q.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer q.Stop()
	nitro.Ok(newApp(db, q).Listen(":8080"))
}
