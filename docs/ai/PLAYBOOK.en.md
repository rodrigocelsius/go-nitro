# Nitro v1 — Playbook for AIs

**Language:** [English](PLAYBOOK.en.md) · [Português](PLAYBOOK.md)

Operational document: read and **implement** a full system in the framework’s patterns.  
Short entry: [`AGENTS.md`](../../AGENTS.md) · [`CLAUDE.md`](../../CLAUDE.md) · [`AI.md`](../../AI.md) · CLI [`docs/guide/cli.md`](../guide/cli.md).

**Audience:** coding agents (Cursor, Claude, Codex, etc.).  
**Code language:** Go 1.25+ for SQL apps (`data/adapter/sql` / pgx).  
**Canonical import:** `nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"`.  
## DEFAULT DATA ACCESS (STOP)

**Unspecified ORM ⇒ BLinq for every system.**  
`sqladapter.OpenEnv` / `platform.Open` + `StoreOf` + `modelgen` + `WhereIf`.  
BLinq is the default; GORM / sqlx / sqlc are supported when the user names them.
“Use an ORM” / “repository” / “data layer” without a stack name still means **BLinq**.  
Examples: `cleanarch-blinq`, `db-blinq` — open `db-gorm` / `db-sqlx` / `db-sqlc` **only after** opt-out. Guide: [`docs/guide/blinq.md`](../guide/blinq.md).

---

## 0. Mission and work order

When you receive a system request (API, CRUD, auth, etc.):

1. Classify the domain (resources, rules, auth, DB). **If no ORM is named → BLinq only.**
2. **Generate with the CLI** (`nitrogen new` → `install` → `make:scaffold` / `make:*`) — do not invent the tree.
3. Pick the **recipe** (§1) only for what the CLI does not cover (Resource vs handlers, auth…).
4. Finish wiring (`cmd/server`, `handler/routes.go`), **BLinq** (`make … --data`) + migrate if SQL.
5. Validate the checklist (§11) + `nitrogen doctor`.
6. Do not invent APIs outside this playbook; copy from the listed examples. **BLinq is the default** — warn / refuse other stacks if the ask is generic (“use an ORM”).

---

## 1. Recipes (when to use what)

### R1 — API without a database (prototype)

- `nitrogen new app --basic` **or** default CA + `make:scaffold` with a memory repository.
- `nitro.New()` + routes **or** Resource + `MemoryRepository`.
- Example: `examples/v1/basic`, `examples/v1/cleanarch`, `examples/v1/crud`.

### R2 — Postgres CRUD (preferred)

1. `nitrogen new app` + `nitrogen install postgresql` (or `mysql` / `mariadb` / `sqlserver` / `sqlite`).
2. **Default shape:** `nitrogen make:scaffold Product name:string price:float64 active:bool --resource`
   - `app.Resource` + BLinq `StoreOf` + **`internal/handler/routes.go`**
   - Minimal `main` (open + wire + Listen); **no** CA usecase/repository for plain CRUD
   - Filters: `ResourceConfig` / `blinq:"index"` (`?q=` / `?name=` / `sort=`)
3. **Only if non-CRUD domain rules need usecases:** omit `--resource` (CA handlers + BLinq repository) — still thin layers.
4. Optional: `--with-seed` / `--with-factory`.
5. `nitrogen up` or `go run ./cmd/server`.
6. Copy: `examples/v1/blinq-hello` → `db-blinq` → `resource-dx`; CA: `cleanarch-blinq` only when needed.

Filters: `blinq:"index"` on the model → Filterable + Sortable on Resource; CA List uses `WhereIf` + `?sort=`.
Query params accept **snake and camel** by default (`AcceptCamelQuery`; e.g. `?artistId=` ≡ `?artist_id=`).

### R2b — Secure Resource + DX (required with auth)

