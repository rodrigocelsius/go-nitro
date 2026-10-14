# BLinq

**Language:** [English](blinq.md) · [Português](../pt/guide/blinq.md)

Typed query DSL for Go — **AST + Providers** — incubated in `github.com/rodrigocelsius/go-nitro/blinq/v2`.

> **Nitro default:** BLinq is the official data-access path with Postgres (`StoreOf`, `modelgen`, `Resource` / `WhereIf`). Do not write raw SQL or pick GORM/sqlx/sqlc unless the requirement asks.

**SQL dialects:** PostgreSQL, MySQL, MariaDB (MySQL dialect), SQL Server and SQLite. Postgres-wire databases (CockroachDB, YugabyteDB) should work with the Postgres dialect but are not tested. Non-SQL providers: memory, MongoDB, Elasticsearch, GraphQL, JSON API. `nitrogen doctor` warns when a model changed after its generated `modelgen`.

BLinq separates *what* (immutable query / AST) from *where* (memory provider, Postgres/SQLite/MySQL SQL over the Nitro Gateway).

## Quick start

```go
import (
    "github.com/rodrigocelsius/go-nitro/blinq/v2/field"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/query"
    blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
)

ent := field.Entity{Name: "users", SoftColumn: "deleted_at"}
// SoftColumn enables soft delete automatically — Soft() is optional; Hard() turns it off.
q := query.From(ent).Where(...)
user := struct {
    Age field.Field[int]
}{Age: field.Col[int]("users", "age")}

q := query.From(ent).Soft().Where(user.Age.Gte(18)).Limit(50)
plan, _ := q.Explain(ctx, sql.New(pgx.AdaptGateway(gw), dialect.Postgres()))
```

## Packages

| Package | Role |
|--------|--------|
| `blinq/expr` | Public AST (`ASTVersion=1`) |
| `blinq/field` | `Field[T]`, `Entity`, predicates |
| `blinq/query` | Immutable DSL + Paginate / Soft / Tenant / Join / Upsert |
| `blinq/scan` | `MapToStruct` / converters |
| `blinq/provider/memory` | slice execution (+ `NewMulti`) |
| `blinq/provider/sql` | compile + execute via `DB` (Gateway) |
| `blinq/provider/sql/dialect` | Postgres / SQLite / MySQL / MSSQL |
| `blinq/provider/sql/postgres` | thin `New(db)` helper |
| `blinq/provider/mongo` | `CompileFind` / `CompileLookup` → map (zero deps) |
| `blinq/provider/mongo/driver` | submodule: Execute via mongo-driver (`CollectionAPI`) |
| `blinq/provider/elastic` | `CompileSearch` → Query DSL (zero deps) |
| `blinq/provider/elastic/client` | submodule: Search API (`go-elasticsearch/v8`) |
| `blinq/provider/graphql` | `CompileQuery` + HTTP Provider (`Poster`) |
| `blinq/provider/api` | JSON wire + HTTP Provider |
| `blinq/migrate` | Runner Up/Down/Status + `FSSource` |

## Codegen

You write the model **like sqlc** (`type Product struct { ... }`). Codegen emits the query metamodel (`ProductModel` with `Field[T]`):

```bash
nitrogen make models.go --data
# model/ package → DSL in ../modelgen/
# Flat package (e.g. productmodel/) → productmodel/modelgen/
# Legacy: --same-package → *_model_gen.go in the same package (includes TableName)
```

| File / package | Role |
|------------------|--------|
| `model.Product` | row / JSON / scan (like sqlc) |
| `modelgen.ProductModel` / **`modelgen.Products`** | typed columns for the DSL |

Short alias (codegen): table `products` → `Products` (= `ProductModel`).

```go
modelgen.Products.Active.Eq(true)
modelgen.Products.Price.Gte(100)
// bool: field.IsTrue(modelgen.Products.Active)
```

You cannot use only the data struct in the DSL: `float64` has no `.Gte()` — hence the generated metamodel.

Package: `github.com/rodrigocelsius/go-nitro/codegen/blinq/v2`.

Useful struct tags:

