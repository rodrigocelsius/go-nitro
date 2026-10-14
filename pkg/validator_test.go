package nitro_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
	"github.com/valyala/fasthttp"
)

func TestAppValidator_ScopedCPF(t *testing.T) {
	vA := nitro.NewValidator()
	vA.Register("cpf", func(ctx nitro.FieldContext, param string) error {
		s, _ := ctx.FieldValue.(string)
		if s != "ok" {
			return &domain.ValidationError{Field: ctx.Path, Message: "invalid cpf", Rule: "cpf"}
		}
		return nil
	})

	appA := nitro.New(nitro.Config{DisableDefaults: true, Validator: vA})
	appB := nitro.New(nitro.Config{DisableDefaults: true}) // no cpf

	type User struct {
		CPF string `json:"cpf" validate:"cpf"`
	}

	appA.Post("/u", func(c *nitro.Ctx) error {
		var u User
		if err := c.BodyJSON(&u); err != nil {
			return err
		}
		return c.OK(u)
	})
	appB.Post("/u", func(c *nitro.Ctx) error {
		var u User
		if err := c.BodyJSON(&u); err != nil {
			return err
		}
		return c.OK(u)
	})

	// App A: invalid cpf → 422
	code, body := postJSON(t, appA, "/u", `{"cpf":"bad"}`)
	if code != 422 {
		t.Fatalf("appA bad cpf: %d %s", code, body)
	}

	// App A: cpf ok → 200
	code, body = postJSON(t, appA, "/u", `{"cpf":"ok"}`)
	if code != 200 {
		t.Fatalf("appA ok cpf: %d %s", code, body)
	}

	// App B: cpf tag without registration → unknown rule → 422
	code, body = postJSON(t, appB, "/u", `{"cpf":"ok"}`)
	if code != 422 || !bytes.Contains(body, []byte("unknown")) {
		t.Fatalf("appB expected unknown rule: %d %s", code, body)
	}
}

func TestAppSetValidator(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	v := nitro.NewValidator()
	v.Register("sku", func(ctx nitro.FieldContext, param string) error {
		return errors.New("bad sku")
	})
	app.SetValidator(v)

	type Item struct {
		SKU string `json:"sku" validate:"sku"`
	}
	app.Post("/i", func(c *nitro.Ctx) error {
		var it Item
		if err := c.BodyJSON(&it); err != nil {
			return err
		}
		return c.OK(it)
	})

	code, _ := postJSON(t, app, "/i", `{"sku":"x"}`)
	if code != 422 {
		t.Fatalf("got %d", code)
	}
}

func postJSON(t *testing.T, app *nitro.App, path, jsonBody string) (int, []byte) {
	t.Helper()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("POST")
	fctx.Request.SetRequestURI(path)
	fctx.Request.Header.SetContentType("application/json")
	fctx.Request.SetBodyString(jsonBody)
	app.Handler()(fctx)
	return fctx.Response.StatusCode(), append([]byte(nil), fctx.Response.Body()...)
}
