# Data Layer

**Language:** [English](data-layer.md) · [Português](../pt/guide/data-layer.md)

Nitro provides a complete data layer with **BLinq** (query builder + providers), **Gateway observability**, and a typed **Store**. PostgreSQL uses pgx; MySQL/MariaDB, SQL Server, and SQLite use `database/sql` through the same `sqladapter.OpenEnv` + Gateway.

> **Default:** `sqladapter.OpenEnv` (or `platform.Open` in CLI apps) → `StoreOf[T](db, "table")`, `nitrogen make <model>.go --data` (`modelgen`), typed queries (`WhereIf`, `Eq`, `OrderBy`). Set `NITRO_DB_DRIVER`. GORM, sqlx, sqlc, and raw SQL only when the requirement asks — still through the Gateway. Do not open the pool directly.
>
> **Go:** `data/adapter/pgx` and `data/adapter/sql` require **Go 1.25+**. Live multi-engine smoke: `make test-sql-integration`.

## Architecture

All SQL stacks (BLinq, GORM, sqlx, sqlc) go through the **Gateway**:

```
Your Code → BLinq | GORM | sqlx | sqlc → Gateway → PostgreSQL (pgx pool)
                                              ↓
                                    Trace / Metrics / Log / Cache
```

BLinq (typed path):

```
Your Code → Store[T] / Repository[T] → blinq/query → Provider SQL → Gateway → PostgreSQL (pgx)
```

## JSONB columns

First-class types (Valuer/Scanner + HTTP JSON):

```go
Missions nitro.StringSlice                    `json:"missions" db:"missions"`
Slots    nitro.JSONSlice[Window]              `json:"slots" db:"slots"`
Meta     nitro.JSON[map[string]any]           `json:"meta" db:"meta"`
Legacy   string                               `json:"x" db:"x,jsonb"` // force JSONB
```

`nitrogen migrate:make` generates `JSONB` (+ `DEFAULT '[]'::jsonb` for slices / `StringSlice` / `JSONSlice`). GIN index: `blinq:"gin"`.

**Do not** reimplement Valuer/Scanner in the app.

## BLinq Query Builder

A fluent, immutable, type-safe query builder that produces an AST (`blinq/expr`) compiled by `blinq/provider/sql`.

### Import

```go
import (
    "github.com/rodrigocelsius/go-nitro/blinq/v2/field"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)
```

### Creating Queries

```go
q := query.FromName("users")
// or:
// q := query.From(field.Entity{Name: "users"})
// via Store:
// store := pgx.NewStore[User](api, "users")
// q := store.Query()
```

### Filtering with Where

```go
q = q.Where(UserModel.Name.Eq("Joao"))

// Several at once
q = q.WhereAll(UserModel.Age.Gte(18), UserModel.Name.Like("Rod%"))

// Conditional (query-string filters)
q = q.WhereIf(activeOnly, UserModel.Active.Eq(true)).
    WhereIf(term != "", UserModel.Name.IContains(term))

// OR / NOT
q = q.Where(UserModel.Age.Lt(18).Or(UserModel.Age.Gte(65)))
q = q.Where(field.Not(UserModel.Active.Eq(false)))
```

### Column Operators

| Method | SQL Equivalent |
|--------|----------------|
| `Eq(value)` | `col = $N` |
| `Neq(value)` | `col <> $N` |
| `Gt(value)` | `col > $N` |
| `Gte(value)` | `col >= $N` |
| `Lt(value)` | `col < $N` |
| `Lte(value)` | `col <= $N` |
| `Like(pattern)` | `col LIKE $N` |
| `ILike(pattern)` | `col ILIKE $N` |
| `Contains(text)` | `col LIKE '%text%'` |
| `IContains(text)` | `col ILIKE '%text%'` |
| `In(values...)` | `col IN ($N, ...)` |
| `IsNull()` | `col IS NULL` |
| `IsNotNull()` | `col IS NOT NULL` |
| `Between(a, b)` | `col BETWEEN $N AND $N+1` |