```go
auth := nitro.NewAuth(nitro.AuthConfig{JWTSecret: secret}).WithMemoryStore()
app.AuthRoutes("/auth", auth)
app.SetDefaultResourceMiddleware(nitro.JWTMiddleware(auth.Provider()))

app.Resource("/api/catalog/albums", &model.Album{}, stores.Albums, &nitro.ResourceConfig{
    NoCache: true,
    FilterRules: []nitro.FilterRule{
        {Param: "artist_id", Field: "artist_id", Op: "eq"}, // also accepts ?artistId=
        {Param: "q", Field: "name", Op: "icontains"},
    },
    BeforeWrite: func(ctx context.Context, entity any) error {
        a := entity.(*model.Album)
        return nitro.FillStringFrom(ctx, stores.Artists, a.ArtistID, &a.ArtistName,
            func(ar *model.Artist) string { return ar.Name })
    },
    Commands: nitro.Commands(
        nitro.Cmd("enrich", func(c *nitro.Ctx, album *model.Album) error {
            // POST /api/catalog/albums/:id/enrich
            return c.OK(nitro.Map{"ok": true, "id": album.ID})
        }),
    ),
})
```

Rules:

- Do **not** set `Public: true` only to silence warnings — in production an unauthenticated Resource **panics** (V-04) unless `AllowUnauthenticatedResources`.
- Denormalized fields: `computed:"true"` + `BeforeWrite` / `FillStringFrom`.
- Business actions (`enrich`, `publish`): **`Commands`**, not loose `app.Post`.
- Dashboard: `store.CountAll` / `CountWhere` / `SumInt64` / `CountByTake` / `CountBuckets` + `query.MonthBuckets`.
- Guides: [`docs/guide/resource-security.md`](../guide/resource-security.md) · [`docs/guide/resource-hooks.md`](../guide/resource-hooks.md) · [`docs/guide/business-resource.md`](../guide/business-resource.md).

### R2c — Multi-tenant Resource (shared-schema)

```go
app.Use(nitro.TenantMiddleware(nitro.TenantConfig{Require: true}))
app.Resource(&model.Order{}, store) // tenant_id column ⇒ auto-scoped
```

- Embed `nitro.TenantModel`; set `User.TenantID` at login (JWT claim `tenant_id`) or (trusted callers only, requires `AllowHeader: &true`) pass `X-Tenant-ID`.
- List filters + Get/Update/Delete checks + Create stamp — do **not** rely on FilterRules alone (IDOR).
- Example: `examples/v1/resource-tenant-scoped`.

### R3 — Custom queries (dynamic filters)

- Handlers + `store.Query().WhereIf(...).OrderBy(...).Limit(...)`.
- Example: `examples/v1/db-blinq-handlers`.

### R4 — Auth

- Prefer CLI: `nitrogen install auth-jwt` or `nitrogen install auth-keycloak` (env + `internal/auth` stub + protected sample route).
- Local: `NewAuth` + `AuthRoutes` + `JWTMiddleware` / `RequireRole` (`examples/v1/auth`).
- Keycloak/OIDC: `nitro.NewKeycloak` / `nitro.OIDCProvider` + `JWTMiddleware` (`examples/v1/auth-keycloak`).
- Cache L2: `nitro.RedisCache(rdb)` (`adapters/cache`).
- OpenAPI artifact: `nitrogen openapi --bearer` (static) or `app.OpenAPI` / `app.Swagger` at runtime.
- SQL engine switch for local tests: `nitrogen install sqlite` then `nitrogen doctor --db` (upserts DSN; warns on migrations).

### R5 — Another SQL stack (GORM / sqlx / sqlc)

- **Only after explicit opt-out** (`use GORM`, `skip BLinq`, …). Unnamed ORM still means BLinq (R2).
- Even then: `pgxadapter.Open` → `db.SQL()` (GORM/sqlx) or `db.DBTX()` (sqlc).
- Examples: `examples/v1/db-gorm`, `db-sqlx`, `db-sqlc` — open **only** after opt-out.

### R6 — Clean Architecture

- Layout: `cmd/server` + `internal/{config,model,usecase,handler,repository}` (+ `migrations`).
- Usecase per operation: `usecase/product/{create,get,list,update,delete}.go`.
- Service + manual route for rules beyond CRUD (e.g. discount).
- Do **not** create `internal/{seed,factory,middleware,job}` in the initial scaffold — only via `nitrogen make:seeder|factory|middleware|job` when the requirement asks. When writing content, the CLI removes `.gitkeep` if present.
- Example: `examples/v1/cleanarch` (memory) or `examples/v1/cleanarch-blinq` (Postgres + BLinq). CLI: `nitrogen new` / `make:scaffold`.

