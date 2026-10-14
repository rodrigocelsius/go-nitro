package main

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// Contact demonstrates transform tags on bind (Parse / BodyJSON).
type Contact struct {
	Name  string `json:"name" validate:"required,min=3" transform:"trim,titlecase=pt"`
	Phone string `json:"phone" validate:"required,min=10" transform:"digits"`
	UF    string `json:"uf" validate:"required,len=2" transform:"trim,upper"`
}

func main() {
	app := nitro.New()

	app.Post("/contacts", func(c *nitro.Ctx) error {
		in, err := nitro.Parse[Contact](c)
		if err != nil {
			return err
		}
		return c.Created(in)
	})

	nitro.Ok(app.Listen(":8080"))
}
