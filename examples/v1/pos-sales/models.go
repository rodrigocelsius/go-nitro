package main

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

//go:generate nitrogen make models.go --data

// Sale is the header of a point-of-sale transaction. Items is the nested
// input accepted by POST /sales (sqladapter.Related binds it, then discards
// it — db:"-" — since line items live in their own table).
//
//nitro:table=sales
type Sale struct {
	nitro.Model
	CustomerName string     `json:"customer_name" db:"customer_name" validate:"required" blinq:"index"`
	Status       string     `json:"status" db:"status" default:"open" blinq:"index" validate:"omitempty,oneof=open voided"`
	Total        float64    `json:"total" db:"total"`
	Items        []SaleItem `json:"items" db:"-" validate:"omitempty,dive"`
}

// SaleItem is a line item of a Sale. SaleID is the FK sqladapter.Related
// expects by convention (snake_case(Sale) + "_id").
//
//nitro:table=sale_items
type SaleItem struct {
	nitro.Model
	SaleID    nitro.UUID `json:"sale_id" db:"sale_id"`
	SKU       string     `json:"sku" db:"sku" validate:"required"`
	Quantity  int        `json:"quantity" db:"quantity" validate:"required,gt=0"`
	UnitPrice float64    `json:"unit_price" db:"unit_price" validate:"required,gt=0"`
	Subtotal  float64    `json:"subtotal" db:"subtotal"`
}
