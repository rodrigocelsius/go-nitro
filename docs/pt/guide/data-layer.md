# Data Layer

**Idioma:** [Português](data-layer.md) · [English](../../guide/data-layer.md)

Nitro oferece uma camada de dados completa com **BLinq**, **Gateway** e **Store** tipado. PostgreSQL usa pgx; MySQL/MariaDB, SQL Server e SQLite usam `database/sql` via `sqladapter.OpenEnv`.

> **Padrão:** `sqladapter.OpenEnv` (ou `platform.Open` nos apps da CLI) → `StoreOf[T](db, "tabela")`, `nitrogen make <model>.go --data` (`modelgen`). Defina `NITRO_DB_DRIVER`. GORM, sqlx, sqlc e SQL cru só quando o requisito pedir — ainda pelo Gateway.
>
> **Go:** `data/adapter/pgx` e `data/adapter/sql` exigem **Go 1.25+**. Smoke multi-engine: `make test-sql-integration`.

## Architecture

Todas as stacks SQL (BLinq, GORM, sqlx, sqlc) passam pelo **Gateway**:

```
Your Code → BLinq | GORM | sqlx | sqlc → Gateway → PostgreSQL (pgx pool)
                                              ↓
                                    Trace / Metrics / Log / Cache
```

BLinq (caminho tipado):

```
Your Code → Store[T] / Repository[T] → blinq/query → Provider SQL → Gateway → PostgreSQL (pgx)
```

## JSONB columns

Tipos de primeira classe (Valuer/Scanner + JSON HTTP):

```go
Missions nitro.StringSlice                    `json:"missions" db:"missions"`
Slots    nitro.JSONSlice[Window]              `json:"slots" db:"slots"`
Meta     nitro.JSON[map[string]any]           `json:"meta" db:"meta"`
Legacy   string                               `json:"x" db:"x,jsonb"` // força JSONB
```

`nitrogen migrate:make` gera `JSONB` (+ `DEFAULT '[]'::jsonb` para slices / `StringSlice` / `JSONSlice`). Índice GIN: `blinq:"gin"`.

**Não** reimplementar Valuer/Scanner na app.

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
// ou:
// q := query.From(field.Entity{Name: "users"})
// via Store:
// store := pgx.NewStore[User](api, "users")
// q := store.Query()
```

### Filtering with Where

```go
q = q.Where(UserModel.Name.Eq("Joao"))

// Várias de uma vez
q = q.WhereAll(UserModel.Age.Gte(18), UserModel.Name.Like("Rod%"))

// Condicional (filtros de query string)
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

// Atalho Paginate
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
    DoUpdate(user.Name.Set("")). // EXCLUDED.name / VALUES(name) conforme dialeto
    Execute(ctx, p)

query.InsertInto(users).
    Rows(row1, row2, /* … */).
    Chunk(1000).
    Execute(ctx, p)

// UPDATE em massa (mesmo SET em todas as linhas do WHERE). OmitReturning não carrega as linhas.
query.Update(users).
    Set(user.Status.Set("archived")).
    Where(user.Active.Eq(false)).
    OmitReturning().
    Execute(ctx, p)

query.Update(users).Set(user.LoginCount.Inc(1)).Where(user.ID.Eq(id))

// Dialetos: dialect.Postgres() | dialect.SQLite() | dialect.MySQL()
p := sql.New(adaptGateway(gw), dialect.SQLite())
```

| Feature | Postgres | SQLite | MySQL |
|---------|----------|--------|-------|
| Placeholder | `$N` | `?` | `?` |
| ILIKE | `ILIKE` | `LOWER LIKE` | `LOWER LIKE` |
| RETURNING | ✅ | ❌ | ❌ |
| Upsert | `ON CONFLICT` | `ON CONFLICT` | `ON DUPLICATE KEY` |
| DoNothing | ✅ | ✅ | ❌ (erro explícito) |

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
ast := q.AST() // expr.Query — compile com blinq/provider/sql
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
pool, err := pgxadapter.New(ctx, dsn) // Ping na conexão
executor := pgxadapter.NewExecutor(pool)

// Defaults ON: logger slog (stderr), tracer/meter via domain.Default* (OTel após otel.Ensure), cache OFF
gw := gateway.New(pgxadapter.AdaptExecutor(executor))

// Opt-out:
// gateway.New(exec, gateway.DisableLogger())
// gateway.New(exec, gateway.DisableTracer(), gateway.DisableMeter())
// gateway.New(exec, gateway.DisableObservability()) // tudo off
// Env: NITRO_DB_QUIET=1 no pgx.Open; NITRO_OTEL=0 desliga OTel global

// Custom / OTel explícito:
gw := gateway.New(exec,
    gateway.WithTracer(tracer),
    gateway.WithMeter(meter),
    gateway.WithLogger(logger),
    gateway.WithCache(cache),
)
```

