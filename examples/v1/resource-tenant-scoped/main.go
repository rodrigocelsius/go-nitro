// Shared-schema multi-tenant Resource — TenantMiddleware + the tenant_id convention (auto scope).
//
//	go run .
//
//	# Tenant A creates an order
//	curl -X POST localhost:8080/orders -H 'Content-Type: application/json' \
//	  -H 'X-Tenant-ID: 01900000-0000-7000-8000-000000000001' \
//	  -d '{"title":"order-a"}'
//
//	# Tenant B cannot list or get Tenant A's rows
//	curl localhost:8080/orders -H 'X-Tenant-ID: 01900000-0000-7000-8000-000000000002'
//
// Prefer JWT claim tenant_id in production (set User.TenantID at login) and
// TenantMiddleware after JWTMiddleware — see docs/guide/resource-security.md.
package main

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type Order struct {
	nitro.TenantModel
	Title string `json:"title" validate:"required,min=1"`
}

var orders = nitro.NewMemoryRepo[Order]()

func main() {
	app := nitro.New(nitro.Config{DisableDefaults: true})

	// Resolve tenant from the JWT claim (default, safe). This demo has no auth,
	// so it opts in to the X-Tenant-ID header explicitly. NEVER do this for
	// untrusted clients: they could read/write any tenant's data.
	allowHeader := true
	app.Use(nitro.TenantMiddleware(nitro.TenantConfig{Require: true, AllowHeader: &allowHeader}))

	// Convention: Order embeds TenantModel (a tenant_id column), so the
	// Resource is tenant-scoped automatically — no TenantScope needed.
	// Opt out with NoTenantScope: true.
	app.Resource(&Order{}, nitro.AdaptRepo(orders), &nitro.ResourceConfig{
		NoCache: true,
		Public:  true, // demo only — prefer JWT + SetDefaultResourceMiddleware
	})

	_ = app.Listen(":8080")
}
