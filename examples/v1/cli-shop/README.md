# cli-shop

Generated **entirely by the CLI** (no hand edits) — the reference for what `nitrogen` produces:

```bash
nitrogen new shop --module github.com/acme/shop
cd shop
nitrogen make:scaffold Invoice total:float64 customer:string --resource --tenant   # tenant_id ⇒ auto tenant scope
nitrogen make:scaffold Product name:string price:float64 --resource
go test ./...
NITRO_ENV=development go run ./cmd/server      # or: nitrogen dev
```

- Paths by convention: `/invoices`, `/products` (plural of the model).
- `Invoice` has a `tenant_id` column, so its Resource is tenant-scoped automatically; in production the tenant
  comes from the JWT claim (`nitrogen install auth-jwt`). See [`docs/guide/conventions.md`](../../../docs/guide/conventions.md).
- Generated tests (`*_resource_test.go`) exercise create + list through the real `Register*` wiring.
