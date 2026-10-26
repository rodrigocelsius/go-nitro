package sqladapter_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// Scenario 1 (sales + sale details) and scenario 2 (payable → customer
// status) from the transaction requirements, through app.Resource hooks.

type TxSale struct {
	nitro.Model
	Customer string  `json:"customer" db:"customer"`
	Total    float64 `json:"total" db:"total"`
}

type TxSaleDetail struct {
	nitro.Model
	TxSaleID nitro.UUID `json:"tx_sale_id" db:"tx_sale_id"`
	Note     string     `json:"note" db:"note"`
}

type TxCustomer struct {
	nitro.Model
	Name   string `json:"name" db:"name"`
	Status string `json:"status" db:"status"`
}

type TxPayable struct {
	nitro.Model
	TxCustomerID nitro.UUID `json:"tx_customer_id" db:"tx_customer_id"`
	Amount       float64    `json:"amount" db:"amount"`
	Paid         bool       `json:"paid" db:"paid"`
}

const txSchema = `
CREATE TABLE tx_sales (id TEXT PRIMARY KEY, customer TEXT NOT NULL, total REAL NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT);
CREATE TABLE tx_sale_details (id TEXT PRIMARY KEY, tx_sale_id TEXT NOT NULL, note TEXT NOT NULL,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT);
CREATE TABLE tx_customers (id TEXT PRIMARY KEY, name TEXT NOT NULL, status TEXT NOT NULL,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT);
CREATE TABLE tx_payables (id TEXT PRIMARY KEY, tx_customer_id TEXT NOT NULL, amount REAL NOT NULL,
  paid INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT);`

func openTxDB(t *testing.T) *sqladapter.DB {
	t.Helper()
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := execRaw(ctx, db, txSchema); err != nil {
		t.Fatal(err)
	}
	return db
}

