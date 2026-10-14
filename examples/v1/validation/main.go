// Validation example — RegisterBRValidators (cpf/cnpj/phone_br/cep/uf) + alias same=.
//
// Run:
//
//	go run .
//
// Smoke:
//
//	curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' -d '{}'
//	curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' \
//	  -d '{"cpf":"000","password":"a","confirm":"b"}'
//	curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' \
//	  -d '{"cpf":"529.982.247-25","password":"secret","confirm":"secret"}'
package main

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type User struct {
	CPF      string `json:"cpf" validate:"required,cpf" msg_cpf:"Informe um CPF válido"`
	Password string `json:"password" validate:"required,min=3"`
	Confirm  string `json:"confirm" validate:"required,same=Password" msg_same:"As senhas não conferem"`
}

func main() {
	v := nitro.NewValidator()
	nitro.RegisterBRValidators(v)

	app := nitro.New(nitro.Config{Validator: v})

	app.Post("/users", createUser)

	app.Get("/", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{
			"service": "nitro-v1-validation",
			"hint":    "POST /users with cpf, password, confirm",
		})
	})

	nitro.Ok(app.Listen(":8080"))
}

func createUser(c *nitro.Ctx) error {
	user, err := nitro.Parse[User](c)
	if err != nil {
		return err
	}
	return c.Created(nitro.Map{
		"cpf": user.CPF,
		"ok":  true,
	})
}
