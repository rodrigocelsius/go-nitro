# JSON (`nitro.JSON[T]`)

**Idioma:** [Português](json.md) · [English](../../guide/json.md)

Colunas JSON tipadas com Valuer/Scanner. O **tipo Go é estável**; o SQL muda por dialeto.

```go
Meta nitro.JSON[map[string]any] `json:"meta" db:"meta"`
Tags nitro.JSONSlice[string]    `json:"tags" db:"tags"`
```

| Dialeto | Coluna |
|---------|--------|
| Postgres | `JSONB` (+ default `'[]'::jsonb` para slices) |
| MySQL | `JSON` |
| SQLite | `TEXT` |
| SQL Server | `NVARCHAR(MAX)` |

Dialeto ativo: `migration.SetDialect` / `NITRO_DB_DRIVER` no codegen.

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

`TenantModel` / `Attrs` / `ValidateAttrs` são primitivos do framework. Resolva o
tenant com `nitro.TenantMiddleware` (claim JWT `tenant_id` e/ou header) e
escopo o CRUD com `ResourceConfig.TenantScope` — ver
[`resource-security.md`](../../guide/resource-security.md). Persistência de
`field_schemas` por tenant continua na app.
