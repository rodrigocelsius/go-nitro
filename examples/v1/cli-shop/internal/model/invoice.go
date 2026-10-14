package model

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//go:generate nitrogen make invoice.go --data

//nitro:table=invoices
type Invoice struct {
	nitro.Model
	Total    float64 `json:"total" db:"total" validate:"gte=0"`
	Customer string  `json:"customer" db:"customer" validate:"required"`
	TenantID string  `json:"tenant_id" db:"tenant_id" blinq:"index"`
}