---

## 2. Mental layout of the monorepo

```
pkg/                 → facade (always use this)
data/adapter/pgx/    → Open / OpenEnv, StoreOf, AdaptStore (optional), MigrateUp, SQL, DBTX
data/gateway/        → SQL telemetry (automatic via Open)
blinq/               → DSL, migrate runner
codegen/             → blinq metamodel, migration SQL
cmd/nitrogen/        → CLI make:* / install / new / migrate
examples/v1/         → canonical truths (copy from here)
domain/, usecases/   → framework internals (the app does not need them on the happy path)
```

User apps must **not** depend on `usecases/` directly; they use `pkg`.

---

## 3. HTTP v1 — Context contract

### Principle

Everything starts and ends on `c` (`*nitro.Ctx`).

### Input

| Method | When |
|--------|--------|
| `c.Param("id")` / `c.Param("id", "0")` | path (optional default) |
| `c.Int64Param("id")` | required path int64 → `(int64, error)` |
| `c.Query("q")` / `c.Int("page", 1)` / `c.Bool("active")` | query |
| `c.Sort("created_at")` | `?sort=-price` → `(field, desc)` |
| `nitro.Parse[T](c)` | body + validate by Content-Type (JSON or form; preferred) |
| `c.BodyParser(&v)` | same; default → transform → validate pipeline |
| `c.BodyJSON(&v)` | JSON only + validate |
| `c.Bind(&v)` | decode without validate (webhooks) |
| `c.URL` / `app.GetRouteURL` | reverse routing (named route `.Name(...)`) |
| `c.UserUUID()` / `c.UserInt64()` | typed claims |

### Output

| Method | Status |
|--------|--------|
| `c.OK(v)` | 200 |
| `c.Created(v)` | 201 |
| `c.NoContent()` | 204 |
| `c.Text("…")` / `c.JSON(status, v)` | text / explicit status |
| `c.File` / `c.CSV` / `c.Attachment` / `c.Stream` / `c.Redirect` | special |

### Errors

```go
return nitro.NotFound("user not found")
return c.BadRequest("invalid id")
return err // if err == pgx.ErrNotFound / domain.ErrNotFound → 404
```

Unhandled errors → **500** with `{"error":"internal server error"}`. In **development** (`Config.Env` / `NITRO_ENV=development`), the body also includes `request_id` and a short hint (`NITRO_OTEL=0` to quiet telemetry). Production never leaks internals.

### Context

`*nitro.Ctx` implements `context.Context`. Use `c` (or `c.Context()` if the API requires a named `context.Context`).

**Note:** strings returned by Param/Query live in the request pool — do not keep them after the handler.

---

## 4. Resource (CRUD Convention over Configuration)

```go
app.Resource(&Product{}, store, &nitro.ResourceConfig{
    NoCache: true,
    // Filterable/Sortable: omit to infer from blinq:"index"
    FilterRules: []nitro.FilterRule{
        {Param: "min_price", Field: "price", Op: "gte"},
        {Param: "q", Field: "name", Op: "icontains"},
    },
})
// explicit path still valid:
app.Resource("/products", &Product{}, store, &nitro.ResourceConfig{NoCache: true})
// typed memory:
nitro.Resource(app, "/products", repo, &nitro.ResourceConfig{NoCache: true})
```

### `app.Resource` (single API)

```go
app.Resource(&Product{}, store) // → /products (TableName / plural)
app.Resource("/users", &UserController{})
app.Resource("/users", &User{}, repo)
app.Resource("/users", &User{}, repo, &nitro.ResourceConfig{NoCache: true, Filterable: []string{"name"}})
```

