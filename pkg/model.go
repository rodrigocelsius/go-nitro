package nitro

import (
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Model is Nitro's default fields: UUID v7, UTC DateTime, and soft delete.
//
//	type Product struct {
//	    nitro.Model
//	    Name string `json:"name" db:"name"`
//	}
//
// If the user does not declare id/created_at/updated_at/deleted_at, embed Model
// (or let codegen inject columns in the migration). Soft delete stays enabled
// while DeletedAt exists on the type; use HardModel or //nitro:nosoftdelete
// for physical DELETE.
type Model struct {
	ID        UUID      `json:"id" db:"id"`
	CreatedAt DateTime  `json:"created_at" db:"created_at"`
	UpdatedAt DateTime  `json:"updated_at" db:"updated_at"`
	DeletedAt *DateTime `json:"deleted_at,omitempty" db:"deleted_at"`
}

// HardModel is UUID v7 + timestamps without soft delete (DELETE removes the row).
//
//	//nitro:nosoftdelete
//	type Product struct {
//	    nitro.HardModel
//	    Name string `json:"name" db:"name"`
//	}
type HardModel struct {
	ID        UUID     `json:"id" db:"id"`
	CreatedAt DateTime `json:"created_at" db:"created_at"`
	UpdatedAt DateTime `json:"updated_at" db:"updated_at"`
}

// SoftDeleteSQL returns "deleted_at IS NULL" (or the detected column) if T has soft delete.
// Soft delete ≠ business status (active/inactive) — filter status via Filterable.
func SoftDeleteSQL[T any]() string {
	return domain.SoftDeleteSQLOf[T]()
}

// SoftDeleteColumn returns the soft-delete column name, or "".
func SoftDeleteColumn(t reflect.Type) string {
	return domain.SoftDeleteColumn(t)
}
