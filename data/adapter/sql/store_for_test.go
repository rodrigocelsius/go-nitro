package sqladapter

import (
	"context"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type sfOrderItem struct {
	nitro.Model
	Qty int
}

type sfLegacy struct {
	nitro.Model
	Name string
}

func (sfLegacy) TableName() string { return "tbl_legacy" }

func TestStoreForInfersTable(t *testing.T) {
	db, err := Open(context.Background(), "sqlite", "file:storefor?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := StoreFor[sfOrderItem](db).Table(); got != "sf_order_items" {
		t.Errorf("StoreFor[sfOrderItem] table = %q", got)
	}
	if got := StoreFor[sfLegacy](db).Table(); got != "tbl_legacy" {
		t.Errorf("TableName() must win, got %q", got)
	}
}
