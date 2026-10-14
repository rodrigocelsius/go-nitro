# Resource + shared-schema tenant

Demonstrates `nitro.TenantMiddleware` + `ResourceConfig.TenantScope` with `nitro.TenantModel`.

```bash
go test ./...
go run .
```

```bash
# Create as tenant A
curl -s -X POST localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: 01900000-0000-7000-8000-000000000001' \
  -d '{"title":"order-a"}'

# Tenant B list (empty / only own rows)
curl -s localhost:8080/orders \
  -H 'X-Tenant-ID: 01900000-0000-7000-8000-000000000002'
```

Production: put `tenant_id` on the JWT (`User.TenantID` at login), use `JWTMiddleware` then `TenantMiddleware`, and drop `Public: true`. See [resource-security.md](../../../docs/guide/resource-security.md).