func countRows[T any](t *testing.T, db *sqladapter.DB) int {
	t.Helper()
	n, err := sqladapter.StoreFor[T](db).CountAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestResourceTx_SaleWithDetailsCommitsAtomically(t *testing.T) {
	db := openTxDB(t)
	details := sqladapter.StoreFor[TxSaleDetail](db)
	var committed []string

	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource(&TxSale{}, sqladapter.StoreFor[TxSale](db), &nitro.ResourceConfig{
		Public:  true,
		NoCache: true,
		AfterCreate: func(ctx context.Context, entity any) error {
			s := entity.(*TxSale)
			if _, err := details.CreateOne(ctx, &TxSaleDetail{TxSaleID: s.ID, Note: "item"}); err != nil {
				return err
			}
			if s.Customer == "FAIL" { // test switch: fail after the detail insert
				return nitro.Conflict("stock unavailable")
			}
			return nil
		},
		AfterCommit: func(ctx context.Context, action string, entity any) {
			committed = append(committed, action)
		},
	})
	tc := nitro.NewTestClient(app)

	ok := tc.Post(t, "/tx_sales", `{"customer":"Ada","total":10}`)
	tc.AssertStatus(t, ok, 201)
	if got := countRows[TxSale](t, db); got != 1 {
		t.Fatalf("sales = %d, want 1", got)
	}
	if got := countRows[TxSaleDetail](t, db); got != 1 {
		t.Fatalf("details = %d, want 1", got)
	}

	// The hook inserted the detail, then failed: sale + detail roll back
	// and the client gets the hook's error to retry.
	bad := tc.Post(t, "/tx_sales", `{"customer":"FAIL","total":5}`)
	tc.AssertStatus(t, bad, 409)
	if got := countRows[TxSale](t, db); got != 1 {
		t.Fatalf("sales after rollback = %d, want 1", got)
	}
	if got := countRows[TxSaleDetail](t, db); got != 1 {
		t.Fatalf("details after rollback = %d, want 1", got)
	}
	if len(committed) != 1 || committed[0] != "create" {
		t.Fatalf("AfterCommit calls = %v, want [create] (not on rollback)", committed)
	}
}

func TestResourceTx_PaidPayableUpdatesCustomerStatus(t *testing.T) {
	db := openTxDB(t)
	ctx := context.Background()
	customers := sqladapter.StoreFor[TxCustomer](db)
	cust, err := customers.CreateOne(ctx, &TxCustomer{Name: "Ada", Status: "pending"})
	if err != nil {
		t.Fatal(err)
	}
	ghost := nitro.NewUUID() // customer that does not exist → hook fails

	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource(&TxPayable{}, sqladapter.StoreFor[TxPayable](db), &nitro.ResourceConfig{
		Public:  true,
		NoCache: true,
		AfterCreate: func(ctx context.Context, entity any) error {
			p := entity.(*TxPayable)
			if !p.Paid { // conditional action
				return nil
			}
			n, err := customers.UpdateWhere(ctx,
				customers.Query().Where(field.Col[any]("tx_customers", "id").Eq(p.TxCustomerID)),
				field.Col[string]("tx_customers", "status").Set("clear"))
			if err != nil {
				return err
			}
			if n == 0 {
				return nitro.NotFound("customer not found")
			}
			return nil
		},
	})
	tc := nitro.NewTestClient(app)

	// Not paid: no status change.
	tc.AssertStatus(t, tc.Post(t, "/tx_payables", `{"tx_customer_id":"`+cust.ID.String()+`","amount":10}`), 201)
	if got, _ := customers.Get(ctx, cust.ID); got.Status != "pending" {
		t.Fatalf("status = %q, want pending", got.Status)
	}

	// Paid: status updated in the same transaction.
	tc.AssertStatus(t, tc.Post(t, "/tx_payables", `{"tx_customer_id":"`+cust.ID.String()+`","amount":10,"paid":true}`), 201)
	if got, _ := customers.Get(ctx, cust.ID); got.Status != "clear" {
		t.Fatalf("status = %q, want clear", got.Status)
	}

	// Paid for a missing customer: payable insert rolled back.
	before := countRows[TxPayable](t, db)
	tc.AssertStatus(t, tc.Post(t, "/tx_payables", `{"tx_customer_id":"`+ghost.String()+`","amount":10,"paid":true}`), 404)
	if got := countRows[TxPayable](t, db); got != before {
		t.Fatalf("payables = %d, want %d (rollback)", got, before)
	}
}

func TestResourceTx_BeforeDeleteVetoes(t *testing.T) {
	db := openTxDB(t)
	ctx := context.Background()
	customers := sqladapter.StoreFor[TxCustomer](db)
	payables := sqladapter.StoreFor[TxPayable](db)
	cust, _ := customers.CreateOne(ctx, &TxCustomer{Name: "Ada", Status: "pending"})
	_, _ = payables.CreateOne(ctx, &TxPayable{TxCustomerID: cust.ID, Amount: 1})

	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource(&TxCustomer{}, customers, &nitro.ResourceConfig{
		Public:  true,
		NoCache: true,
		BeforeDelete: func(ctx context.Context, id any) error {
			n, err := payables.CountWhere(ctx, field.Col[any]("tx_payables", "tx_customer_id").Eq(id))
			if err != nil {
				return err
			}
			if n > 0 {
				return nitro.Conflict("customer has open payables")
			}
			return nil
		},
	})
	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Delete(t, "/tx_customers/"+cust.ID.String()), 409)
	if got := countRows[TxCustomer](t, db); got != 1 {
		t.Fatalf("customers = %d, want 1", got)
	}
}