| Option | Default | Desliga com |
|--------|---------|-------------|
| Logger (slog stderr) | ON | `DisableLogger()` |
| Tracer (OTel / nop) | ON | `DisableTracer()` / `DisableTelemetry` / `NITRO_OTEL=0` — spans DB são **filhos** do span HTTP (`c.Context()`); export OTLP (`OTEL_EXPORTER_OTLP_*`); sem endpoint não vai ao console (`NITRO_OTEL_STDOUT=1` só debug) |
| Meter (OTel / nop) | ON | `DisableMeter()` / `DisableTelemetry` / `NITRO_OTEL=0` — attrs `operation` + `status=ok\|error` |
| Cache | OFF | `DisableCache()` / só liga com `WithCache` |
| Tudo | — | `DisableObservability()` / `NITRO_DB_QUIET=1` |

`gw.LoggerEnabled()` / `TracerEnabled()` / `MeterEnabled()` / `CacheEnabled()` reportam o estado.

Shutdown: `nitro.New` registra `OnShutdown` → flush OTel; ou `nitro.ShutdownTelemetry(ctx)`.

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

### GORM / sqlx / sqlc (mesmo Gateway — não é o padrão)

Use estas stacks **somente** se o requisito pedir. O caminho padrão continua sendo BLinq (`StoreOf` + `modelgen`).

```go
db, err := pgxadapter.Open(ctx, pgxadapter.DSN("postgres://..."))
defer db.Close()
// NITRO_DB_QUIET=1 desliga telemetria automaticamente

sqlDB := db.SQL()                    // GORM / sqlx
q := sqlc.New(db.DBTX())             // sqlc
store := pgxadapter.StoreOf[T](db, "products") // BLinq
```

| Stack | Bootstrap |
|-------|-----------|
| Qualquer | `pgx.Open` (+ `DSN`) |
| GORM / sqlx | `db.SQL()` |
| sqlc | `db.DBTX()` |
| BLinq | `StoreOf[T](db, table)` |
| I/O custom | `db.Gateway.Instrument(...)` |

## Store[T] (CRUD enxuto)

Para operações padrão, use `pgx.Store[T]` — encapsula DataAPI + BLinq:

```go
store := pgx.NewStoreFromPool[Product](pool, "products")
// ou: store, pool, err := pgx.OpenStore[Product](ctx, dsn, "products")

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
}, pgx.WithChunk(500)) // Chunk opcional
// ou tipado: store.CreateSets(ctx, ProductModel.Name.Set("K"), ...)
// ou map:    store.Create(ctx, map[string]any{"name": "K", "price": 10.0})
p, err := store.Update(ctx, id, ProductModel.Name.Set("K2"), ProductModel.Price.Set(12))
n, err := store.UpdateWhere(ctx,
    store.Query().Where(ProductModel.Active.Eq(false)),
    ProductModel.Name.Set("archived"),
) // UPDATE em massa; sem RETURNING
err = store.Delete(ctx, id)
```

`CreateMany` / `query.InsertInto().Rows().Chunk()` = **insert** em massa. `UpdateWhere` / `query.Update().Set().Where().OmitReturning()` = **update** em massa (os mesmos valores em todas as linhas do filtro). `CreateMany` faz scan de `RETURNING *` — mantenha chunks na casa das centenas/milhares. Dumps de milhões de linhas podem usar `COPY` no importer; isso não substitui BLinq no app Go.

### Resource + Store (filtros no SQL)

