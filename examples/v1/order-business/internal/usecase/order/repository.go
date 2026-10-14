// Package order holds the Order business logic: one file per Command/Query.
package order

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
)

// Repository is the persistence port for single-table commands (Submit,
// Cancel, Pending). Place and Approve write orders + invoices atomically via
// *sqladapter.DB.Tx instead — see place.go and approve.go.
//
// Satisfied by pgxadapter.NewStoreRepo(store) in production
// (internal/repository) and by nitro.NewMemoryRepo[model.Order]() in tests.
type Repository = nitro.TypedRepository[model.Order]