### Ordering

```go
q = q.OrderBy(UserModel.Name.OrderAsc())
q = q.OrderBy(UserModel.CreatedAt.OrderDesc())
```

### Pagination

```go
// Using Page/PerPage (auto-calculates offset)
q = q.Page(2).PerPage(20).WithCount()

// Paginate shortcut
q = q.Paginate(2, 20) // Page+PerPage+WithCount

page, err := query.PaginateCollect[User](ctx, q, provider)
// page.Items, page.Total, page.Page, page.PerPage, page.TotalPages
```

### Soft delete & Tenant

```go
ent := field.Entity{Name: "users", SoftColumn: "deleted_at", TenantColumn: "tenant_id"}

// SELECT: AND deleted_at IS NULL AND tenant_id = $N
q := query.From(ent).Soft().Tenant(companyID)

// DELETE soft → UPDATE SET deleted_at = NOW()
err := query.DeleteFrom(ent).Soft().Where(user.ID.Eq(id)).Execute(ctx, p)
```

### Aggregations

```go
n, err := q.Count(ctx, p)                    // COUNT(*)
sum, err := q.Sum(ctx, p, order.Total.Ref()) // SUM(total)
avg, err := q.Avg(ctx, p, order.Total.Ref())
```

### Join / GroupBy

```go
q := query.From(users).
    Join(orders, user.ID.Ref(), order.UserID.Ref()).
    Select(user.Name.Ref(), order.Total.Ref())

q = query.FromName("users").
    GroupBy(user.Country.Ref()).
    SelectItems(
        query.FieldSelect(user.Country.Ref(), ""),
        query.AggSelect(expr.Aggregate{Func: expr.AggCount}, "n"),
    ).
    Having(user.Country.Neq(""))
```

### Upsert / Bulk / Inc

```go
query.UpsertInto(users).
    OnConflict(user.Email.Ref()).
    Values(user.Name.Set("Ada"), user.Email.Set("a@x.com")).
    DoUpdate(user.Name.Set("")). // EXCLUDED.name / VALUES(name) per dialect
    Execute(ctx, p)

query.InsertInto(users).
    Rows(row1, row2, /* … */).
    Chunk(1000).
    Execute(ctx, p)

// Bulk UPDATE (same SET for all matching rows). OmitReturning avoids loading rows.
query.Update(users).
    Set(user.Status.Set("archived")).
    Where(user.Active.Eq(false)).
    OmitReturning().
    Execute(ctx, p)

query.Update(users).Set(user.LoginCount.Inc(1)).Where(user.ID.Eq(id))

// Dialects: dialect.Postgres() | dialect.SQLite() | dialect.MySQL()
p := sql.New(adaptGateway(gw), dialect.SQLite())
```

| Feature | Postgres | SQLite | MySQL |
|---------|----------|--------|-------|
| Placeholder | `$N` | `?` | `?` |
| ILIKE | `ILIKE` | `LOWER LIKE` | `LOWER LIKE` |
| RETURNING | ✅ | ❌ | ❌ |
| Upsert | `ON CONFLICT` | `ON CONFLICT` | `ON DUPLICATE KEY` |
| DoNothing | ✅ | ✅ | ❌ (explicit error) |

### Selecting Columns

```go
q = q.Select(UserModel.ID.Ref(), UserModel.Name.Ref())
```

### Immutable Builder

Each method returns a **new** query — the original is never mutated:

```go
base := query.FromName("users").Where(UserModel.Status.Eq("active"))

// This does NOT modify `base`
activeAdults := base.Where(UserModel.Age.Gte(18))
activeAdmins := base.Where(UserModel.Role.Eq("admin"))

// base is still: WHERE status = 'active'
```

### Generating ASTs

```go
ast := q.AST() // expr.Query — compile with blinq/provider/sql
plan, err := q.Explain(ctx, provider)
```

## Gateway

The Gateway wraps database operations with **observability** (tracing, metrics, logging, caching) using the decorator pattern.

