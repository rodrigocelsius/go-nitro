// business-commands — business actions on a Resource instead of ad-hoc routes:
//
//	POST /orders/place            collection Command (creates a draft)
//	POST /orders/:id/submit       item Command
//	POST /orders/:id/approve      item Command, role "manager" only when total > 10 000
//	POST /orders/:id/cancel       item Command with a validated body
//	GET  /orders/queries/pending  named Query
//
// Rules run before every item Command (a cancelled order rejects all of
// them). Each Command is one transaction: the status change and the audit row
// in order_events commit together. JWT protects every route by default.
//
//	cd examples/v2/business-commands && go test . && go run .
package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Order struct {
	nitro.Model
	CustomerID string  `json:"customer_id"`
	Status     string  `json:"status" readonly:"true"`
	Total      float64 `json:"total"`
	Note       string  `json:"note"`
}

type OrderEvent struct {
	nitro.Model
	OrderID nitro.UUID `json:"order_id"`
	Kind    string     `json:"kind"`
	Actor   string     `json:"actor"`
}

// HighValue is the threshold above which approval needs a manager.
const HighValue = 10_000

type PlaceInput struct {
	CustomerID string  `json:"customer_id" validate:"required"`
	Total      float64 `json:"total" validate:"gt=0"`
}

type ReasonInput struct {
	Reason string `json:"reason" validate:"required,min=3"`
}

// orders is the use-case layer: plain functions over BLinq stores. Every
// call made with the Command's ctx joins its transaction.
type orders struct {
	store  *sqladapter.Store[Order]
	events *sqladapter.Store[OrderEvent]
}

func actor(ctx context.Context) string {
	if c, ok := nitro.RequestFrom(ctx); ok && c.UserID() != "" {
		return c.UserID()
	}
	return "system"
}

func (o *orders) transition(ctx context.Context, id, from, to string, note ...string) (*Order, error) {
	ord, err := o.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if from != "" && ord.Status != from {
		return nil, nitro.Conflict("order is " + ord.Status + ", expected " + from)
	}
	ord.Status = to
	if len(note) > 0 {
		ord.Note = note[0]
	}
	upd, err := o.store.Update(ctx, ord.ID,
		field.Col[string]("orders", "status").Set(ord.Status),
		field.Col[string]("orders", "note").Set(ord.Note))
	if err != nil {
		return nil, err
	}
	if _, err := o.events.CreateOne(ctx, &OrderEvent{OrderID: ord.ID, Kind: to, Actor: actor(ctx)}); err != nil {
		return nil, err // the status change is rolled back too
	}
	return upd, nil
}

func (o *orders) Place(ctx context.Context, in PlaceInput) (*Order, error) {
	return o.store.CreateOne(ctx, &Order{CustomerID: in.CustomerID, Total: in.Total, Status: "draft"})
}

func (o *orders) Submit(ctx context.Context, id string) (*Order, error) {
	return o.transition(ctx, id, "draft", "pending")
}

func (o *orders) Approve(ctx context.Context, id string) (*Order, error) {
	return o.transition(ctx, id, "pending", "approved")
}

func (o *orders) Cancel(ctx context.Context, id string, in ReasonInput) (*Order, error) {
	return o.transition(ctx, id, "", "cancelled", in.Reason)
}

func (o *orders) Pending(ctx context.Context, page nitro.Page) ([]Order, error) {
	q := o.store.Query().
		Where(field.Col[string]("orders", "status").Eq("pending")).
		OrderBy(field.Col[any]("orders", "created_at").OrderDesc())
	return o.store.Find(ctx, q.Take(page.PerPage))
}

func newApp(db *sqladapter.DB, auth *nitro.Auth) *nitro.App {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.AuthRoutes("/auth", auth)
	app.SetDefaultResourceMiddleware(nitro.JWTMiddleware(auth.Provider()))

	uc := &orders{store: sqladapter.StoreFor[Order](db), events: sqladapter.StoreFor[OrderEvent](db)}
	app.Resource(&Order{}, uc.store, &nitro.ResourceConfig{
		Only: []string{nitro.ActionList, nitro.ActionGet}, // writes only through Commands
		Rules: nitro.Rules(func(o *Order) error {
			if o.Status == "cancelled" {
				return nitro.Conflict("order is cancelled")
			}
			return nil
		}),
		Commands: nitro.Commands(
			nitro.Cmd("place", uc.Place),
			nitro.Cmd("submit", uc.Submit),
			nitro.Cmd("approve", uc.Approve).Roles("manager").When(func(o *Order) bool { return o.Total > HighValue }),
			nitro.Cmd("cancel", uc.Cancel),
		),
		Queries: nitro.Queries(nitro.Q("pending", uc.Pending)),
	})
	return app
}

func main() {
	ctx := context.Background()
	var db *sqladapter.DB
	var err error
	if sqladapter.EnvDSN() == "" {
		db, err = sqladapter.Open(ctx, "sqlite", "file:data/orders.db")
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
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-only-secret-change-me-0123456789abcdef"
	}
	auth := nitro.NewAuth(nitro.AuthConfig{JWTSecret: secret}).WithMemoryStore()
	nitro.Ok(newApp(db, auth).Listen(":8080"))
}
