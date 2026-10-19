package nitro

import (
	"context"
	"testing"
)

type hookItem struct {
	Model
	Name string `json:"name"`
}

func TestResourceHooksReceiveRequest(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	var gotTenant, gotHeader string
	var onMutateOK bool
	app.Use(func(c *Ctx) error { c.SetTenantID("t-42"); return nil })
	app.Resource("/hook_items", &hookItem{}, AdaptRepo(NewMemoryRepo[hookItem]()), &ResourceConfig{
		NoCache:       true,
		Public:        true,
		NoTenantScope: true,
		BeforeCreate: func(ctx context.Context, _ any) error {
			if c, ok := RequestFrom(ctx); ok {
				gotTenant = c.TenantID()
				gotHeader = c.Header("X-Trace")
			}
			return nil
		},
		OnMutate: func(ctx context.Context, _ string, _ any) error {
			_, onMutateOK = RequestFrom(ctx)
			return nil
		},
	})
	tc := NewTestClient(app).WithHeader("X-Trace", "abc")
	tc.AssertStatus(t, tc.Post(t, "/hook_items", `{"name":"x"}`), 201)
	if gotTenant != "t-42" || gotHeader != "abc" || !onMutateOK {
		t.Fatalf("hooks did not see the request: tenant=%q header=%q onMutate=%v", gotTenant, gotHeader, onMutateOK)
	}
}

func TestRequestFromPlainContext(t *testing.T) {
	if _, ok := RequestFrom(context.Background()); ok {
		t.Fatal("background context carries no request")
	}
}
