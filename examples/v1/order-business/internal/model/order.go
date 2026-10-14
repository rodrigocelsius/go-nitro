package model

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

//go:generate nitrogen make order.go --data

//nitro:table=orders
type Order struct {
	nitro.Model
	CustomerID string  `json:"customer_id" db:"customer_id" validate:"required" blinq:"index"`
	Status     string  `json:"status" db:"status" default:"draft" blinq:"index" validate:"omitempty,oneof=draft pending approved cancelled"`
	Total      float64 `json:"total" db:"total" validate:"gte=0"`
	Note       string  `json:"note,omitempty" db:"note"`
}