- `db:"col"` — column name  
- `blinq:"index"` — `CREATE INDEX` in the migration **and** Resource Filterable/Sortable when lists are nil.  
  Use only on columns you **filter or sort by** (name, email, sku, status, FKs…). Do **not** put it on every field (description, notes, password, cost…) — excess indexes hurt writes and bloat the schema.  
  Scaffold: common filter names get `blinq:"index"` automatically; others need `field:type:index` (or omit with `:noindex`).
- `blinq:"unique"` or `unique:"true"` — UNIQUE + unique index  
- `blinq:"pk"` or `primary:"true"` — primary key (`id` is already PK by default)  
- `blinq:"soft"` / `DeletedAt` field — fills `Entity.SoftColumn`  
- `blinq:"tenant"` / `tenant_id` — fills `Entity.TenantColumn`  
- `//nitro:table=name` — table name override  

```go
type Product struct {
    ID     int64  `json:"id" db:"id"`                         // PK → automatic index
    Name   string `json:"name" db:"name" blinq:"index"`       // CREATE INDEX
    Email  string `json:"email" db:"email" blinq:"unique"`    // UNIQUE INDEX
    Active bool   `json:"active" db:"active" blinq:"index"`
}
```

## HTTP example (Postgres)

[`examples/v1/blinq-hello`](../../examples/v1/blinq-hello/) — **hello world**: model + schema + Store + Resource.

[`examples/v1/db-blinq`](../../examples/v1/db-blinq/) — CRUD via **Resource** + store directly (filters / sort; `blinq:"index"` → Filterable/Sortable).

[`examples/v1/db-blinq-handlers`](../../examples/v1/db-blinq-handlers/) — the same routes with handlers + DSL (`WhereIf`).

Filters/sort on Resource: `Filterable` / `FilterRules` / `Sortable`, **or** omit Filterable/Sortable to infer from `blinq:"index"` (+ `id`/`created_at`/`updated_at` on Sortable). Custom query:

Anti-DoS (defaults): `MaxPerPage=100`, `MaxPage=10000`, `MaxFilters=16`, `MaxFilterValueLen=200`. Tune via `ResourceConfig`.

```go
// *nitro.Ctx is already context.Context — pass c directly
store.Find(c, store.Query().
    WhereIf(active, ProductModel.Active.Eq(true)).
    WhereIf(q != "", ProductModel.Name.IContains(q)).
    Limit(50))
```

`WhereIf(cond, pred)` = shortcut for `if cond { q = q.Where(pred) }`.

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
cd examples/v1/blinq-hello && go run .

