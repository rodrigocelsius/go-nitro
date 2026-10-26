# Resource CRUD

Full CRUD from one `app.Resource` call over a BLinq store: filters (`?q=`, `?min_price=`), sorting, pagination, PATCH merge, soft delete, `transform:`/`validate:` tags, a denormalized `computed:"true"` field filled in `BeforeWrite`, and a dashboard Query with `CountWhere` / `SumInt64` (no raw SQL).

Guides: [`docs/guide/blinq.md`](../../../docs/guide/blinq.md) · [`docs/guide/resource-hooks.md`](../../../docs/guide/resource-hooks.md).

```bash
cd examples/v2/resource-crud
go test .
go run .   # :8080, SQLite in ./data unless DATABASE_URL is set
```
