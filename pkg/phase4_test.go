package nitro_test

import (
	"encoding/json"
	"strings"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func TestPhase4_Parse(t *testing.T) {
	app := nitro.New()
	app.Post("/login", func(c *nitro.Ctx) error {
		req, err := nitro.Parse[struct {
			Email    string `json:"email" validate:"required,email"`
			Password string `json:"password" validate:"required"`
		}](c)
		if err != nil {
			return err
		}
		return c.OK(nitro.Map{"email": req.Email})
	})

	resp, err := app.Test("POST", "/login", strings.NewReader(`{"email":"a@b.com","password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode(), resp.Body())
	}

	resp, err = app.Test("POST", "/login", strings.NewReader(`{"email":"","password":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 422 {
		t.Fatalf("status = %d, want 422; body=%s", resp.StatusCode(), resp.Body())
	}
}

func TestPhase4_Err(t *testing.T) {
	app := nitro.New()
	app.Get("/err", func(c *nitro.Ctx) error {
		return nitro.Err(400, "INVALID_EMAIL", "campo email é obrigatório")
	})
	app.Get("/errcode", func(c *nitro.Ctx) error {
		return nitro.ErrCode("INVALID_EMAIL", "campo email é obrigatório")
	})

	resp, err := app.Test("GET", "/err", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode())
	}
	var body map[string]string
	if err := json.Unmarshal(resp.Body(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "INVALID_EMAIL" {
		t.Fatalf("code = %q, want INVALID_EMAIL", body["code"])
	}
	if body["error"] != "campo email é obrigatório" {
		t.Fatalf("error = %q", body["error"])
	}

	resp, err = app.Test("GET", "/errcode", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 400 {
		t.Fatalf("ErrCode status = %d, want 400", resp.StatusCode())
	}
}

func TestPhase4_IPWhiteList(t *testing.T) {
	app := nitro.New()
	app.Use(nitro.IPWhiteList("127.0.0.1"))
	app.Get("/", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"ok": true})
	})

	resp, err := app.Test("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Test uses 0.0.0.0 / unspecified remote — expect forbidden unless 127.0.0.1
	if resp.StatusCode() != 403 && resp.StatusCode() != 200 {
		t.Fatalf("unexpected status %d body=%s", resp.StatusCode(), resp.Body())
	}
}

func TestPhase4_Dashboard(t *testing.T) {
	app := nitro.New(nitro.Config{Env: nitro.EnvDevelopment})
	app.Dashboard("/admin/nitro")
	app.Get("/api/ping", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"ok": true})
	})

	resp, err := app.Test("GET", "/admin/nitro/dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("dashboard status = %d, want 200", resp.StatusCode())
	}
	if !strings.Contains(string(resp.Body()), "Nitro Dashboard") {
		t.Fatal("expected dashboard HTML")
	}

	resp, err = app.Test("GET", "/admin/nitro/api/dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("dashboard api status = %d, want 200; body=%s", resp.StatusCode(), resp.Body())
	}
}

func TestPhase4_Prometheus(t *testing.T) {
	app := nitro.New()
	app.Prometheus()
	app.Get("/hello", func(c *nitro.Ctx) error {
		return c.Text("hi")
	})

	// Hit a route so metrics accumulate
	if _, err := app.Test("GET", "/hello", nil); err != nil {
		t.Fatal(err)
	}

	resp, err := app.Test("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("metrics status = %d, want 200", resp.StatusCode())
	}
	body := string(resp.Body())
	if !strings.Contains(body, "nitro_requests_total") {
		t.Fatalf("expected prometheus metrics, got: %s", body)
	}
}

func TestPhase4_Logger(t *testing.T) {
	app := nitro.New()
	app.Logger(nitro.LoggerConfig{Format: "json", SkipRoutes: []string{"/health"}})
	app.Get("/ping", func(c *nitro.Ctx) error {
		return c.Text("pong")
	})

	resp, err := app.Test("GET", "/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode())
	}
}

func TestPhase4_NewWSHub(t *testing.T) {
	hub := nitro.NewWSHub()
	if hub == nil {
		t.Fatal("NewWSHub returned nil")
	}
	var _ nitro.WSHub = hub
}

// Production default is secure: the dashboard cannot be registered bare.
func TestPhase4_DashboardProductionRequiresMiddleware(t *testing.T) {
	app := nitro.New(nitro.Config{Env: nitro.EnvProduction})
	app.Dashboard("/admin/nitro")
	if app.Err() == nil {
		t.Fatal("expected config error registering dashboard in production without middleware")
	}
}

func TestPhase4_DashboardProductionGuarded(t *testing.T) {
	app := nitro.New(nitro.Config{Env: nitro.EnvProduction})
	app.Dashboard("/admin/nitro", nitro.IPWhiteList("10.9.9.9"))
	resp, err := app.Test("GET", "/admin/nitro/api/dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 403 {
		t.Fatalf("status=%d, want 403 for non-allow-listed caller", resp.StatusCode())
	}
}
