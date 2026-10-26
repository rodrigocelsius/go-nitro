package handler_test

import (
	"context"
	"path/filepath"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/repository"
)

func TestAdjustStock_TransactionalUseCase(t *testing.T) {
	ctx := context.Background()
	db, err := sqladapter.OpenTestMigrate(ctx, filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := sqladapter.StoreFor[model.Product](db)
	repo := repository.NewProduct(store)
	p, err := store.CreateOne(ctx, &model.Product{Name: "Chair", Price: 10, Stock: 2})
	if err != nil {
		t.Fatal(err)
	}

	app := nitro.New(nitro.Config{DisableDefaults: true})
	handler.RegisterStock(app, db, repo)
	tc := nitro.NewTestClient(app)

	tc.AssertBodyContains(t, tc.PostJSON(t, "/products/"+p.ID.String()+"/stock", map[string]int{"delta": 3}), `"stock":5`)
	tc.AssertStatus(t, tc.PostJSON(t, "/products/"+p.ID.String()+"/stock", map[string]int{"delta": -9}), 409)
	got, _ := store.Get(ctx, p.ID)
	if got.Stock != 5 {
		t.Fatalf("stock = %d, want 5 (failed adjustment rolled back)", got.Stock)
	}
}
