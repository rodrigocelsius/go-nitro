package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestConventions(t *testing.T) {
	app := buildApp()

	r, err := app.TestWithHeaders("POST", "/products", strings.NewReader(`{"name":"Pen","unit_price":2.5}`),
		map[string]string{"Content-Type": "application/json"})
	if err != nil || r.StatusCode() != 201 {
		t.Fatalf("create: %v %d %s", err, r.StatusCode(), r.Body())
	}
	if !strings.Contains(string(r.Body()), `"unit_price":2.5`) {
		t.Fatalf("wire name must be snake_case: %s", r.Body())
	}

	g, _ := app.Test("GET", "/greet/Ada?shout=true", nil)
	if g.StatusCode() != 200 || !strings.Contains(string(g.Body()), "HELLO, ADA!") {
		t.Fatalf("typed handler: %d %s", g.StatusCode(), g.Body())
	}

	if a, _ := app.Test("GET", "/admin/stats", nil); a.StatusCode() != 401 {
		t.Fatalf("basic auth must reject anonymous: %d", a.StatusCode())
	}
	ok := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	if a, _ := app.TestWithHeaders("GET", "/admin/stats", nil, map[string]string{"Authorization": ok}); a.StatusCode() != 200 {
		t.Fatalf("basic auth: %d", a.StatusCode())
	}
	if a, _ := app.TestWithHeaders("GET", "/internal/ping", nil, map[string]string{"X-API-Key": "dev-key"}); a.StatusCode() != 200 {
		t.Fatalf("api key: %d", a.StatusCode())
	}
	if h, _ := app.Test("GET", "/health", nil); h.StatusCode() != 200 {
		t.Fatalf("skip: %d", h.StatusCode())
	}
}