### Import

```go
import "github.com/rodrigocelsius/go-nitro/data/gateway/v2"
```

### Setup

```go
pool, err := pgxadapter.New(ctx, dsn) // Ping on connect
executor := pgxadapter.NewExecutor(pool)

// Defaults ON: slog logger (stderr), tracer/meter via domain.Default* (OTel after otel.Ensure), cache OFF
gw := gateway.New(pgxadapter.AdaptExecutor(executor))

// Opt-out:
// gateway.New(exec, gateway.DisableLogger())
// gateway.New(exec, gateway.DisableTracer(), gateway.DisableMeter())
// gateway.New(exec, gateway.DisableObservability()) // everything off
// Env: NITRO_DB_QUIET=1 on pgx.Open; NITRO_OTEL=0 turns off global OTel

// Custom / explicit OTel:
gw := gateway.New(exec,
    gateway.WithTracer(tracer),
    gateway.WithMeter(meter),
    gateway.WithLogger(logger),
    gateway.WithCache(cache),
)
```

| Option | Default | Turn off with |
|--------|---------|-------------|
| Logger (slog stderr) | ON — queries at DEBUG, slow ones (`NITRO_DB_SLOW_MS`, default 200 ms) at WARN; format/level follow `NITRO_LOG_FORMAT` / `NITRO_LOG_LEVEL` | `DisableLogger()` / `WithLogger(l)` / `WithSlowQuery(d)` |
| Tracer (OTel / nop) | ON | `DisableTracer()` / `DisableTelemetry` / `NITRO_OTEL=0` — DB spans are **children** of the HTTP span (`c.Context()`); export OTLP (`OTEL_EXPORTER_OTLP_*`); without an endpoint it will not go to the console (`NITRO_OTEL_STDOUT=1` debug only) |
| Meter (OTel / nop) | ON | `DisableMeter()` / `DisableTelemetry` / `NITRO_OTEL=0` — attrs `operation` + `status=ok\|error` |
| Cache | OFF | `DisableCache()` / only on with `WithCache` |
| Tudo | — | `DisableObservability()` / `NITRO_DB_QUIET=1` |

`gw.LoggerEnabled()` / `TracerEnabled()` / `MeterEnabled()` / `CacheEnabled()` reportam o estado.

Shutdown: `nitro.New` registers `OnShutdown` → OTel flush; or `nitro.ShutdownTelemetry(ctx)`.

### Query with Observability

```go
results, err := gw.Query(ctx, "SELECT * FROM users WHERE active = $1", []any{true})
// Automatically:
// 1. Checks cache (if configured)
// 2. Creates a trace span
// 3. Executes the query
// 4. Records metrics (duration, row count)
// 5. Logs the query
// 6. Caches the result
```

### Exec with Cache Invalidation

```go
n, err := gw.Exec(ctx, "insert", "INSERT INTO users (name) VALUES ($1)", []any{"Joao"})
// n = rows affected
```

### Gateway Pipeline

For `Query` (SELECT):
1. **Cache Check** — returns immediately on hit
2. **Trace Span** — creates OpenTelemetry span
3. **Execute** — runs SQL against PostgreSQL
4. **Scan** — converts rows to `[]map[string]any`
5. **Metrics** — records duration and row count
6. **Log** — structured log with operation details
7. **Cache Set** — stores result with TTL

For `Exec` (INSERT/UPDATE/DELETE):
1. **Trace Span** — with operation attribute
2. **Execute** — runs SQL
3. **Metrics**
4. **Log**
5. **Cache Invalidation** — parses SQL to extract table name, invalidates cache

### GORM / sqlx / sqlc (same Gateway — not the default)

Use these stacks **only** if the requirement asks. The default path remains BLinq (`StoreOf` + `modelgen`).

```go
db, err := pgxadapter.Open(ctx, pgxadapter.DSN("postgres://..."))
defer db.Close()
// NITRO_DB_QUIET=1 turns telemetry off automatically

sqlDB := db.SQL()                    // GORM / sqlx
q := sqlc.New(db.DBTX())             // sqlc
store := pgxadapter.StoreOf[T](db, "products") // BLinq
```

