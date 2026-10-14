# JSON (`nitro.JSON[T]`)

**Language:** [English](json.md) · [Português](../pt/guide/json.md)

Typed JSON columns with Valuer/Scanner. The **Go type is stable**; SQL changes per dialect.

```go
Meta nitro.JSON[map[string]any] `json:"meta" db:"meta"`
Tags nitro.JSONSlice[string]    `json:"tags" db:"tags"`
```

| Dialect | Column |
|---------|--------|
| Postgres | `JSONB` (+ default `'[]'::jsonb` for slices) |
| MySQL | `JSON` |
| SQLite | `TEXT` |
| SQL Server | `NVARCHAR(MAX)` |

Active dialect: `migration.SetDialect` / `NITRO_DB_DRIVER` in codegen.

## Multitenant attrs

```go
type Customer struct {
    nitro.TenantModel // Model + TenantID
    Name  string      `json:"name" db:"name"`
    Attrs nitro.Attrs `json:"attrs" db:"attrs,jsonb" blinq:"gin"`
}

defs := []nitro.FieldDef{{Key: "gramatura", Type: "string", Required: true}}
_ = nitro.ValidateAttrs(defs, customer.Attrs.V)
```

`TenantModel` / `Attrs` / `ValidateAttrs` are framework primitives. Resolve the
tenant with `nitro.TenantMiddleware` (JWT claim `tenant_id` and/or header) and
scope CRUD with `ResourceConfig.TenantScope` — see
[`resource-security.md`](resource-security.md). Per-tenant `field_schemas`
storage still lives in the app.
