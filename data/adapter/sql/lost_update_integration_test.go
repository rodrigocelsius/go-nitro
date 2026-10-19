//go:build integration

package sqladapter_test

import (
	"context"
	"os"
	"testing"

	blinqfield "github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type luAccount struct {
	nitro.HardModel
	Name    string `json:"name"`
	Balance int    `json:"balance" readonly:"true"`
}

func (luAccount) TableName() string { return "lu_accounts" }

// A PATCH that changes only `name` must not overwrite `balance` written by a
// concurrent request between the Resource's read and its write.
func TestResourcePatch_NoLostUpdate(t *testing.T) {
	dsn := os.Getenv("NITRO_PG_DSN")
	if dsn == "" {
		t.Skip("set NITRO_PG_DSN")
	}
	ctx := context.Background()
	db, err := sqladapter.Open(ctx, "postgresql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Pgx().ExecSQL(ctx, `DROP TABLE IF EXISTS lu_accounts;
		CREATE TABLE lu_accounts (id uuid primary key, name text not null, balance int not null default 0,
		created_at timestamptz not null, updated_at timestamptz not null)`); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Pgx().ExecSQL(ctx, `DROP TABLE IF EXISTS lu_accounts`) }()

	store := sqladapter.StoreFor[luAccount](db)
	acc, err := store.CreateOne(ctx, &luAccount{Name: "old", Balance: 100})
	if err != nil {
		t.Fatal(err)
	}

	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource(&luAccount{}, store, &nitro.ResourceConfig{
		NoCache: true,
		Public:  true,
		BeforeUpdate: func(ctx context.Context, _ any) error {
			// Concurrent writer (another request / service) credits the account.
			_, err := store.UpdateWhere(ctx,
				store.Query().Where(sqlCol[string]("lu_accounts", "id").Eq(acc.ID.String())),
				sqlCol[int]("lu_accounts", "balance").Set(250))
			return err
		},
	})
	tc := nitro.NewTestClient(app)
	res := tc.Patch(t, "/lu_accounts/"+acc.ID.String(), `{"name":"new"}`)
	tc.AssertStatus(t, res, 200)

	got, err := store.Get(ctx, acc.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "new" {
		t.Fatalf("name = %q, want new", got.Name)
	}
	if got.Balance != 250 {
		t.Fatalf("balance = %d, want 250 (concurrent write was overwritten: lost update)", got.Balance)
	}
}

func sqlCol[T any](table, col string) blinqfield.Field[T] { return blinqfield.Col[T](table, col) }