| Stack | Bootstrap |
|-------|-----------|
| Any | `pgx.Open` (+ `DSN`) |
| GORM / sqlx | `db.SQL()` |
| sqlc | `db.DBTX()` |
| BLinq | `StoreOf[T](db, table)` |
| Custom I/O | `db.Gateway.Instrument(...)` |

## Store[T] (lean CRUD)

For standard operations, use `pgx.Store[T]` — wraps DataAPI + BLinq:

```go
store := pgx.NewStoreFromPool[Product](pool, "products")
// or: store, pool, err := pgx.OpenStore[Product](ctx, dsn, "products")

items, err := store.Find(ctx, store.Query().
    WhereIf(active, ProductModel.Active.Eq(true)).
    WhereIf(q != "", ProductModel.Name.IContains(q)).
    OrderBy(ProductModel.CreatedAt.OrderDesc()).
    Limit(50))

p, err := store.Get(ctx, id)
p, err := store.CreateOne(ctx, &Product{Name: "K", Price: 10.0, Active: true})
// bulk:
created, err := store.CreateMany(ctx, []Product{
    {Name: "A", Price: 10, Active: true},
    {Name: "B", Price: 20, Active: true},
}, pgx.WithChunk(500)) // optional Chunk
// or typed: store.CreateSets(ctx, ProductModel.Name.Set("K"), ...)
// or map:    store.Create(ctx, map[string]any{"name": "K", "price": 10.0})
p, err := store.Update(ctx, id, ProductModel.Name.Set("K2"), ProductModel.Price.Set(12))
n, err := store.UpdateWhere(ctx,
    store.Query().Where(ProductModel.Active.Eq(false)),
    ProductModel.Name.Set("archived"),
) // bulk SET; no RETURNING
err = store.Delete(ctx, id)
```

`CreateMany` / `query.InsertInto().Rows().Chunk()` = bulk **insert**. `UpdateWhere` / `query.Update().Set().Where().OmitReturning()` = bulk **update** (same values for every matching row). `CreateMany` scans `RETURNING *` — keep chunks in the hundreds/thousands. One-shot dumps of millions of rows may still use Postgres `COPY` in importers; that does not replace BLinq in the Go app.

### Resource + Store (filtros no SQL)

`*pgx.Store` implements `AsResourceRepository` / `QueryableLister` — pass the store directly.
`pgx.AdaptStore` remains valid if you need the explicit wrapper.

Equality filters (`?name=x`) e `?sort=` become BLinq predicates (no 10k in-memory window).
Fields with `blinq:"index"` feed Filterable + Sortable when the config lists are empty/`nil`.

```go
app.Resource(&Product{}, store, &nitro.ResourceConfig{
    NoCache: true,
    // Filterable/Sortable omitted → inferred from blinq:"index"
    FilterRules: []nitro.FilterRule{
        {Param: "min_price", Field: "price", Op: "gte"},
    },
})
// explicit path:
app.Resource("/products", &Product{}, store, &nitro.ResourceConfig{NoCache: true})
```

Repos: `MemoryRepo` still post-filters in memory. Repos that implement `domain.QueryableLister` push down.

### Open / OpenEnv

```go
db, err := pgx.OpenEnv(ctx)       // DATABASE_URL | POSTGRES_DSN
db, err := pgx.Open(ctx, "")      // idem
db, err := pgx.Open(ctx, pgx.DSN("postgres://…fallback…"))
```

Row conversion (`map[string]any` → struct) uses `blinq/scan` (`AsString` / `AsBool` / `AsInt64` / `AsFloat64` / `AsTime` e `MapToStruct` with `db` tags).

## Repository

Generic CRUD repository over the Gateway + BLinq query builder.

### Interface

