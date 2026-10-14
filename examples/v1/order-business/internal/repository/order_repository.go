// Package repository adapts BLinq Stores to the usecase Repository ports.
package repository

import (
	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/usecase/order"
)

// NewOrderRepository adapts a BLinq Store to order.Repository
// (FindByID/List/Create/Update/Delete over the full struct).
func NewOrderRepository(store *sqladapter.Store[model.Order]) order.Repository {
	return pgxadapter.NewStoreRepo[model.Order](store)
}
