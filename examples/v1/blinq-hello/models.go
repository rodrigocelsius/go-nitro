package main

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

//nitro:table=hellos
type Hello struct {
	nitro.Model
	Message string `json:"message" db:"message" validate:"required,min=1"`
}