A single config parameter: `*ResourceConfig`.
`*pgx.Store` implements `AsResourceRepository` — **no** `AdaptStore` needed on the happy path.
Model default: `nitro.Model` (`nitro.UUID` + `nitro.DateTime` + soft delete). BIGSERIAL: `ID int64` (AutoID already infers false; `AutoID: nitro.Ptr(false)` to force it).
Use `nitro.Date` / `TimeOfDay` for civil days/times; **not** `string`+`VARCHAR` or `time.Time` on Resource.
See guides: [uuid](../guide/uuid.md) · [temporal](../guide/temporal.md) · [json](../guide/json.md) · [money](../guide/money.md) · [units](../guide/units.md) · [locale](../guide/locale.md) · [convert](../guide/convert.md).

`nitrogen make … --data` generates the BLinq DSL in **`modelgen`**. Resource path is the plural of the type; if `//nitro:table=` differs, add `func (T) TableName() string` on the model.

`nitro.Resource(app, path, repo, opts...)` — generic variant for `MemoryRepository[T]`.

Generated routes: `GET /`, `GET /:id`, `POST /`, `PATCH /:id`, `DELETE /:id`.

**Rule:** do not write these 5 handlers by hand if Resource + filters are enough.

---

## 5. Data layer + Gateway

```
App → Store (BLinq) → Gateway → pgx | database/sql → PostgreSQL / MySQL / MariaDB / SQL Server / SQLite
```

```go
ctx := context.Background()
db, err := sqladapter.OpenEnv(ctx) // NITRO_DB_DRIVER + DATABASE_URL | DB_DSN
// CLI apps: platform.Open(ctx) → sqladapter.OpenEnv
defer db.Close()

if err := db.MigrateUp(ctx); err != nil { /* ./migrations */ }

store := sqladapter.StoreOf[Product](db, "products")
app.Resource(&Product{}, store, opts)
```

Tests (no Docker):

```go
db, err := sqladapter.OpenTest(ctx)                    // in-memory SQLite
db, err := sqladapter.OpenTestMigrate(ctx, "migrations")
```

| API | Use |
|-----|-----|
| `sqladapter.OpenEnv` / `platform.Open` | **preferred** app entry |
| `MigrateUp` | goose `./migrations` (**preferred**) |
| `MigrateDir` / `MigrateFS` | custom path / embed |
| `OpenTest` / `OpenTestMigrate` | unit/integration tests without Docker |
| `NITRO_DB_QUIET=1` | turns off SQL logger |

**Forbidden:** opening a pool without Gateway (`sqladapter` / `pgxadapter.Open`). Low-level `pgxadapter.Open` remains valid for Postgres-only code paths.
### Transactions

```go
exec := pgxadapter.NewExecutor(db.Pool)
tx, err := exec.Begin(c)
if err != nil {
    return err
}
defer tx.Rollback(c)

// use tx.Exec / Query in the atomic scope
if err := tx.Commit(c); err != nil {
    return err
}
```

Detail: [`docs/guide/data-layer.md`](../guide/data-layer.md) (Transactions section).

---

## 6. BLinq — model vs metamodel

| Artifact | Who writes it | Role |
|----------|--------------|--------|
| `type Product struct` (`model` package) | you | row / JSON / scan |
| `modelgen.Products` / `ProductModel` | codegen (`nitrogen make … --data`) | typed DSL |
| `modelgen.ProductEntity` | codegen | table name |
| `TableName()` (optional) | you, on the model | Resource path when it is **not** the plural of the type |

```go
items, err := store.Find(c, store.Query().
    WhereIf(c.Query("active") == "true", modelgen.Products.Active.Eq(true)).
    WhereIf(q != "", modelgen.Products.Name.IContains(q)).
    OrderBy(modelgen.Products.CreatedAt.OrderDesc()).
    Limit(50))
```

```go
p, err := store.CreateOne(c, &product)
p, err := store.Update(c, id, modelgen.Products.Name.Set(name), modelgen.Products.Price.Set(price))

created, err := store.CreateMany(c, rows, pgx.WithChunk(500))
n, err := store.UpdateWhere(c,
    store.Query().Where(modelgen.Products.Active.Eq(false)),
    modelgen.Products.Name.Set("archived"),
)
```

Bulk: **insert** = `CreateMany` / `query.InsertInto().Rows().Chunk()`; **update** = `UpdateWhere` (or `query.Update().Set().Where().OmitReturning()`). Do not loop `CreateOne`/`Update` per row. `CreateMany` returns `RETURNING *` — modest chunks. Million-row dumps: Postgres `COPY` in an importer is OK; the Go app still uses BLinq + `modelgen`.