`*pgx.Store` implementa `AsResourceRepository` / `QueryableLister` — passe o store direto.
`pgx.AdaptStore` continua válido se precisar do wrapper explícito.

Equality filters (`?name=x`) e `?sort=` viram predicados BLinq (sem janela 10k in-memory).
Campos com `blinq:"index"` alimentam Filterable + Sortable quando as listas do config estão vazias/`nil`.

```go
app.Resource(&Product{}, store, &nitro.ResourceConfig{
    NoCache: true,
    // Filterable/Sortable omitidos → inferência blinq:"index"
    FilterRules: []nitro.FilterRule{
        {Param: "min_price", Field: "price", Op: "gte"},
    },
})
// path explícito:
app.Resource("/products", &Product{}, store, &nitro.ResourceConfig{NoCache: true})
```

Reposos: `MemoryRepo` continua com post-filter in-memory. Repos que implementam `domain.QueryableLister` fazem push-down.

### Open / OpenEnv

```go
db, err := pgx.OpenEnv(ctx)       // DATABASE_URL | POSTGRES_DSN
db, err := pgx.Open(ctx, "")      // idem
db, err := pgx.Open(ctx, pgx.DSN("postgres://…fallback…"))
```

Conversão de linhas (`map[string]any` → struct) usa `blinq/scan` (`AsString` / `AsBool` / `AsInt64` / `AsFloat64` / `AsTime` e `MapToStruct` com tags `db`).

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

// Max/Min conns opcionais
pool, err = pgxadapter.NewWithConfig(ctx, dsn, pgxadapter.PoolConfig{MaxConns: 20, MinConns: 2})

// Or from existing pool
pool = pgxadapter.NewFromPool(existingPool)

