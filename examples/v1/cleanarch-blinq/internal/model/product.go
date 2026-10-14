package model

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//go:generate nitrogen make product.go --data

//nitro:table=products
type Product struct {
	nitro.Model
	Name   string  `json:"name" db:"name" validate:"required" blinq:"index"`
	Price  float64 `json:"price" db:"price" validate:"gte=0"`
	Active bool    `json:"active" db:"active" blinq:"index" default:"true"`
}