Shared stand-in: `examples/v1/productmodel` + `productmodel/modelgen`.
In a real app: `nitrogen make internal/model/product.go --data` → `internal/modelgen/`.
Legacy (same package): `--same-package`.

### Model tags (migration + runtime)

```go
//nitro:table=products
type Product struct {
    nitro.Model // UUID + DateTime UTC + soft delete

    Name   string  `json:"name" db:"name" validate:"required,min=2"`
    Price  float64 `json:"price" db:"price" validate:"gte=0"`
    Active bool    `json:"active" db:"active" default:"true" blinq:"index"`
}

// Hard delete: nitro.HardModel + //nitro:nosoftdelete
```

| Tag / directive | Effect |
|----------------|--------|
| `db:"col"` | column |
| `validate:"…"` | Parse/Resource validation |
| `default:"…"` | JSON default if key missing |
| `transform:"…"` | sanitize/case **before** validate (`trim`, `titlecase=pt`, `digits`, `upper`, …) |
| `nitro.StringSlice` / `JSONSlice[T]` / `JSON[T]` | JSON per dialect (PG JSONB / MySQL JSON / SQLite TEXT); Valuer/Scanner |
| `nitro.UUID` / `Date` / `TimeOfDay` / `DateTime` | portable types (not `string`/`time.Time` on the model) |
| `nitro.Money` / `Measure` / `Decimal` | money (2 cols), SI units, decimal |
| `nitro.Latitude` / `Longitude` / `LatLng` | WGS84 float64 + 9 digits; SQL NUMERIC/DECIMAL(12,9) or SQLite TEXT — [geo](../guide/geo.md) |
| `db:"col,jsonb"` / `blinq:"jsonb,gin"` | force JSON (+ GIN index if `gin` on PG) |
| `blinq:"index"` | CREATE INDEX **and** Filterable+Sortable on Resource (if lists are nil) |
| `blinq:"unique"` / `unique:"true"` | unique |
| `blinq:"pk"` / `primary:"true"` | PK (`id` already is) |
| `blinq:"soft"` / `DeletedAt` / `nitro.Model` | soft delete |
| `HardModel` / `//nitro:nosoftdelete` | physical DELETE |
| `blinq:"tenant"` | tenant |

Without `notimestamps`/`nosoftdelete`, the migration generator **injects** `id` (UUID), `created_at`, `updated_at`, `deleted_at`.
At runtime, embed `nitro.Model` so the Go struct has the same fields.

### CLI migrations

```bash
nitrogen migrate:make models.go --name create_products
# → migrations/YYYYMMDDHHMMSS_create_products.sql
# app: db.MigrateUp(ctx)
```

---

## 7. Validation

```go
in, err := nitro.Parse[Product](c)
if err != nil {
    return err // 400 JSON | 422 fields
}
```

- Defaults: `default:"true"` or `validate:"default=true"` — only if the JSON key **did not** appear in the body.
- Transforms: `transform:"trim,titlecase=pt,digits,upper,emptyslice"` — pipeline **default → transform → validate**. Helpers: `nitro.Digits` / `IsDigits` / `SplitName` / `RandomDigits` / `IsEmail` / `NormalizeEmail`. See `examples/v1/transform`. Do **not** write `toTitleCase` / `isDigits` / `randomDigits` in the app.
- Config: `nitro.ConfigOf[T]()` / `MustConfig` / `LoadDotEnv` — nested + CSV `[]string` + aliases `env:"DB_DSN,DATABASE_URL"` + `Duration`/`url.URL` + `env:"X,overwrite"`. Guide `docs/guide/config.md`. Do **not** reparse `.env` or map `flatEnv` by hand.
- Auth: `RequireRoles([]string{"admin"}, WithBypassRoles("super_admin"))`. Resource: `BeforeCreate`/`OnMutate`, `ExtraRoutes`, `Aliases`, `Only`/`Except`/`Replace`, PATCH merge on.
- BR validators: `nitro.RegisterBRValidators(v)` (opt-in). DB: `OpenFrom` + `Ready` (or `OpenWithPool` + `EnsureSchema`). Health: `Ping` / `NamedHealth`. Loose body: `c.BindMap()`. Soft≠status: `FilterActive` / `SoftDeleteSQL`.
- Default handlers: `Me` / `AppInfo`; CSV: `c.CSV` / `c.CSVFrom` + `JSONCell`.
- Scaffold: `nitrogen new app [--basic|--microservice]`; `nitrogen install postgresql`; DTO: `nitrogen make:dto` (`nitro:"public"`). Example: `examples/v1/resource-dx` / `cleanarch`.
- Custom: `nitro.RegisterBRValidators(v)` for BR documents; `v.Register("sku", …)` only for **app** rules. See `examples/v1/validation`.
- Resource validates Create/Update automatically.

