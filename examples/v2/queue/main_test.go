package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type fakeMailer struct {
	mu    sync.Mutex
	sent  []string
	fails int
	done  chan struct{}
}

func (f *fakeMailer) Send(to, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fails > 0 {
		f.fails--
		return errors.New("smtp timeout")
	}
	f.sent = append(f.sent, to)
	close(f.done)
	return nil
}

func setup(t *testing.T, mail *fakeMailer) *nitro.TestClient {
	t.Helper()
	for _, k := range []string{"RABBITMQ_URL", "NATS_URL", "KAFKA_BROKERS", "NITRO_QUEUE"} {
		t.Setenv(k, "") // in-process queue
	}
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.MigrateFS(ctx, migrations); err != nil {
		t.Fatal(err)
	}
	q := nitro.QueueFromEnv()
	if err := subscribe(q, mail); err != nil {
		t.Fatal(err)
	}
	if err := q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Stop() })
	return nitro.NewTestClient(newApp(db, q))
}

func TestOrderPlaced_IsConsumedAfterCommit(t *testing.T) {
	mail := &fakeMailer{fails: 1, done: make(chan struct{})} // first attempt fails → retried
	tc := setup(t, mail)
	tc.AssertStatus(t, tc.Post(t, "/orders", `{"customer_email":"ada@example.com","total":99}`), 201)
	select {
	case <-mail.done:
	case <-time.After(10 * time.Second):
		t.Fatal("event not consumed")
	}
	if mail.sent[0] != "ada@example.com" {
		t.Fatalf("sent = %v", mail.sent)
	}
}

func TestInvalidOrder_PublishesNothing(t *testing.T) {
	mail := &fakeMailer{done: make(chan struct{})}
	tc := setup(t, mail)
	tc.AssertStatus(t, tc.Post(t, "/orders", `{"customer_email":"not-an-email","total":99}`), 422)
	select {
	case <-mail.done:
		t.Fatal("no event expected")
	case <-time.After(200 * time.Millisecond):
	}
}
