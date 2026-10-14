package fasthttp

import (
	"os"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// In production pprof must be protected: registering it bare records a
// configuration error (Listen/Test refuse to run), and the provided
// middleware actually guards the routes.
func TestSetupPprof_ProductionRequiresMiddleware(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvProduction})
	SetupPprof(app)
	if app.Err() == nil {
		t.Fatal("expected config error: SetupPprof in production without middleware")
	}
	if err := app.Listen("127.0.0.1:0"); err == nil {
		t.Fatal("Listen must refuse to start with a config error")
	}
	if _, err := app.Test("GET", "/debug/pprof/", nil); err == nil {
		t.Fatal("Test must surface the config error")
	}
}

func TestSetupPprof_ProductionWithMiddlewareIsGuarded(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvProduction})
	SetupPprof(app, IPWhitelistMiddleware("10.9.9.9"))
	resp, err := app.Test("GET", "/debug/pprof/cmdline", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 403 {
		t.Fatalf("status=%d, want 403 for a non-whitelisted caller", resp.StatusCode())
	}
}

func TestPprof_RejectsLongCapture(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvDevelopment})
	SetupPprof(app)
	resp, _ := app.Test("GET", "/debug/pprof/profile?seconds=3600", nil)
	if resp.StatusCode() != 400 {
		t.Fatalf("status=%d, want 400 for seconds>30", resp.StatusCode())
	}
}

func TestSetupPprof_NoWarningInDevelopment(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w

	app := New(domain.Config{Env: domain.EnvDevelopment})
	SetupPprof(app)

	_ = w.Close()
	os.Stderr = orig
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if n > 0 {
		t.Fatalf("expected no warning outside production, got %q", string(buf[:n]))
	}
}

func TestPprofHandler_Index(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvDevelopment})
	SetupPprof(app)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", ctx.Response.StatusCode())
	}

	body := string(ctx.Response.Body())
	if len(body) == 0 {
		t.Error("Empty response body")
	}
}

func TestPprofHandler_Cmdline(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvDevelopment})
	SetupPprof(app)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/cmdline")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", ctx.Response.StatusCode())
	}
}

func TestPprofHandler_Symbol(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvDevelopment})
	SetupPprof(app)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/symbol")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", ctx.Response.StatusCode())
	}
}

func TestPprofHandler_Goroutine(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvDevelopment})
	SetupPprof(app)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/goroutine")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", ctx.Response.StatusCode())
	}

	body := string(ctx.Response.Body())
	if len(body) == 0 {
		t.Error("goroutine profile should not be empty")
	}
}

func TestPprofHandler_Heap(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvDevelopment})
	SetupPprof(app)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/heap")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", ctx.Response.StatusCode())
	}
}

func TestSetupPprof(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvDevelopment})
	SetupPprof(app)

	routes := app.Routes()
	if len(routes) < 10 {
		t.Errorf("SetupPprof registered %d routes, want >= 10", len(routes))
	}

	// Verify key routes exist
	paths := make(map[string]bool)
	for _, r := range routes {
		paths[r.Path] = true
	}

	for _, expected := range []string{"/debug/pprof", "/debug/pprof/heap", "/debug/pprof/goroutine", "/debug/pprof/profile"} {
		if !paths[expected] {
			t.Errorf("SetupPprof missing route %s", expected)
		}
	}
}

func BenchmarkPprofHandler(b *testing.B) {
	app := New(domain.Config{Env: domain.EnvDevelopment})
	SetupPprof(app)

	handler := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/debug/pprof/goroutine")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler(ctx)
	}
}