---

## 8. Auth

```go
auth := nitro.NewAuth(nitro.AuthConfig{
    JWTSecret:       os.Getenv("JWT_SECRET"),
    AccessTokenTTL:  15 * time.Minute,
    RefreshTokenTTL: 7 * 24 * time.Hour,
}).WithMemoryStore()

app := nitro.New()
app.AuthRoutes("/auth", auth)

api := app.Group("/api", nitro.JWTMiddleware(auth.Provider()))
api.Get("/profile", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"user_id": c.UserID()})
})
api.Resource("/items", &Item{}, repo, &nitro.ResourceConfig{
    WriteMiddleware: []nitro.Handler{nitro.RequireRole("admin")},
})

admin := app.Group("/admin",
    nitro.JWTMiddleware(auth.Provider()),
    nitro.RequireRole("admin"),
)
admin.Resource("/items", &Item{}, repo) // JWT+admin on all Resource routes
```

Do **not** invent global path-prefix middleware (`apiAuthGate`) — use `Group` + `WriteMiddleware` / `ReadMiddleware`.

---

## 9. Middleware and defaults

`nitro.New()` already enables: Recover, Logger, MaxBody, RequestID, security headers; wide CORS in `EnvDevelopment`.

Common opt-in: `CORS`, `RateLimit`, `Compress`, `JWTMiddleware`, groups `app.Group(prefix, mws...)`.

Listen: `nitro.Ok(app.Listen(":8080"))` or `log.Fatal(app.Listen(...))` — graceful shutdown by default.

---

### v2 conventions — do NOT configure these (STOP)

Nitro is Convention over Configuration; the defaults below are tested by `pkg/coc_contract_test.go`. Writing the override by hand is a review defect.

| Topic | Convention (zero config) | Never write |
|-------|--------------------------|-------------|
| Env | `nitro.New()` reads `NITRO_ENV` (`development`\|`dev`\|`local`, else production). Apps from the CLI pass `Env: nitro.ParseEnv(cfg.Env)` | `Config{Env: nitro.EnvDevelopment}` hard-coded |
| Resource path/wire | `app.Resource(&Product{}, repo)` → `/products`; JSON/DB names are snake_case of the Go field | `Resource("/products", …)` paths or `json`/`db` tags that only repeat the default |
| Multi-tenant | a model with a `tenant_id` column (`nitro.TenantModel`, or `make:scaffold … --tenant`) is auto-scoped; tenant comes from the JWT claim (`tenant_id`/`tenantId`/`tid`/`org_id`/`organization_id`) | `TenantScope: TenantFromCtx()` (only `NoTenantScope: true` to opt out); trusting `X-Tenant-ID` (ignored unless `TenantConfig.AllowHeader`) |
| Client IP | `c.ClientIP()` trusts forwarding headers only from private/loopback peers (nginx/ALB/Docker work as-is) | `SetTrustedProxies` unless peers are public (`Config.TrustedProxies`) |
| Middleware | `app.CORS().ETag().Compress().Timeout(d).RateLimit(n, w)`; per route `nitro.BasicAuth`, `nitro.APIKeyAuth`, `nitro.Skip`, `nitro.RedirectHTTPS` | `app.UsePost(CompressMiddleware())` placement by hand |
| Typed handlers | `nitro.Handle(func(ctx, *In) (*Out, error))`: path (`:id`→`ID`), query (`?unit_price`→`UnitPrice`) and body bind by JSON/snake_case field name | `param:`/`query:` tags that repeat the field name |
| Config errors | never panic: `app.Err()` / `Listen` return the first mistake | `recover()` around setup |
| IDs / request id | UUID v7 (`nitro.Model`); `X-Request-ID` is UUID v7 | custom id generators |
| Errors | `{"error": …}`; RFC 9457 via `Config{ErrorHandler: nitro.ProblemDetails()}` | hand-rolled error envelopes |