```go
type Repository[T any] interface {
    FindByID(ctx context.Context, id any) (*T, error)
    Find(ctx context.Context, q query.Query) ([]T, error)
    FindOne(ctx context.Context, q query.Query) (*T, error)
    Count(ctx context.Context, q query.Query) (int, error)
    List(ctx context.Context, q query.Query) (*query.PageResult[T], error)
    Create(ctx context.Context, entity *T) error
    Update(ctx context.Context, entity *T) error
    Delete(ctx context.Context, id any) error
}
```

### Usage

```go
import "github.com/rodrigocelsius/go-nitro/data/repository/v2"

type User struct {
    ID    int    `json:"id" db:"id"`
    Name  string `json:"name" db:"name"`
    Email string `json:"email" db:"email"`
    Age   int    `json:"age" db:"age"`
}

// Create repository
repo := repository.NewBaseRepository[User](gw, "users", "id")

// Find by ID
user, err := repo.FindByID(ctx, 42)

// Find with query
q := query.FromName("users").Where(UserModel.Age.Gte(18))
users, err := repo.Find(ctx, q)

// Find one
q = query.FromName("users").Where(UserModel.Email.Eq("joao@test.com"))
user, err := repo.FindOne(ctx, q)

// Count
total, err := repo.Count(ctx, q)

// List with pagination
q = q.Page(1).PerPage(20).WithCount()
result, err := repo.List(ctx, q)
// result.Items      []User
// result.Total      int
// result.Page       int
// result.PerPage    int
// result.TotalPages int

// Create
user = &User{Name: "Joao", Email: "joao@test.com", Age: 30}
err = repo.Create(ctx, user)

// Update
user.Name = "Joao Updated"
err = repo.Update(ctx, user)

// Delete
err = repo.Delete(ctx, user.ID)
```

### Paginated Result

```go
type PageResult[T any] struct {
    Items      []T
    Total      int64
    Page       int
    PerPage    int
    TotalPages int
}
```

## PostgreSQL Adapter

The `pgx` adapter provides connection pooling, query execution, and transactions.

### Import

```go
import pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
```

### Connection Pool

```go
pool, err := pgxadapter.New(ctx, "postgres://localhost/mydb?sslmode=disable")
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

// optional Max/Min conns
pool, err = pgxadapter.NewWithConfig(ctx, dsn, pgxadapter.PoolConfig{MaxConns: 20, MinConns: 2})

// Or from existing pool
pool = pgxadapter.NewFromPool(existingPool)

_ = pool.Ping(ctx)
_ = pool.Stat() // pgxpool.Stat
```

### Pool sizing (all engines)

Every engine gets **in-process pooling by default** — the fallback when no external pooler is configured: requests beyond the pool size wait inside the process for a free connection instead of opening new ones until the server refuses them.

