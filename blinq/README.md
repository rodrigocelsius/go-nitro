# BLinq

DSL tipada de consulta para Go — **AST + Providers**.

- **Fase 1:** core (`expr`, `field`, `query`) + provider **memory**
- **Fase 2:** `provider/sql` (dialect Postgres) executando sobre o Nitro Gateway
- **Fase 3:** agregações, `Paginate`, `.Soft()`, `.Tenant()`
- **Fase 4:** `Join`/`LeftJoin`, `GroupBy`/`Having`, Upsert, Bulk+Chunk, `Inc`
- **Fase 5:** dialetos `Postgres` / `SQLite` / `MySQL`
- **Fase 6:** codegen Fields + `examples/v1/db-blinq` + guide
- **Fase 7:** `WithConcurrency`, `Cache`/`CacheTag`, `provider.Wrap` (hooks/telemetry)
- **Fase 8:** Include batch, Exists/InQuery, `ForEach`, `provider/mongo` + `provider/api`
- **Fase 10:** OuterRef/JSON, MSSQL dialect, `SuggestIndexes`, Resource FilterRules
- **Fase 11:** MSSQL MERGE; Elastic/GraphQL; `mongo/driver` + `elastic/client` submodules; `blinq/migrate`

```go
p := sql.New(db, dialect.MySQL())   // ou dialect.SQLite() / dialect.MSSQL()
```

## Exemplo (memory)

```go
import (
    "context"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/field"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

p := memory.New("users", rows)
var out []map[string]any
err := query.From(field.Entity{Name: "users"}).
    Where(user.Age.Gte(18).And(user.Country.Eq("BR"))).
    OrderBy(user.Age.OrderDesc()).
    Limit(10).
    Collect(ctx, p, &out)
```

## Exemplo (SQL via Gateway)

```go
import (
    blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
    pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
)

p := blinqSQL.New(pgxadapter.AdaptGateway(gw), dialect.Postgres())
plan, _ := query.FromName("users").Where(user.Age.Gte(18)).Explain(ctx, p)
// plan.Text = "SELECT * FROM users WHERE age >= $1"
```

No Nitro, use `pgx.Store[T]` / `DataAPI` — já compilam via BLinq.

## Layout

```text
blinq/
  expr/       AST pública (ASTVersion=1)
  field/      Field[T], Pred, Entity
  query/      DSL imutável (+ Page/PerPage/WithCount)
  scan/       MapToStruct / MapsToSlice
  migrate/    Runner Up/Down/Status + FSSource
  provider/   Provider, SQLExecutor, Wrap (cache/hooks/telemetry)
  provider/memory/
  provider/cache/     # Memory in-process (provider.Cache)
  provider/mongo/     # CompileFind / CompileLookup (sem driver)
  provider/mongo/driver/  # submodule: mongo-driver
  provider/elastic/   # CompileSearch → Query DSL
  provider/elastic/client/ # submodule: go-elasticsearch
  provider/graphql/   # CompileQuery + Poster
  provider/api/       # wire JSON + Poster
  provider/sql/
  provider/sql/dialect/
codegen/blinq/       # nitrogen make data → Field[T] + Entity
codegen/migration/   # gera .sql goose; use GooseFSSource ou converter para .up/.down
```

## Extras

`examples/v1/blinq-extras` — MERGE / Mongo / Elastic / GraphQL / migrate demos.

## Testes

```bash
cd blinq && go test ./...
cd blinq/provider/mongo/driver && go test ./...
cd blinq/provider/elastic/client && go test ./...
```

Ver `plan_blinq.md` na raiz do monorepo.