Removed in 2.0.0 (do not use): `nitro.CORS/Compress/Timeout`, `nitro.OK/Created/NoContent` helpers, `MapAny`, `SerialID`, `WithCache`, `ResourceOpts` option structs — see the migration table in [`docs/guide/conventions.md`](../guide/conventions.md). Reference app generated 100% by the CLI: [`examples/v1/cli-shop`](../../examples/v1/cli-shop/); zero-config tour: [`examples/v1/conventions`](../../examples/v1/conventions/).

## 10. Complete template (paste and adapt)

Target structure for an API with a `Note` resource:

```
app/
  models.go
  modelgen/
    models_gen.go           # nitrogen make models.go --data (DSL)
  migrations/
    YYYYMMDDHHMMSS_create_notes.sql
  main.go
  go.mod
```

### models.go

```go
package main

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

//nitro:table=notes
type Note struct {
    nitro.Model
    Title  string `json:"title" db:"title" validate:"required,min=1"`
    Body   string `json:"body" db:"body"`
    Active bool   `json:"active" db:"active" default:"true" blinq:"index"`
}
```

### main.go (skeleton)

```go
package main

import (
    "context"
    "log"

    pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
    nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
    ctx := context.Background()
    db, err := pgxadapter.Open(ctx, pgxadapter.DSN("postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable"))
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    if err := db.MigrateUp(ctx); err != nil {
        log.Fatal(err)
    }

    store := pgxadapter.StoreOf[Note](db, "notes")

    app := nitro.New()
    app.Resource(&Note{}, store, &nitro.ResourceConfig{
        NoCache: true,
        // Filterable/Sortable inferred from blinq:"index"; FilterRules for extra ops:
        FilterRules: []nitro.FilterRule{
            {Param: "q", Field: "title", Op: "icontains"},
        },
    })

    // Custom route (optional):
    // app.Get("/notes/search", func(c *nitro.Ctx) error { ... WhereIf ... })

    nitro.Ok(app.Listen(":8080"))
}
```

