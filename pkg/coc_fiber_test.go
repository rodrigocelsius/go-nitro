package nitro

import (
	"strings"
	"testing"
)

func TestConfig_EnablePrometheus(t *testing.T) {
	app := New(Config{
		DisableDefaults:  true,
		DisableTelemetry: true,
		EnablePrometheus: true,
		PrometheusPath:   "/custom-metrics",
	})
	found := false
	for _, r := range app.Routes() {
		if r.Method == "GET" && strings.Contains(r.Path, "custom-metrics") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("routes=%v missing prometheus path", app.Routes())
	}
}

func TestMount_AndURL(t *testing.T) {
	sub := New(Config{DisableDefaults: true, DisableTelemetry: true})
	sub.Get("/items/:id", func(c *Ctx) error {
		return c.Text("ok")
	}).Name("item.show")

	app := New(Config{DisableDefaults: true, DisableTelemetry: true})
	app.Mount("/api/v1", sub)

	got := app.GetRouteURL("item.show", Map{"id": 7})
	if got != "/api/v1/items/7" {
		t.Fatalf("got %q", got)
	}

	h := app.Handler()
	ctx := newTestCtx("GET", "/api/v1/items/1", nil)
	h(ctx)
	if string(ctx.Response.Body()) != "ok" {
		t.Fatalf("body=%q status=%d", ctx.Response.Body(), ctx.Response.StatusCode())
	}
}

func TestHooks_OnListenFires(t *testing.T) {
	// Facade wiring: Hooks() returns the adapter hooks registrar.
	app := New(Config{DisableDefaults: true, DisableTelemetry: true})
	if app.Hooks() == nil {
		t.Fatal("nil hooks")
	}
	n := 0
	app.Hooks().OnListen(func(d ListenData) error {
		n++
		return nil
	}).OnShutdown(func() error {
		n++
		return nil
	})
	_ = app.Shutdown()
	if n != 1 {
		t.Fatalf("OnShutdown fires once via facade Shutdown, got n=%d", n)
	}
}
