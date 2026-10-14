package main

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func buildApp() *nitro.App {
	v := nitro.NewValidator()
	nitro.RegisterBRValidators(v)
	app := nitro.New(nitro.Config{DisableDefaults: true, Validator: v})
	app.Post("/users", createUser)
	app.Get("/", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"service": "nitro-v1-validation"})
	})
	return app
}

func TestCreateUser_Empty(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/users", `{}`)
	tc.AssertStatus(t, resp, 422)
}

func TestCreateUser_BadCPFAndPasswordMismatch(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/users", `{"cpf":"000","password":"a","confirm":"b"}`)
	tc.AssertStatus(t, resp, 422)
	tc.AssertBodyContains(t, resp, "cpf")
}

func TestCreateUser_OK(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/users", `{"cpf":"529.982.247-25","password":"secret","confirm":"secret"}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, "529.982.247-25")
}

func TestCreateUser_SameMismatch(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/users", `{"cpf":"529.982.247-25","password":"secret","confirm":"other"}`)
	tc.AssertStatus(t, resp, 422)
	tc.AssertBodyContains(t, resp, "As senhas não conferem")
}
