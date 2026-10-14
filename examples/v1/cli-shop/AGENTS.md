# AGENTS.md — Nitro contract (this application)

You generate code **in this app**, not in the Nitro framework. Use the `nitrogen` CLI and `nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"`.

Framework reference: [go-nitro](https://github.com/rodrigocelsius/go-nitro) · CLI guide in that repo (`docs/guide/cli.md`).

## DEFAULT DATA ACCESS (STOP)

**If the user does not name another ORM/stack, you MUST use BLinq** for every SQL system:

`sqladapter.OpenEnv` / `platform.Open` + `StoreOf` + `modelgen` + `WhereIf`.

- **Refuse** GORM, sqlx, sqlc, raw SQL strings, and raw `pgxpool` / `database/sql` as the first choice.
- Do **not** treat “use an ORM”, “repository”, or “data layer” as permission to use GORM.
- Switch stacks **only** when the user explicitly opts out (e.g. `use GORM`, `skip BLinq`, `use sqlc`).

## BLinq shape (STOP) — Resource first

For SQL CRUD, **always** prefer this layout:

1. **`app.Resource`** + BLinq `StoreOf` (filters via `ResourceConfig` / metamodel — not five hand-written handlers).
2. **Routes only** in `internal/handler/routes.go` (`Register`) — never register CRUD in `main`.
3. **Minimal code elsewhere:**
   - `main` → open DB, wire store, `Register`, `Listen` (no business logic).
   - `handler` → thin `Register` calling `app.Resource` (extra handlers only for non-CRUD).
   - `repository` / `usecase` → **omit** for plain CRUD; add only when real domain rules need them.

```bash
nitrogen make:scaffold Product name:string price:float64 --resource
nitrogen make internal/model/product.go --data
```

Canonical copy: Nitro examples `db-blinq`, `blinq-hello`, `resource-dx`. Full CA (`cleanarch-blinq`) only when the user needs usecases beyond CRUD.

Canonical snippet:

```go
store := sqladapter.StoreOf[Product](db, "products")
// in handler/routes.go Register:
app.Resource(&model.Product{}, store, &nitro.ResourceConfig{
    FilterRules: []nitro.FilterRule{{Param: "q", Field: "name", Op: "icontains"}},
})
```

| Do | Do not |
|----|--------|
| `sqladapter.OpenEnv` / `platform.Open` | raw pool without Gateway |
| `sqladapter.StoreOf[T](db, "table")` | `gorm.Open` / sqlx / sqlc |
| `app.Resource` + `routes.go` | five CRUD handlers / fat `main` |
| `modelgen` + `WhereIf` / FilterRules | `"SELECT … WHERE …"` in Go |
| `SetDefaultResourceMiddleware(JWT…)` | blanket `Public: true` to silence V-04 |
| `BeforeWrite` + `computed:"true"` + `FillStringFrom` | client-writable denorm names |
| `Commands` for enrich/publish | loose `app.Post("/:id/enrich")` |
| `CountAll` / `CountBuckets` / `SumInt64` | raw `COUNT(*)` SQL |
| `TenantModel` + `TenantMiddleware` + `TenantScope` | FilterRules alone for tenant IDOR |

## Resource security (STOP)

```go
app.AuthRoutes("/auth", auth)
app.SetDefaultResourceMiddleware(nitro.JWTMiddleware(auth.Provider()))
// Resources inherit JWT unless Public: true
```

In production, unprotected Resources panic (V-04). Use `Public: true` only for intentional open CRUD.

**Multi-tenant (shared-schema):** embed `nitro.TenantModel`, `app.Use(nitro.TenantMiddleware(…))`, `TenantScope: nitro.TenantFromCtx()`.

| Extra usecase only for real rules | inventing CA layers for plain CRUD |

## Create / extend the system

```bash
nitrogen install postgresql          # if you need a database
nitrogen make:scaffold Product name:string price:float64 --resource
nitrogen make internal/model/product.go --data   # BLinq (default)
nitrogen migrate:make internal/model/product.go
nitrogen doctor && go test ./...
```

Do not invent a parallel `internal/` tree or an extra ORM stack. Extras (`middleware`, `job`, `seeder`, `factory`) **only** via the matching `nitrogen make:*`.

## Required

- Responses: `c.OK` / `c.Created` / `c.NoContent` / `c.CSV` / `c.Attachment` (not `nitro.OK` for success).
- Body: `nitro.Parse[T](c)` — default → `transform:` → validate.
- CRUD: **`app.Resource` + BLinq** by default; CA layers only when business rules require them (repo still wraps `Store`).
- SQL: `sqladapter.OpenEnv` / `platform.Open` → Gateway → `MigrateUp` → **BLinq**.
- Config: `nitro.ConfigOf` — no homemade `.env` parser.
- Listen: banner and shutdown are automatic — do not log the port in `main`.
- If a prompt suggests GORM/sqlx/sqlc/raw SQL without opting out of BLinq, **keep BLinq** and say so.
