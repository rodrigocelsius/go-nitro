// transactions — the two scenarios every business app has, with BLinq's
// ambient transactions (the transaction travels in ctx):
//
//  1. POST /sales inserts the sale and its items (and decrements stock) in one
//     transaction; any error rolls everything back and the client retries.
//  2. POST /payables with paid=true clears the customer's status in the same
//     transaction; a customer with open payables cannot be deleted.
//
// Plus a use case outside Resource (db.InTx) and side effects after COMMIT.
//
//	cd examples/v2/transactions && go test . && go run .
package main

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Sale struct {
	nitro.Model
	Customer string     `json:"customer" validate:"required"`
	Total    float64    `json:"total" computed:"true"`
	Items    []SaleItem `json:"items" db:"-"` // nested body, inserted by AfterCreate
}

type SaleItem struct {
	nitro.Model
	SaleID nitro.UUID `json:"sale_id"`
	SKU    string     `json:"sku" validate:"required"`
	Qty    int        `json:"qty" validate:"gt=0"`
	Price  float64    `json:"price" validate:"gt=0"`
}

type Stock struct {
	nitro.Model
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

// TableName keeps the table singular ("stock", not "stocks").
func (Stock) TableName() string { return "stock" }

type Customer struct {
	nitro.Model
	Name   string `json:"name" validate:"required"`
	Status string `json:"status" readonly:"true"`
}

type Payable struct {
	nitro.Model
	CustomerID nitro.UUID `json:"customer_id" validate:"required"`
	Amount     float64    `json:"amount" validate:"gt=0"`
	Paid       bool       `json:"paid"`
}

// Events receives side effects published only after COMMIT (a queue in a real app).
type Events interface {
	Publish(ctx context.Context, name string, payload any) error
}

func newApp(db *sqladapter.DB, events Events) *nitro.App {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	sales := sqladapter.StoreFor[Sale](db)
	items := sqladapter.StoreFor[SaleItem](db)
	stock := sqladapter.StoreFor[Stock](db)
	customers := sqladapter.StoreFor[Customer](db)
	payables := sqladapter.StoreFor[Payable](db)

	// Scenario 1: sale + items + stock, all-or-nothing.
	app.Resource(&Sale{}, sales, &nitro.ResourceConfig{
		Public: true,
		BeforeWrite: func(ctx context.Context, e any) error {
			s := e.(*Sale)
			if len(s.Items) == 0 {
				return nitro.UnprocessableEntity("a sale needs at least one item")
			}
			s.Total = 0
			for _, it := range s.Items {
				s.Total += float64(it.Qty) * it.Price
			}
			return nil
		},
		AfterCreate: func(ctx context.Context, e any) error {
			s := e.(*Sale)
			for i := range s.Items {
				s.Items[i].SaleID = s.ID
				// Conditional check before the write: enough stock?
				n, err := stock.UpdateWhere(ctx,
					stock.Query().Where(field.Col[string]("stock", "sku").Eq(s.Items[i].SKU)).
						Where(field.Col[int]("stock", "qty").Gte(s.Items[i].Qty)),
					field.Col[int]("stock", "qty").Inc(-s.Items[i].Qty))
				if err != nil {
					return err
				}
				if n == 0 {
					return nitro.Conflict("insufficient stock for " + s.Items[i].SKU) // rolls back the sale
				}
			}
			saved, err := items.CreateMany(ctx, s.Items)
			s.Items = saved
			return err
		},
		AfterCommit: func(ctx context.Context, action string, e any) {
			_ = events.Publish(ctx, "sale."+action+"d", e) // never fired on rollback
		},
	})

	// Scenario 2: a paid payable clears the customer, in the same transaction.
	app.Resource(&Payable{}, payables, &nitro.ResourceConfig{
		Public: true,
		AfterCreate: func(ctx context.Context, e any) error {
			p := e.(*Payable)
			if !p.Paid { // conditional action
				return nil
			}
			_, err := customers.Update(ctx, p.CustomerID, field.Col[string]("customers", "status").Set("clear"))
			if err != nil {
				return nitro.NotFound("customer not found") // payable insert rolled back
			}
			return nil
		},
	})
	app.Resource(&Customer{}, customers, &nitro.ResourceConfig{
		Public: true,
		BeforeWrite: func(ctx context.Context, e any) error {
			if c := e.(*Customer); c.Status == "" {
				c.Status = "pending"
			}
			return nil
		},
		// Check before delete: open payables veto it (409).
		BeforeDelete: func(ctx context.Context, id any) error {
			open, err := payables.CountWhere(ctx,
				field.Col[any]("payables", "customer_id").Eq(id),
				field.Col[bool]("payables", "paid").Eq(false))
			if err != nil {
				return err
			}
			if open > 0 {
				return nitro.Conflict("customer has open payables")
			}
			return nil
		},
	})
	return app
}

// Restock is a use case outside Resource: several writes, one transaction,
// retried on deadlock / serialization failure.
func Restock(ctx context.Context, db *sqladapter.DB, events Events, skus map[string]int) error {
	stock := sqladapter.StoreFor[Stock](db)
	return db.InTxRetry(ctx, 3, func(ctx context.Context) error {
		for sku, qty := range skus {
			n, err := stock.UpdateWhere(ctx,
				stock.Query().Where(field.Col[string]("stock", "sku").Eq(sku)),
				field.Col[int]("stock", "qty").Inc(qty))
			if err != nil {
				return err
			}
			if n == 0 {
				if _, err := stock.CreateOne(ctx, &Stock{SKU: sku, Qty: qty}); err != nil {
					return err
				}
			}
		}
		nitro.AfterCommit(ctx, func(ctx context.Context) { _ = events.Publish(ctx, "stock.restocked", skus) })
		return nil
	})
}

type logEvents struct{}

func (logEvents) Publish(_ context.Context, name string, payload any) error {
	log.Printf("event %s: %v", name, payload)
	return nil
}

func openDB(ctx context.Context) (*sqladapter.DB, error) {
	if sqladapter.EnvDSN() == "" {
		return sqladapter.Open(ctx, "sqlite", "file:data/transactions.db")
	}
	return sqladapter.OpenEnv(ctx)
}

func main() {
	ctx := context.Background()
	db, err := openDB(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.MigrateFS(ctx, migrations); err != nil {
		log.Fatal(err)
	}
	if err := Restock(ctx, db, logEvents{}, map[string]int{"A1": 10, "B2": 5}); err != nil {
		log.Fatal(err)
	}
	fmt.Println(`try: curl -XPOST localhost:8080/sales -d '{"customer":"Ada","items":[{"sku":"A1","qty":2,"price":10}]}'`)
	nitro.Ok(newApp(db, logEvents{}).Listen(":8080"))
}
