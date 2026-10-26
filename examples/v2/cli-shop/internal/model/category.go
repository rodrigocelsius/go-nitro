package model

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//go:generate nitrogen make category.go --data

//nitro:table=categories
type Category struct {
	nitro.Model
	Name string `json:"name" db:"name" validate:"required" blinq:"index"`
}