_ = pool.Ping(ctx)
_ = pool.Stat() // pgxpool.Stat
```

### Tamanho do pool (todos os bancos)

Todo banco tem **pooling em processo por padrão** — o fallback quando não há pooler externo: requisições além do tamanho do pool esperam no processo por uma conexão livre, em vez de abrir conexões novas até o servidor recusar.

| Env | Padrão | Observações |
|-----|--------|-------------|
| `NITRO_DB_MAX_CONNS` | pgx: `max(4, NumCPU)` · MySQL/MariaDB/SQL Server: `max(10, 4×NumCPU)` · SQLite arquivo: 4 | Mantenha `réplicas × MAX_CONNS` abaixo do limite do servidor (`max_connections`; MySQL = 151). `0` = ilimitado (bancos `database/sql`) |
| `NITRO_DB_MIN_CONNS` | 0 | só pgx |
| `NITRO_DB_MAX_IDLE_CONNS` | 10 | só `database/sql` (o padrão do Go é 2 → abre/fecha conexões em rajadas) |
| `NITRO_DB_CONN_MAX_LIFETIME` | pgx: 1h · sql: 30m | Abaixo do timeout de proxies / load balancers |
| `NITRO_DB_CONN_MAX_IDLE_TIME` | pgx: 30m · sql: 5m | |

`PoolConfig{MaxConns, MinConns}` e `pool_max_conns=` no DSN continuam tendo prioridade. SQLite em memória mantém os padrões do `database/sql`.

**SQLite em arquivo** também recebe `_pragma=busy_timeout(5000)`, `_pragma=journal_mode(WAL)` e `_txlock=immediate` quando o DSN não os define (`NITRO_SQLITE_PRAGMAS=0` desliga): escritores concorrentes esperam o lock em vez de falhar com `database is locked`, e leitores não bloqueiam o escritor.

### Melhor opção por banco

| Banco | Muitas instâncias / alta concorrência | Sem isso (fallback do Nitro) |
|-------|---------------------------------------|------------------------------|
| PostgreSQL | **PgBouncer** (transaction pooling) — ou RDS Proxy / Supavisor / pgcat, que falam o mesmo protocolo | pgxpool por processo; o modo PgBouncer liga sozinho na porta 6432 |
| MySQL / MariaDB | **ProxySQL** (MySQL e MariaDB) ou **MaxScale** (MariaDB); RDS Proxy na AWS | pool `database/sql` limitado + reciclagem de conexões |
| SQL Server | o driver já faz pooling; **Azure SQL** / SQL Server aumentam `max worker threads` — sem pooler externo | pool `database/sql` limitado |
| SQLite | um escritor por design: uma instância, ou migre para um banco servidor | busy_timeout + WAL + pool pequeno |

### PgBouncer (transaction pooling)

Ative com **um** de: `?pgbouncer=true` no `DATABASE_URL`, `NITRO_DB_PGBOUNCER=1`, `PoolConfig{PgBouncer: true}` — ou nada, quando o DSN usa a porta **6432** (padrão do PgBouncer; `NITRO_DB_PGBOUNCER=0` desliga). O Nitro então:

- usa o **protocolo simples** (`default_query_exec_mode=simple_protocol`): os prepared statements nomeados (padrão do pgx) quebram quando o PgBouncer entrega o próximo comando a outra conexão do servidor (`prepared statement "stmtcache_…" does not exist`). Um `default_query_exec_mode` explícito no DSN tem prioridade (ex.: `cache_describe` com PgBouncer ≥ 1.21 e `max_prepared_statements`);
- roda as migrations via **`DATABASE_DIRECT_URL`** (ou `NITRO_DB_DIRECT_URL`) — DSN direto no Postgres — porque o lock de migration é um `pg_advisory_lock` de sessão. Sem ele, as migrations rodam sem lock (com aviso no log): rode-as de uma única instância;
- `nitro.InTx` / transações do Resource funcionam sem mudança (uma transação = uma conexão do servidor no PgBouncer).

Evite estado de sessão através do PgBouncer: `SET`, `LISTEN`, tabelas temporárias e advisory locks de sessão não sobrevivem entre transações. `search_path` como parâmetro de startup exige `track_extra_parameters = search_path` (PgBouncer ≥ 1.20) ou `ignore_startup_parameters`.

Conflitos transitórios (deadlock / serialização: Postgres 40001/40P01, MySQL 1213/1205) retornam 409 `tx_conflict` — veja [transactions.md](transactions.md).

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

**Preferido (Store + todos os engines):**

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

Funciona em PostgreSQL (pgx) e MySQL/MariaDB/SQL Server/SQLite. Em erro ou panic faz rollback.

**Baixo nível (SQL cru no Executor pgx):**

```go
tx, err := executor.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx)
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
    // Default: DSL em modelgen/.
    // --same-package: user_model_gen.go (legado, mesmo pacote).
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

Padrão de baixo nível (`pgx.New` + `gateway.New` + `BaseRepository`) ainda existe para casos avançados, mas **apps devem preferir** `OpenEnv` → `MigrateUp` → `StoreOf` → `Resource(&M{}, store)`.

## See Also

- [`docs/guide/blinq.md`](blinq.md) — DSL, providers, migrate, Elastic/GraphQL/Mongo  
- [`examples/v1/blinq-hello/`](../../examples/v1/blinq-hello/) — hello world BLinq + Resource  
- [`examples/v1/db-blinq/`](../../examples/v1/db-blinq/) — BLinq + Store + Gateway + Resource  
- [`examples/v1/db-blinq-handlers/`](../../examples/v1/db-blinq-handlers/) — BLinq com rotas custom (DSL / WhereIf)  
- [`examples/v1/blinq-extras/`](../../examples/v1/blinq-extras/) — MERGE / compile Mongo·ES·GQL / migrate  
- [`examples/v1/db-linq/`](../../examples/v1/db-linq/) — mesmo CRUD (nome legado; já é BLinq)  
- [`examples/v1/db-gorm/`](../../examples/v1/db-gorm/) — GORM + Gateway (`OpenSQL`)  
- [`examples/v1/db-sqlx/`](../../examples/v1/db-sqlx/) — sqlx + Gateway (`OpenSQL`)  
- [`examples/v1/db-sqlc/`](../../examples/v1/db-sqlc/) — sqlc + Gateway (`AdaptDBTX`)  
- [`examples/v1/README-db.md`](../../examples/v1/README-db.md) — overview  
- [`examples/cleanarch/`](../../examples/cleanarch/) — Clean architecture with data layer