| Env | Default | Notes |
|-----|---------|-------|
| `NITRO_DB_MAX_CONNS` | pgx: `max(4, NumCPU)` · MySQL/MariaDB/SQL Server: `max(10, 4×NumCPU)` · SQLite file: 4 | Keep `replicas × MAX_CONNS` below the server limit (`max_connections`; MySQL = 151). `0` = unlimited (database/sql engines) |
| `NITRO_DB_MIN_CONNS` | 0 | pgx only |
| `NITRO_DB_MAX_IDLE_CONNS` | 10 | `database/sql` only (Go's default is 2 → churn under bursts) |
| `NITRO_DB_CONN_MAX_LIFETIME` | pgx: 1h · sql: 30m | Below proxy / load-balancer idle timeouts |
| `NITRO_DB_CONN_MAX_IDLE_TIME` | pgx: 30m · sql: 5m | |

`PoolConfig{MaxConns, MinConns}` and `pool_max_conns=` in the DSN still win. In-memory SQLite keeps the `database/sql` defaults.

**SQLite files** also get `_pragma=busy_timeout(5000)`, `_pragma=journal_mode(WAL)` and `_txlock=immediate` unless the DSN sets them (`NITRO_SQLITE_PRAGMAS=0` opts out): concurrent writers queue for the lock instead of failing with `database is locked`, and readers do not block the writer.

### Best option per database

| Database | Many app instances / high concurrency | Without it (Nitro fallback) |
|----------|----------------------------------------|-----------------------------|
| PostgreSQL | **PgBouncer** (transaction pooling) — or RDS Proxy / Supavisor / pgcat, which speak the same protocol | pgxpool per process; PgBouncer mode is enabled automatically on port 6432 |
| MySQL / MariaDB | **ProxySQL** (MySQL & MariaDB) or **MaxScale** (MariaDB); RDS Proxy on AWS | bounded `database/sql` pool + connection recycling |
| SQL Server | the driver pools already; **Azure SQL** / SQL Server raise `max worker threads` — no external pooler needed | bounded `database/sql` pool |
| SQLite | single writer by design: one instance, or move to a server database | busy_timeout + WAL + small pool |

### PgBouncer (transaction pooling)

Enable with **one** of: `?pgbouncer=true` in `DATABASE_URL`, `NITRO_DB_PGBOUNCER=1`, `PoolConfig{PgBouncer: true}` — or nothing at all when the DSN uses port **6432** (PgBouncer's default; `NITRO_DB_PGBOUNCER=0` opts out). Nitro then:

- uses the **simple query protocol** (`default_query_exec_mode=simple_protocol`): pgx's default named prepared statements break when PgBouncer hands the next statement to another server connection (`prepared statement "stmtcache_…" does not exist`). An explicit `default_query_exec_mode` in the DSN wins (e.g. `cache_describe` with PgBouncer ≥ 1.21 and `max_prepared_statements`);
- runs migrations over **`DATABASE_DIRECT_URL`** (or `NITRO_DB_DIRECT_URL`) — a DSN straight to Postgres — because the migration lock is a session-level `pg_advisory_lock`. Without it, migrations run without the lock (log warning): run them from a single instance;
- `nitro.InTx` / Resource transactions work unchanged (one transaction = one server connection in PgBouncer).

Avoid session state through PgBouncer: `SET`, `LISTEN`, temporary tables and session advisory locks do not survive between transactions. `search_path` as a startup parameter needs `track_extra_parameters = search_path` (PgBouncer ≥ 1.20) or `ignore_startup_parameters`.

Retryable conflicts (deadlock / serialization: Postgres 40001/40P01, MySQL 1213/1205) surface as 409 `tx_conflict` — see [transactions.md](transactions.md).

### Migrations (blinq/migrate)

```go
r := &migrate.Runner{
    DB:     pgxadapter.AdaptMigrate(pool),
    Source: migrate.FSSource{FS: migrationsFS},
}
_ = r.Up(ctx)
```

### Executor

```go
executor := pgxadapter.NewExecutor(pool)

// Query
rows, err := executor.Query(ctx, "SELECT * FROM users WHERE id = $1", []any{42})

// Query single row
row := executor.QueryRow(ctx, "SELECT * FROM users WHERE id = $1", []any{42})

// Execute (INSERT/UPDATE/DELETE)
tag, err := executor.Exec(ctx, "UPDATE users SET name = $1 WHERE id = $2", []any{"Joao", 42})
```

### Transactions

**Preferred (Store + all engines):**

```go
err := db.Tx(ctx, func(tx *sqladapter.DB) error {
    users := sqladapter.StoreOf[User](tx, "users")
    orders := sqladapter.StoreOf[Order](tx, "orders")
    u, err := users.CreateOne(ctx, &User{Name: "Ada"})
    if err != nil {
        return err
    }
    _, err = orders.CreateOne(ctx, &Order{UserID: u.ID, Total: 10})
    return err
})
```

Works for PostgreSQL (pgx) and MySQL/MariaDB/SQL Server/SQLite (`database/sql`). On error or panic the transaction is rolled back.

**Low-level (raw SQL on pgx Executor):**

```go
tx, err := executor.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx)
// ...
return tx.Commit(ctx)
```

## SQL Compilation

Compile ASTs to raw SQL (PostgreSQL syntax with `$N` placeholders):

```go
import (
    blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

q := query.FromName("users").Where(UserModel.Age.Gte(18))
c, err := blinqSQL.Compile(q.AST(), dialect.Postgres())
// c.SQL  = "SELECT * FROM users WHERE age >= $1"
// c.Args = [18]
```

## Code Generation

Generate type-safe column fields from Go structs:

```go
//go:build ignore

package main

import "github.com/rodrigocelsius/go-nitro/codegen/data/v2"

func main() {
    models, err := data.ParseModels("user.go")
    // Default: DSL in modelgen/.
    // --same-package: user_model_gen.go (legacy, same package).
    // - Table name / Entity
    // - field.Field[T] for each column
    // - Well-known fields (ID, CreatedAt, UpdatedAt, DeletedAt)
}
```

### Annotations

Use struct tags and comments to control code generation:

```go
//nitro:table=app_users  // Override table name
type User struct {
    ID    int       `db:"id"`
    Name  string    `db:"name"`
    Email string    `db:"email"`

    //nitro:notimestamps  // Skip CreatedAt/UpdatedAt injection
    CustomField string `db:"custom_field"`

    //nitro:noid  // Skip ID field injection
    NoIDField string `db:"no_id_field"`
}
```

## Full Example

```go
package main

import (
    "context"
    "log"

    pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
    nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//nitro:table=products
type Product struct {
    nitro.Model // UUID v7 + timestamps + soft delete
    Name   string  `json:"name" db:"name" validate:"required"`
    Price  float64 `json:"price" db:"price" validate:"gte=0"`
    Active bool    `json:"active" db:"active" default:"true"`
}

func main() {
    ctx := context.Background()
    db, err := pgxadapter.Open(ctx, pgxadapter.DSN("postgres://localhost/mydb?sslmode=disable"))
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    if err := db.MigrateUp(ctx); err != nil { // ./migrations
        log.Fatal(err)
    }

    store := pgxadapter.StoreOf[Product](db, "products")

    app := nitro.New()
    app.Resource(&Product{}, store, &nitro.ResourceConfig{
        NoCache: true,
        FilterRules: []nitro.FilterRule{
            {Param: "min_price", Field: "price", Op: "gte"},
        },
    })
    nitro.Ok(app.Listen(":8080"))
}
```

The low-level pattern (`pgx.New` + `gateway.New` + `BaseRepository`) still exists for advanced cases, but **apps should prefer** `OpenEnv` → `MigrateUp` → `StoreOf` → `Resource(&M{}, store)`.

## See Also

- [`docs/guide/blinq.md`](blinq.md) — DSL, providers, migrate, Elastic/GraphQL/Mongo  
- [`examples/v1/blinq-hello/`](../../examples/v1/blinq-hello/) — hello world BLinq + Resource  
- [`examples/v1/db-blinq/`](../../examples/v1/db-blinq/) — BLinq + Store + Gateway + Resource  
- [`examples/v1/db-blinq-handlers/`](../../examples/v1/db-blinq-handlers/) — BLinq with custom routes (DSL / WhereIf)  
- [`examples/v1/blinq-extras/`](../../examples/v1/blinq-extras/) — MERGE / compile Mongo·ES·GQL / migrate  
- [`examples/v1/db-linq/`](../../examples/v1/db-linq/) — same CRUD (legacy name; already BLinq)  
- [`examples/v1/db-gorm/`](../../examples/v1/db-gorm/) — GORM + Gateway (`OpenSQL`)  
- [`examples/v1/db-sqlx/`](../../examples/v1/db-sqlx/) — sqlx + Gateway (`OpenSQL`)  
- [`examples/v1/db-sqlc/`](../../examples/v1/db-sqlc/) — sqlc + Gateway (`AdaptDBTX`)  
- [`examples/v1/README-db.md`](../../examples/v1/README-db.md) — overview  
- [`examples/cleanarch/`](../../examples/cleanarch/) — Clean architecture with data layer
