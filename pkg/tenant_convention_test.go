package nitro

import (
	"strings"
	"testing"
)

type tcCustomer struct {
	TenantModel
	Name string `json:"name" db:"name"`
}

type tcRef struct {
	TenantModel
	Code string `json:"code" db:"code"`
}

func tcApp(current *string, cfg *ResourceConfig) *App {
	app := New(Config{DisableDefaults: true})
	setTenant := func(c *Ctx) error {
		if *current != "" {
			c.SetTenantID(*current)
		}
		return nil
	}
	if cfg == nil {
		cfg = &ResourceConfig{}
	}
	cfg.Middleware = append(cfg.Middleware, setTenant)
	Resource(app, "/customers", NewMemoryRepo[tcCustomer](), cfg)
	return app
}

// CoC: a model with a tenant column is tenant-scoped with zero configuration,
// and a request without a tenant is rejected (fail-closed).
func TestTenantConvention_AutoScopeFailClosed(t *testing.T) {
	var cur string
	app := tcApp(&cur, nil)

	resp, _ := app.Test("POST", "/customers", strings.NewReader(`{"name":"X"}`))
	if resp.StatusCode() != 401 {
		t.Fatalf("create without tenant must be 401, got %d %s", resp.StatusCode(), resp.Body())
	}
	resp, _ = app.Test("GET", "/customers", nil)
	if !strings.Contains(string(resp.Body()), `"total":0`) {
		t.Fatalf("list without tenant must be empty, got %d %s", resp.StatusCode(), resp.Body())
	}

	cur = "11111111-1111-1111-1111-111111111111"
	resp, _ = app.Test("POST", "/customers", strings.NewReader(`{"name":"Ana"}`))
	if resp.StatusCode() != 201 {
		t.Fatalf("create as tenant A: %d %s", resp.StatusCode(), resp.Body())
	}
	resp, _ = app.Test("GET", "/customers", nil)
	if resp.StatusCode() != 200 || !strings.Contains(string(resp.Body()), "Ana") {
		t.Fatalf("tenant A list: %d %s", resp.StatusCode(), resp.Body())
	}

	cur = "22222222-2222-2222-2222-222222222222"
	resp, _ = app.Test("GET", "/customers", nil)
	if strings.Contains(string(resp.Body()), "Ana") {
		t.Fatalf("tenant B must not see tenant A data: %s", resp.Body())
	}
}

func TestTenantConvention_OptOutAndNonTenantModels(t *testing.T) {
	var cur string
	app := tcApp(&cur, &ResourceConfig{NoTenantScope: true})
	resp, _ := app.Test("GET", "/customers", nil)
	if resp.StatusCode() != 200 {
		t.Fatalf("NoTenantScope must not require a tenant, got %d", resp.StatusCode())
	}
	if modelHasColumn(&resItemNoTenant{}, "tenant_id") {
		t.Fatal("plain model must not be detected as tenant model")
	}
	if !modelHasColumn(&tcRef{}, "tenant_id") {
		t.Fatal("embedded TenantModel must be detected")
	}
}

type resItemNoTenant struct {
	Model
	Title string `json:"title"`
}
