package model

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//nitro:table=products
type Product struct {
	nitro.Model
	Name  string  `json:"name" db:"name" validate:"required,min=1" blinq:"index"`
	Price float64 `json:"price" db:"price" validate:"required,gt=0" blinq:"index"`
}
