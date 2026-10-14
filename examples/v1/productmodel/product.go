// Package productmodel is the shared model for the DB examples
// (db-blinq, db-linq, db-blinq-handlers). BLinq DSL: productmodel/modelgen.
package productmodel

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

//go:generate nitrogen make product.go --data

// Product is the default Nitro domain: UUID v7 + timestamps + soft delete.
//
//nitro:table=products
type Product struct {
	nitro.Model
	Name   string  `json:"name" db:"name" validate:"required,min=2" blinq:"index"`
	Price  float64 `json:"price" db:"price" validate:"gte=0" blinq:"index"`
	Active bool    `json:"active" db:"active" default:"true" blinq:"index"`
}
