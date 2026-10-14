package fasthttp

import (
	"errors"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// --- 15.1 Test Helper ---

func TestApp_Test_GET(t *testing.T) {
	app := New()
	app.Get("/hello", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("world"))
		return nil
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

func TestApp_Test_POST(t *testing.T) {
	app := New()
	app.Post("/data", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		fc.SetBody(append([]byte("received: "), fc.Body()...))
		return nil
	})

	body := strings.NewReader(`{"name":"test"}`)
	resp, err := app.Test("POST", "/data", body)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode())
	}
	if !strings.Contains(string(resp.Body()), "received:") {
		t.Errorf("Body = %q, should contain 'received:'", string(resp.Body()))
	}
}

func TestApp_Test_Param(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		fc.SetBody([]byte(c.Param("id")))
		return nil
	})

	resp, err := app.Test("GET", "/users/42", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if string(resp.Body()) != "42" {
		t.Errorf("Body = %q, want 42", string(resp.Body()))
	}
}

func TestApp_Test_NotFound(t *testing.T) {
	app := New()
	app.Get("/exists", func(c *domain.Ctx) error {
		return nil
	})

	resp, err := app.Test("GET", "/notfound", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if resp.StatusCode() != 404 {
		t.Errorf("StatusCode = %d, want 404", resp.StatusCode())
	}
}

func TestApp_Test_MethodMismatch(t *testing.T) {
	app := New()
	app.Get("/onlyget", func(c *domain.Ctx) error {
		return nil
	})

	resp, err := app.Test("POST", "/onlyget", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if resp.StatusCode() != 405 {
		t.Errorf("StatusCode = %d, want 405", resp.StatusCode())
	}
}

// --- 15.2 Error Handler ---

func TestCustomErrorHandler(t *testing.T) {
	customHandler := func(c *domain.Ctx, err error) error {
		c.SetStatus(400)
		c.SetHeader("Content-Type", "application/json")
		c.SetBody([]byte(`{"custom":true,"msg":"` + err.Error() + `"}`))
		return nil
	}

	app := New(domain.Config{ErrorHandler: customHandler})
	app.Get("/fail", func(c *domain.Ctx) error {
		return errors.New("something went wrong")
	})

	resp, err := app.Test("GET", "/fail", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if resp.StatusCode() != 400 {
		t.Errorf("StatusCode = %d, want 400", resp.StatusCode())
	}
	body := string(resp.Body())
	if !strings.Contains(body, `"custom":true`) {
		t.Errorf("Body = %q, want custom error format", body)
	}
	if !strings.Contains(body, `"msg":"something went wrong"`) {
		t.Errorf("Body = %q, should contain error message", body)
	}
}

func TestCustomErrorHandler_HTTTPError(t *testing.T) {
	customHandler := func(c *domain.Ctx, err error) error {
		// Log the error, then use fallback
		c.SetHeader("X-Error-Logged", "true")
		return err
	}

	app := New(domain.Config{ErrorHandler: customHandler})
	app.Get("/unauthorized", func(c *domain.Ctx) error {
		return &domain.UnauthorizedError{Message: "invalid token"}
	})

	resp, err := app.Test("GET", "/unauthorized", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if resp.StatusCode() != 401 {
		t.Errorf("StatusCode = %d, want 401", resp.StatusCode())
	}
	if string(resp.Header.Peek("X-Error-Logged")) != "true" {
		t.Error("X-Error-Logged header not set by custom handler")
	}
}

func TestDefaultErrorHandler(t *testing.T) {
	app := New()
	app.Get("/fail", func(c *domain.Ctx) error {
		return errors.New("internal failure")
	})

	resp, err := app.Test("GET", "/fail", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if resp.StatusCode() != 500 {
		t.Errorf("StatusCode = %d, want 500", resp.StatusCode())
	}
	body := string(resp.Body())
	if !strings.Contains(body, "internal server error") {
		t.Errorf("Body = %q, want default error format", body)
	}
}

// --- 15.3 Map (type alias, tested by compilation) ---

func Test_Map_Integration(t *testing.T) {
	app := New()
	app.Get("/map", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.JSON(200, map[string]any{"key": "value"})
	})

	resp, err := app.Test("GET", "/map", nil)
	if err != nil {
		t.Fatalf("Test error: %v", err)
	}
	if string(resp.Body()) != `{"key":"value"}` {
		t.Errorf("Body = %q, want JSON", string(resp.Body()))
	}
}

func BenchmarkApp_Test(b *testing.B) {
	app := New()
	app.Get("/bench", func(c *domain.Ctx) error {
		return nil
	})

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		app.Test("GET", "/bench", nil)
	}
}
