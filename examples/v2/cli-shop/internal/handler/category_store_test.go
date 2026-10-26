package handler_test

import (
	"context"
	"path/filepath"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
)

func TestCategoryStore_OpenTest(t *testing.T) {
	ctx := context.Background()
	// go test ./internal/handler → cwd is this package; migrations live at module root.
	mig := filepath.Join("..", "..", "migrations")
	db, err := sqladapter.OpenTestMigrate(ctx, mig)
	if err != nil {
		t.Fatalf("OpenTestMigrate: %v (need migrations/ — nitrogen migrate:make)", err)
	}
	defer db.Close()
	store := sqladapter.StoreFor[model.Category](db)
	got, err := store.CreateOne(ctx, &model.Category{})
	if err != nil {
		t.Fatalf("CreateOne: %v", err)
	}
	if got.ID.IsZero() {
		t.Fatal("expected generated id")
	}
}
