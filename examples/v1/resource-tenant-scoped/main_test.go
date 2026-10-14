package main

import (
	"encoding/json"
	"strings"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func TestTenantScope_IsolatesCRUD(t *testing.T) {
	orders = nitro.NewMemoryRepo[Order]()
	tenantA := "01900000-0000-7000-8000-000000000001"
	tenantB := "01900000-0000-7000-8000-000000000002"

	app := nitro.New(nitro.Config{DisableDefaults: true})
	allowHeader := true // test only: production resolves the tenant from the JWT claim
	app.Use(nitro.TenantMiddleware(nitro.TenantConfig{Require: true, AllowHeader: &allowHeader}))
	// No TenantScope: the tenant_id column makes the Resource scoped by convention.
	app.Resource(&Order{}, nitro.AdaptRepo(orders), &nitro.ResourceConfig{
		NoCache: true,
		Public:  true,
	})

	tc := nitro.NewTestClient(app)
	a := tc.WithHeader("X-Tenant-ID", tenantA)
	b := tc.WithHeader("X-Tenant-ID", tenantB)

	createA := a.Post(t, "/orders", `{"title":"a1"}`)
	tc.AssertStatus(t, createA, 201)
	idA := extractID(t, createA.Body())

	// Spoofing tenant_id in JSON must not attach the row to tenant A.
	createB := b.Post(t, "/orders", `{"title":"b1","tenant_id":"`+tenantA+`"}`)
	tc.AssertStatus(t, createB, 201)

	listB := b.Get(t, "/orders")
	tc.AssertStatus(t, listB, 200)
	tc.AssertBodyContains(t, listB, `"title":"b1"`)
	if strings.Contains(string(listB.Body()), idA) {
		t.Fatalf("tenant B list must not include tenant A id %s: %s", idA, listB.Body())
	}

	tc.AssertStatus(t, b.Get(t, "/orders/"+idA), 404)

	getOwn := a.Get(t, "/orders/"+idA)
	tc.AssertStatus(t, getOwn, 200)
	tc.AssertBodyContains(t, getOwn, `"title":"a1"`)
}

func extractID(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	id, _ := m["id"].(string)
	if id == "" {
		t.Fatalf("missing id in %s", body)
	}
	return id
}
