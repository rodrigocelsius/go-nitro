package model

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

//go:generate nitrogen make invoice.go --data

//nitro:table=invoices
type Invoice struct {
	nitro.Model
	OrderID nitro.UUID `json:"order_id" db:"order_id" blinq:"index"`
	Amount  float64    `json:"amount" db:"amount"`
	Status  string     `json:"status" db:"status" default:"open"` // open | charged
}
