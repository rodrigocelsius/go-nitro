package fasthttp

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestRegisterDashboard_ProductionRequiresMiddleware(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvProduction})
	RegisterDashboard(app, DashboardConfig{Enabled: true})
	if app.Err() == nil {
		t.Fatal("expected config error: dashboard in production without middleware")
	}
}

func TestRegisterDashboard_ProductionWithMiddlewareIsGuarded(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvProduction})
	RegisterDashboard(app, DashboardConfig{Enabled: true, Middleware: []domain.Handler{IPWhitelistMiddleware("10.9.9.9")}})
	resp, _ := app.Test("GET", "/nitro/api/dashboard", nil)
	if resp.StatusCode() != 403 {
		t.Fatalf("status=%d, want 403", resp.StatusCode())
	}
}

func TestRegisterDashboard_NoWarningWhenDisabled(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w

	app := New(domain.Config{Env: domain.EnvProduction})
	RegisterDashboard(app, DashboardConfig{Enabled: false})

	_ = w.Close()
	os.Stderr = orig
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if n > 0 {
		t.Fatalf("expected no warning when Enabled=false, got %q", string(buf[:n]))
	}
}

func TestDashboardDataHandler_MemoryStats(t *testing.T) {
	app := New()
	cfg := DashboardConfig{Enabled: true}

	handler := DashboardDataHandler(app, cfg)

	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	if err := handler(&c.Ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var data DashboardData
	if err := json.Unmarshal(c.fasthttpReq.Response.Body(), &data); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Check memory stats are populated
	if data.Memory.Alloc == 0 {
		t.Error("Memory.Alloc should be > 0")
	}
	if data.Memory.Sys == 0 {
		t.Error("Memory.Sys should be > 0")
	}
	if data.Memory.HeapAlloc == 0 {
		t.Error("Memory.HeapAlloc should be > 0")
	}

	// Check runtime stats
	if data.NumGoroutines <= 0 {
		t.Errorf("NumGoroutines = %d, want > 0", data.NumGoroutines)
	}
	if data.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if data.NumCPU <= 0 {
		t.Errorf("NumCPU = %d, want > 0", data.NumCPU)
	}
}

func TestDashboardDataHandler_MiddlewareMetrics(t *testing.T) {
	app := New()
	app.EnableMetrics()
	cfg := DashboardConfig{Enabled: true}

	handler := DashboardDataHandler(app, cfg)

	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	if err := handler(&c.Ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var data DashboardData
	if err := json.Unmarshal(c.fasthttpReq.Response.Body(), &data); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// No middleware registered yet
	if data.Middleware != nil && len(data.Middleware) > 0 {
		t.Error("expected no middleware metrics when none registered")
	}
}

func TestDashboardDataHandler_WithMetrics(t *testing.T) {
	app := New()
	mc := app.EnableMetrics()
	cfg := DashboardConfig{Enabled: true}

	// Register a route with a middleware
	app.Use(mc.Wrap("test-mw", func(c *domain.Ctx) error {
		return nil
	}))
	app.Get("/test", func(c *domain.Ctx) error {
		return nil
	})

	// Make a request through the handler
	handler := app.Handler()
	fctx := fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test")
	handler(&fctx)

	// Now check the dashboard shows the middleware
	dashHandler := DashboardDataHandler(app, cfg)
	c := app.pool.Get().(*FastCtx)
	c.Reset(&fctx)

	fctx2 := fasthttp.RequestCtx{}
	c.Reset(&fctx2)

	if err := dashHandler(&c.Ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var data DashboardData
	if err := json.Unmarshal(c.fasthttpReq.Response.Body(), &data); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if len(data.Middleware) == 0 {
		t.Error("expected middleware metrics")
	}
	if len(data.Middleware) > 0 && data.Middleware[0].Count != 1 {
		t.Errorf("middleware count = %d, want 1", data.Middleware[0].Count)
	}
}

func TestDashboardHTML(t *testing.T) {
	app := New()
	cfg := DashboardConfig{Enabled: true}

	handler := DashboardHandler(cfg)

	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	if err := handler(&c.Ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	body := string(c.fasthttpReq.Response.Body())
	if len(body) == 0 {
		t.Error("dashboard HTML should not be empty")
	}
}