curl -X POST http://localhost:8080/hellos -d '{"message":"world"}'
curl http://localhost:8080/hellos
```

Demo without a database: `go run -tags memory .`

## Migration from `data/runtime`

The `data/runtime` package (old LINQ) was **removed** in BLinq Phase 2. Quick mapping:

| Before (`data/runtime`) | Now (BLinq) |
|------------------------|---------------|
| `runtime.From[T]("users")` | `query.FromName("users")` |
| `runtime.ColumnRef` | `field.Field[T]` / `field.Col[T](...)` |
| `*runtime.Query[T]` | `query.Query` (valor) |
| `runtime.PageResult` | `query.PageResult` / `PaginateCollect` |
| `runtime.MapToStruct` | `blinq/scan.MapToStruct` |
| `pgx.CompileSelect` | `blinq/provider/sql.Compile` + dialeto |
| `InsertAST` / `UpdateAST` | `query.InsertInto` / `Update` / `DeleteFrom` |

`pgx.Store[T]` and `repository.BaseRepository[T]` already use BLinq internally — apps that only used Store mostly change metamodels (`ColumnRef` → `Field[T]`).

## Features (P0/P1)

- Predicates And/Or/Not, WhereIf, OrderBy, Limit/Offset, Distinct  
- Soft delete / Tenant hints  
- Count / Sum / Avg / Min / Max + Paginate  
- Join / LeftJoin, GroupBy / Having  
- Upsert (ON CONFLICT / ON DUPLICATE / MSSQL MERGE), Bulk `Rows` + `Chunk`, `Inc`, `OmitReturning`, Store `CreateMany` / `UpdateWhere`  
- Dialects Postgres, SQLite, MySQL, MSSQL  
- QuoteIdent on SQL identifiers; `SuggestIndexes`; `sql.WithCache`  
- DX: `Any`/`All`, keyset `After`/`Before`, `RawWhere`, `CollectTyped`, `EqOuter` / JSON  
- Elastic / GraphQL / migrate MVP; Mongo driver in a submodule  

## Concurrent, Cache, and Hooks (P2)

```go
import (
    "time"

    "github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/cache"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

inner := memory.New("users", rows)
w := &provider.Wrap{
    Inner: inner,
    Cache: cache.New(), // pluggable — Redis pode implementar provider.Cache
    Hooks: []provider.Hook{provider.FuncHook{
        Before: func(ctx context.Context, q expr.Query) (context.Context, error) { return ctx, nil },
        After:  func(ctx context.Context, q expr.Query, res provider.Result, err error) {},
    }},
    // Telemetry: provider.NopTelemetry{}, // default se omitido
}

err := query.FromName("users").
    Where(user.Age.Gte(18)).
    Cache(5*time.Minute).
    CacheTag("users").
    Collect(ctx, w, &out)

// parallel filter (memory / ConcurrentCapable only)
_ = query.FromName("users").Collect(ctx, inner, &out, provider.WithConcurrency(8))
```

SQL providers ignore concurrency; the `Cache` hint only takes effect with `provider.Wrap` + `Cache` (or `sql.New(..., sql.WithCache(c))` / `postgres.New(db, sql.WithCache(c))`).

## Include, Subquery, Mongo/API/Elastic/GraphQL (P2)

```go
// 1:N
rel := field.HasMany("orders", OrdersEntity, user.ID.Ref(), order.UserID.Ref())
// N:1 (same Include batch)
rel = field.BelongsTo("user", UsersEntity, order.UserID.Ref(), user.ID.Ref())
// CLI: nitrogen make:relation Order belongs_to User
//      nitrogen make:relation User has_many Order  → modelgen/*relations_gen.go
rows, err := query.From(UsersEntity).Include(rel).CollectMaps(ctx, memory.NewMulti(tables))

// EXISTS / IN (SELECT …)
q := query.From(UsersEntity).Where(
    query.Exists(query.From(OrdersEntity).Where(order.Total.Gte(100))),
)
q = query.From(UsersEntity).Where(query.InQuery(user.ID, subSelectUserIDs))

// Mongo filter (no driver) / official driver in a submodule
find, _ := mongo.CompileFind(q.AST())
// p := mongodriver.New(db) // blinq/provider/mongo/driver

// Elastic Query DSL / GraphQL / API wire
es, _ := elastic.CompileSearch(q.AST())
gql, _ := graphql.CompileQuery(q.AST())
body, _ := api.Encode(q.AST())
```

### SQL migrations

From the model (goose style):

```bash
nitrogen migrate:make models.go --name create_hellos
# → migrations/YYYYMMDDHHMMSS_create_hellos.sql
```

Useful model tags: `blinq:"index"`, `unique:"true"`, `//nitro:notimestamps`, `//nitro:nosoftdelete`.

In the app:

```go
_ = db.MigrateUp(ctx) // applies ./migrations
```

Embed (single binary): `db.MigrateFS(ctx, migrationsFS)` — if the FS has a `migrations/` folder, it resolves it automatically.

Or an explicit runner:

```go
pool, _ := pgx.New(ctx, dsn)
r := &migrate.Runner{
    DB:     pgx.AdaptMigrate(pool),
    Source: migrate.GooseFSSource{FS: os.DirFS("migrations")},
}
_ = r.Up(ctx)
```

Generation: `codegen/migration`. Runner: `blinq/migrate`. Demo: [`examples/v1/blinq-hello`](../../examples/v1/blinq-hello/).

Full plan: [`plan_blinq.md`](../../plan_blinq.md).

## See also

- [Data layer guide](data-layer.md) — Store, Gateway, Repository, Pool  
- [`examples/v1/blinq-hello`](../../examples/v1/blinq-hello/) — hello world (Resource)  
- [`examples/v1/db-blinq`](../../examples/v1/db-blinq/) — CRUD + Resource  
- [`examples/v1/blinq-extras`](../../examples/v1/blinq-extras/) — MERGE / Mongo / Elastic / GraphQL / migrate  
- [`examples/v1/db-linq`](../../examples/v1/db-linq/) — same CRUD (legacy name; already BLinq)  
- [`blinq/README.md`](../../blinq/README.md)