func TestInTx_StoresJoinAmbientTransaction(t *testing.T) {
	db := openTxDB(t)
	ctx := context.Background()
	sales := sqladapter.StoreFor[TxSale](db)
	details := sqladapter.StoreFor[TxSaleDetail](db)
	boom := errors.New("boom")
	var afterCommit, afterRollback int

	err := db.InTx(ctx, func(ctx context.Context) error {
		if !nitro.InTransaction(ctx) {
			t.Fatal("ctx should be in a transaction")
		}
		s, err := sales.CreateOne(ctx, &TxSale{Customer: "Ada"})
		if err != nil {
			return err
		}
		// Nested InTx joins the outer transaction.
		if err := sales.InTx(ctx, func(ctx context.Context) error {
			_, err := details.CreateOne(ctx, &TxSaleDetail{TxSaleID: s.ID, Note: "n"})
			return err
		}); err != nil {
			return err
		}
		nitro.AfterCommit(ctx, func(context.Context) { afterCommit++ })
		nitro.AfterRollback(ctx, func(context.Context) { afterRollback++ })
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if countRows[TxSale](t, db) != 0 || countRows[TxSaleDetail](t, db) != 0 {
		t.Fatal("expected full rollback of joined stores")
	}
	if afterCommit != 0 || afterRollback != 1 {
		t.Fatalf("afterCommit=%d afterRollback=%d", afterCommit, afterRollback)
	}

	err = nitro.InTx(ctx, db, func(ctx context.Context) error {
		_, err := sales.CreateOne(ctx, &TxSale{Customer: "Bob"})
		nitro.AfterCommit(ctx, func(context.Context) { afterCommit++ })
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if countRows[TxSale](t, db) != 1 || afterCommit != 1 {
		t.Fatalf("commit path: sales=%d afterCommit=%d", countRows[TxSale](t, db), afterCommit)
	}
}

func TestInTx_TxDBJoinsAndPanicRollsBack(t *testing.T) {
	db := openTxDB(t)
	ctx := context.Background()

	func() {
		defer func() { _ = recover() }()
		_ = db.Tx(ctx, func(tx *sqladapter.DB) error {
			if _, err := sqladapter.StoreFor[TxSale](tx).CreateOne(ctx, &TxSale{Customer: "x"}); err != nil {
				return err
			}
			// Tx on a tx-bound DB joins instead of failing with "no pool".
			if err := tx.Tx(ctx, func(inner *sqladapter.DB) error { return nil }); err != nil {
				t.Errorf("nested Tx: %v", err)
			}
			panic("boom")
		})
	}()
	if got := countRows[TxSale](t, db); got != 0 {
		t.Fatalf("sales = %d, want 0 after panic", got)
	}
}

func TestResourceTx_NoTxOptOut(t *testing.T) {
	db := openTxDB(t)
	details := sqladapter.StoreFor[TxSaleDetail](db)
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource(&TxSale{}, sqladapter.StoreFor[TxSale](db), &nitro.ResourceConfig{
		Public:  true,
		NoCache: true,
		NoTx:    true,
		AfterCreate: func(ctx context.Context, entity any) error {
			if nitro.InTransaction(ctx) {
				t.Error("NoTx: hook must not run in a transaction")
			}
			_, _ = details.CreateOne(ctx, &TxSaleDetail{TxSaleID: entity.(*TxSale).ID, Note: "n"})
			return errors.New("fail")
		},
	})
	tc := nitro.NewTestClient(app)
	if resp := tc.Post(t, "/tx_sales", `{"customer":"Ada"}`); resp.StatusCode() < 400 {
		t.Fatalf("status %d", resp.StatusCode())
	}
	// Without the transaction the sale stays (documented NoTx behaviour).
	if got := countRows[TxSale](t, db); got != 1 {
		t.Fatalf("sales = %d, want 1", got)
	}
}

func TestResourceTx_CommandRunsInTransaction(t *testing.T) {
	db := openTxDB(t)
	ctx := context.Background()
	sales := sqladapter.StoreFor[TxSale](db)
	details := sqladapter.StoreFor[TxSaleDetail](db)
	s, _ := sales.CreateOne(ctx, &TxSale{Customer: "Ada"})

	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource(&TxSale{}, sales, &nitro.ResourceConfig{
		Public:  true,
		NoCache: true,
		Commands: []nitro.ResourceCommand{
			nitro.Cmd("close", func(ctx context.Context, id string) (*TxSale, error) {
				if _, err := details.CreateOne(ctx, &TxSaleDetail{TxSaleID: s.ID, Note: "closing"}); err != nil {
					return nil, err
				}
				return nil, nitro.UnprocessableEntity("cannot close")
			}),
		},
	})
	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Post(t, "/tx_sales/"+s.ID.String()+"/close", `{}`), 422)
	if got := countRows[TxSaleDetail](t, db); got != 0 {
		t.Fatalf("details = %d, want 0 (command rolled back)", got)
	}
}

type TxOrder struct {
	nitro.Model
	Customer string         `json:"customer" db:"customer"`
	Total    float64        `json:"total" db:"total"`
	Items    []TxSaleDetail `json:"items" db:"-"` // nested body, not a column
}

func (TxOrder) TableName() string { return "tx_sales" }

// The documented pattern: nested items (db:"-") survive repo.Create so the
// AfterCreate hook can insert them with the parent id, in the same tx.
func TestResourceTx_NestedItemsReachAfterCreate(t *testing.T) {
	db := openTxDB(t)
	details := sqladapter.StoreFor[TxSaleDetail](db)
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource(&TxOrder{}, sqladapter.StoreFor[TxOrder](db), &nitro.ResourceConfig{
		Public:  true,
		NoCache: true,
		AfterCreate: func(ctx context.Context, e any) error {
			o := e.(*TxOrder)
			if len(o.Items) == 0 {
				return nitro.BadRequest("items required")
			}
			for i := range o.Items {
				o.Items[i].TxSaleID = o.ID
			}
			_, err := details.CreateMany(ctx, o.Items)
			return err
		},
	})
	tc := nitro.NewTestClient(app)
	resp := tc.Post(t, "/tx_sales", `{"customer":"Ada","total":3,"items":[{"note":"a"},{"note":"b"}]}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, `"note":"b"`)
	if got := countRows[TxSaleDetail](t, db); got != 2 {
		t.Fatalf("details = %d, want 2", got)
	}
}
