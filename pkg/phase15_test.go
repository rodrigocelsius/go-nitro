package nitro

import (
	"errors"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// --- 15.1 Test Helper (facade) ---

func TestFacade_Test_GET(t *testing.T) {
	app := New()
	app.Get("/hello", func(c *Ctx) error {
		return c.SendString("world")
	})

	resp, err := app.Test("GET", "/hello", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode())
	}
	if string(resp.Body()) != "world" {
		t.Errorf("Body = %q, want world", string(resp.Body()))
	}
}

func TestFacade_Test_POST(t *testing.T) {
	app := New()
	app.Post("/data", func(c *Ctx) error {
		return c.SendString("received: " + string(c.Body()))
	})

	body := strings.NewReader(`{"name":"test"}`)
	resp, err := app.Test("POST", "/data", body)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if !strings.Contains(string(resp.Body()), "received:") {
		t.Errorf("Body = %q, should contain 'received:'", string(resp.Body()))
	}
}

func TestFacade_Test_Param(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error {
		return c.SendString(c.Param("id"))
	})

	resp, err := app.Test("GET", "/users/42", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if string(resp.Body()) != "42" {
		t.Errorf("Body = %q, want 42", string(resp.Body()))
	}
}

// --- 15.2 Error Handler (facade) ---

func TestFacade_CustomErrorHandler(t *testing.T) {
	app := New(Config{
		ErrorHandler: func(c *domain.Ctx, err error) error {
			c.SetBody([]byte(`{"custom":true}`))
			return nil
		},
	})
	app.Get("/fail", func(c *Ctx) error {
		return errors.New("fail")
	})

	resp, err := app.Test("GET", "/fail", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if string(resp.Body()) != `{"custom":true}` {
		t.Errorf("Body = %q, want custom error", string(resp.Body()))
	}
}

// --- 15.3 Map Helper ---

func TestFacade_Map(t *testing.T) {
	app := New()
	app.Get("/map", func(c *Ctx) error {
		return c.JSON(200, Map{"key": "value", "num": 1})
	})

	resp, err := app.Test("GET", "/map", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	body := string(resp.Body())
	if !strings.Contains(body, `"key":"value"`) {
		t.Errorf("Body = %q, want JSON with key:value", body)
	}
	if !strings.Contains(body, `"num":1`) {
		t.Errorf("Body = %q, want JSON with num:1", body)
	}
}