### Commands

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
nitrogen make models.go --data
nitrogen migrate:make models.go --name create_notes
go run .
```

### When to add a manual handler

- Aggregations, discounts, workflows, payloads that are not the model.
- Pattern: `nitro.Parse[Input](c)` → service → `c.OK` / errors.
- See `ApplyDiscountHandler` in `examples/v1/cleanarch`.

---

## 11. Delivery checklist

- [ ] New system/resource generated with the CLI (`new` / `install` / `make:scaffold` / `make:*`) when applicable
- [ ] v1 APIs only (`c.OK`, `Parse`, no `data/runtime`)
- [ ] CRUD via Resource (`--resource`) **or** CA (`usecase` + `handler`) — do not reinvent the 5 endpoints for no reason
- [ ] DB via `sqladapter.OpenEnv` / `platform.Open` (+ `MigrateUp` / `nitrogen up`) when there is a schema
- [ ] Generated metamodel (`make … --data` → `modelgen`) when using BLinq
- [ ] `nitro.Model` (UUID + soft) or `HardModel` / explicit serial
- [ ] `validate` / `transform` tags on write fields
- [ ] No manual listen/shutdown `log` in `main`
- [ ] Auth only if the requirement asks (`install auth-jwt` / `auth-keycloak` or examples)
- [ ] Optional: `nitrogen openapi`, `nitrogen seed`, `doctor --db`
- [ ] `go build` / `go test` / `nitrogen doctor` OK

---

## 12. Anti-patterns (reject in review)

| Wrong | Right |
|--------|-------|
| Unnamed ORM / “use an ORM” → GORM/sqlx/sqlc | **BLinq** (`StoreOf` + `modelgen`) — opt-out only if explicit |
| Five CRUD handlers / CA for plain CRUD | **`app.Resource`** + `routes.go` (`--resource`) |
| CRUD wired in `main` | `internal/handler/routes.go` `Register` |
| `return nitro.OK(x)` for success | `return c.OK(x)` |
| `c.BindJSON` / `QueryIntDefault` | `Parse` / `c.Int` |
| Raw pgx pool / `database/sql` | `sqladapter.OpenEnv` / `platform.Open` |
| `data/runtime` / old LINQ | BLinq `query` + `field` |
| `product.Price.Gte` on the row struct | `Products.Price.Gte` on the metamodel |
| 5 manual CRUD handlers | `app.Resource` |
| `AutoID: true` + BIGSERIAL | `ID int64` (AutoID turns itself off) or `AutoID: nitro.Ptr(false)` |
| SoftColumn without Soft() | SoftColumn on Entity already enables soft; `Hard()` to opt out |
| Duplicate id/timestamp fields | `nitro.Model` / `HardModel` |
| Schema only in `Exec` without versioning (real apps) | `migrate:make` + `MigrateUp` |
| CPF/CNPJ no `registerBuiltins` / `NewValidator()` | plugin `usecases/validator/br` + `RegisterBRValidators` |
| `toTitleCase` / `digitsOnly` / `isDigits` / `splitName` / `randomDigits` no app | `transform:"titlecase=pt,digits"` e `nitro.Digits` / `IsDigits` / `SplitName` / `RandomDigits` / `IsEmail` |
| Homegrown `.env` parser | `nitro.ConfigOf` / `LoadDotEnv` |
| CSV + manual `Fasthttp().SetBody` | `c.CSV` / `c.CSVFrom` / `JSONCell` / `c.Attachment` |
| Manual health ping + timeout | `nitro.Ping("name", pingFn)` |
| Manual `/api/me` / root info | `nitro.Me()` / `MeRoute` / `AppInfo` / `Info` |
| Separate EnsureSchema + MigrateUp | `OpenFrom` + `db.Ready` |
| Resource without DELETE (or custom List) | `Only` / `Except` / `Replace` |
| CPF/CNPJ/phone/CEP in the app (copying the algorithm) | `nitro.RegisterBRValidators(v)` |
| Hand-rolled `search_path` / CREATE SCHEMA | `OpenFrom` / `OpenWithPool` + `EnsureSchema` |
| Wrap repo only for cache invalidate | `ResourceConfig.OnMutate` |
| Manual public DTO | `nitrogen make:dto` / `nitro:"public"` |
| Homegrown `type X []T` + JSONB Valuer/Scanner | `nitro.JSONSlice[T]` / `StringSlice` |
| Global path-prefix middleware (`apiAuthGate`) | `Group` + `ResourceConfig.WriteMiddleware` |

---

## 13. Quick file map

| Need… | Open… |
|-------------|--------|
| Hello DB | `examples/v1/blinq-hello/` |
| Resource + filters | `examples/v1/db-blinq/` |
| WhereIf handlers | `examples/v1/db-blinq-handlers/` |
| Clean arch | `examples/v1/cleanarch/` (memory) · `examples/v1/cleanarch-blinq/` (BLinq) |
| Auth | `examples/v1/auth/` |
| Custom validation | `examples/v1/validation/` |
| Transforms | `examples/v1/transform/` |
| Ctx syntax | `docs/guide/context.md` · `docs/guide/request.md` |
| BLinq guide | `docs/guide/blinq.md` |
| Data layer | `docs/guide/data-layer.md` |
| Open/Store/Migrate | `data/adapter/sql` · `data/adapter/pgx` |
| CLI | `docs/guide/cli.md` · `cmd/nitrogen/` |

---

## 14. Style of AI-generated code

- Portuguese only in example/README comments if the repo is in PT; identifiers in English.
- Header comment in `main` with `export DATABASE_URL`, `go run .`, and curls.
- Minimal diff: do not refactor the framework while implementing an app.
- Prefer composition matching the v1 examples over new abstractions.
- One resource per initial migration; later migrations for changes.

---

**End of playbook.** If older docs conflict with this file, **this file wins**.
