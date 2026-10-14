# BLinq

**Idioma:** [Português](blinq.md) · [English](../../guide/blinq.md)

DSL tipada de consulta para Go — **AST + Providers** — incubada em `github.com/rodrigocelsius/go-nitro/blinq/v2`.

> **Padrão Nitro:** BLinq é o caminho oficial de acesso a dados com Postgres (`StoreOf`, `modelgen`, `Resource` / `WhereIf`). Não escreva SQL cru nem escolha GORM/sqlx/sqlc a menos que o requisito peça.

BLinq separa *o quê* (query imutável / AST) de *onde* (provider memory, SQL Postgres/SQLite/MySQL sobre o Nitro Gateway).

## Quick start

```go
import (
    "github.com/rodrigocelsius/go-nitro/blinq/v2/field"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/query"
    blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
    "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
)

ent := field.Entity{Name: "users", SoftColumn: "deleted_at"}
// SoftColumn liga soft automaticamente — Soft() é opcional; Hard() desliga.
q := query.From(ent).Where(...)
user := struct {
    Age field.Field[int]
}{Age: field.Col[int]("users", "age")}

q := query.From(ent).Soft().Where(user.Age.Gte(18)).Limit(50)
plan, _ := q.Explain(ctx, sql.New(pgx.AdaptGateway(gw), dialect.Postgres()))
```

## Pacotes

| Pacote | Papel |
|--------|--------|
| `blinq/expr` | AST pública (`ASTVersion=1`) |
| `blinq/field` | `Field[T]`, `Entity`, predicados |
| `blinq/query` | DSL imutável + Paginate / Soft / Tenant / Join / Upsert |
| `blinq/scan` | `MapToStruct` / conversores |
| `blinq/provider/memory` | execução em slice (+ `NewMulti`) |
| `blinq/provider/sql` | compile + execute via `DB` (Gateway) |
| `blinq/provider/sql/dialect` | Postgres / SQLite / MySQL / MSSQL |
| `blinq/provider/sql/postgres` | `New(db)` helper fino |
| `blinq/provider/mongo` | `CompileFind` / `CompileLookup` → map (zero deps) |
| `blinq/provider/mongo/driver` | submodule: Execute via mongo-driver (`CollectionAPI`) |
| `blinq/provider/elastic` | `CompileSearch` → Query DSL (zero deps) |
| `blinq/provider/elastic/client` | submodule: Search API (`go-elasticsearch/v8`) |
| `blinq/provider/graphql` | `CompileQuery` + Provider HTTP (`Poster`) |
| `blinq/provider/api` | wire JSON + Provider HTTP |
| `blinq/migrate` | Runner Up/Down/Status + `FSSource` |

## Codegen

Você escreve o model **como sqlc** (`type Product struct { ... }`). O codegen emite o metamodelo de query (`ProductModel` com `Field[T]`):

```bash
nitrogen make models.go --data
# Pacote model/ → DSL em ../modelgen/
# Pacote flat (ex. productmodel/) → productmodel/modelgen/
# Legado: --same-package → *_model_gen.go no mesmo pacote (inclui TableName)
```

| Arquivo / pacote | Papel |
|------------------|--------|
| `model.Product` | row / JSON / scan (igual sqlc) |
| `modelgen.ProductModel` / **`modelgen.Products`** | colunas tipadas para DSL |

Alias curto (codegen): tabela `products` → `Products` (= `ProductModel`).

```go
modelgen.Products.Active.Eq(true)
modelgen.Products.Price.Gte(100)
// bool: field.IsTrue(modelgen.Products.Active)
```

Não dá para ter só o struct de dados na DSL: `float64` não tem `.Gte()` — por isso o metamodelo gerado.

Pacote: `github.com/rodrigocelsius/go-nitro/codegen/blinq/v2`.

Tags úteis na struct:

- `db:"col"` — nome da coluna  
- `blinq:"index"` — `CREATE INDEX` na migration **e** Filterable/Sortable no Resource quando as listas são nil.  
  Use só em colunas que você **filtra ou ordena** (name, email, sku, status, FKs…). **Não** coloque em todo campo (description, notes, password, cost…) — índice em excesso pesa em escrita e incha o schema.  
  Scaffold: nomes de filtro comuns ganham `blinq:"index"`; demais precisam de `campo:tipo:index` (ou `:noindex` para forçar sem índice).
- `blinq:"unique"` ou `unique:"true"` — UNIQUE + índice único  
- `blinq:"pk"` ou `primary:"true"` — primary key (campo `id` já é PK por padrão)  
- `blinq:"soft"` / campo `DeletedAt` — preenche `Entity.SoftColumn`  
- `blinq:"tenant"` / `tenant_id` — preenche `Entity.TenantColumn`  
- `//nitro:table=nome` — override da tabela  

```go
type Product struct {
    ID     int64  `json:"id" db:"id"`                         // PK → índice automático
    Name   string `json:"name" db:"name" blinq:"index"`       // CREATE INDEX
    Email  string `json:"email" db:"email" blinq:"unique"`    // UNIQUE INDEX
    Active bool   `json:"active" db:"active" blinq:"index"`
}
```

## Exemplo HTTP (Postgres)

[`examples/v1/blinq-hello`](../../examples/v1/blinq-hello/) — **hello world**: model + schema + Store + Resource.

[`examples/v1/db-blinq`](../../examples/v1/db-blinq/) — CRUD via **Resource** + store direto (filtros / sort; `blinq:"index"` → Filterable/Sortable).

[`examples/v1/db-blinq-handlers`](../../examples/v1/db-blinq-handlers/) — mesmas rotas com handlers + DSL (`WhereIf`).

Filtros/sort no Resource: `Filterable` / `FilterRules` / `Sortable`, **ou** omitir Filterable/Sortable para inferir de `blinq:"index"` (+ `id`/`created_at`/`updated_at` em Sortable). Query custom:

Anti-DoS (defaults): `MaxPerPage=100`, `MaxPage=10000`, `MaxFilters=16`, `MaxFilterValueLen=200`. Ajuste via `ResourceConfig`.

```go
// *nitro.Ctx já é context.Context — passe c direto
store.Find(c, store.Query().
    WhereIf(active, ProductModel.Active.Eq(true)).
    WhereIf(q != "", ProductModel.Name.IContains(q)).
    Limit(50))
```

`WhereIf(cond, pred)` = atalho para `if cond { q = q.Where(pred) }`.

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
cd examples/v1/blinq-hello && go run .

curl -X POST http://localhost:8080/hellos -d '{"message":"world"}'
curl http://localhost:8080/hellos
```

Demo sem banco: `go run -tags memory .`

## Migração desde `data/runtime`

O pacote `data/runtime` (LINQ antigo) foi **removido** na Fase 2 do BLinq. Mapeamento rápido:

| Antes (`data/runtime`) | Agora (BLinq) |
|------------------------|---------------|
| `runtime.From[T]("users")` | `query.FromName("users")` |
| `runtime.ColumnRef` | `field.Field[T]` / `field.Col[T](...)` |
| `*runtime.Query[T]` | `query.Query` (valor) |
| `runtime.PageResult` | `query.PageResult` / `PaginateCollect` |
| `runtime.MapToStruct` | `blinq/scan.MapToStruct` |
| `pgx.CompileSelect` | `blinq/provider/sql.Compile` + dialeto |
| `InsertAST` / `UpdateAST` | `query.InsertInto` / `Update` / `DeleteFrom` |

`pgx.Store[T]` e `repository.BaseRepository[T]` já usam BLinq internamente — apps que usavam só o Store mudam sobretudo os metamodelos (`ColumnRef` → `Field[T]`).

## Features (P0/P1)

- Predicados And/Or/Not, WhereIf, OrderBy, Limit/Offset, Distinct  
- Soft delete / Tenant hints  
- Count / Sum / Avg / Min / Max + Paginate  
- Join / LeftJoin, GroupBy / Having  
- Upsert (ON CONFLICT / ON DUPLICATE / MSSQL MERGE), Bulk `Rows` + `Chunk`, `Inc`, `OmitReturning`, Store `CreateMany` / `UpdateWhere`  
- Dialetos Postgres, SQLite, MySQL, MSSQL  
- QuoteIdent nos identificadores SQL; `SuggestIndexes`; `sql.WithCache`  
- DX: `Any`/`All`, keyset `After`/`Before`, `RawWhere`, `CollectTyped`, `EqOuter` / JSON  
- Elastic / GraphQL / migrate MVP; Mongo driver em submodule  

## Concurrent, Cache e Hooks (P2)

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

// filter paralelo (só memory / ConcurrentCapable)
_ = query.FromName("users").Collect(ctx, inner, &out, provider.WithConcurrency(8))
```

SQL providers ignoram concurrency; o hint `Cache` só surte efeito com `provider.Wrap` + `Cache` (ou `sql.New(..., sql.WithCache(c))` / `postgres.New(db, sql.WithCache(c))`).

## Include, Subquery, Mongo/API/Elastic/GraphQL (P2)

```go
// 1:N
rel := field.HasMany("orders", OrdersEntity, user.ID.Ref(), order.UserID.Ref())
// N:1
rel = field.BelongsTo("user", UsersEntity, order.UserID.Ref(), user.ID.Ref())
// CLI: nitrogen make:relation Order belongs_to User
rows, err := query.From(UsersEntity).Include(rel).CollectMaps(ctx, memory.NewMulti(tables))

// EXISTS / IN (SELECT …)
q := query.From(UsersEntity).Where(
    query.Exists(query.From(OrdersEntity).Where(order.Total.Gte(100))),
)
q = query.From(UsersEntity).Where(query.InQuery(user.ID, subSelectUserIDs))

// Mongo filter (sem driver) / driver oficial em submodule
find, _ := mongo.CompileFind(q.AST())
// p := mongodriver.New(db) // blinq/provider/mongo/driver

// Elastic Query DSL / GraphQL / API wire
es, _ := elastic.CompileSearch(q.AST())
gql, _ := graphql.CompileQuery(q.AST())
body, _ := api.Encode(q.AST())
```

### Migrações SQL

A partir do model (estilo goose):

```bash
nitrogen migrate:make models.go --name create_hellos
# → migrations/YYYYMMDDHHMMSS_create_hellos.sql
```

Tags úteis no model: `blinq:"index"`, `unique:"true"`, `//nitro:notimestamps`, `//nitro:nosoftdelete`.

Na app:

```go
_ = db.MigrateUp(ctx) // aplica ./migrations
```

Embed (binário único): `db.MigrateFS(ctx, migrationsFS)` — se o FS tiver pasta `migrations/`, resolve sozinho.

Ou runner explícito:

```go
pool, _ := pgx.New(ctx, dsn)
r := &migrate.Runner{
    DB:     pgx.AdaptMigrate(pool),
    Source: migrate.GooseFSSource{FS: os.DirFS("migrations")},
}
_ = r.Up(ctx)
```

Geração: `codegen/migration`. Runner: `blinq/migrate`. Demo: [`examples/v1/blinq-hello`](../../examples/v1/blinq-hello/).

Plano completo: [`plan_blinq.md`](../../plan_blinq.md).

## See also

- [Data layer guide](data-layer.md) — Store, Gateway, Repository, Pool  
- [`examples/v1/blinq-hello`](../../examples/v1/blinq-hello/) — hello world (Resource)  
- [`examples/v1/db-blinq`](../../examples/v1/db-blinq/) — CRUD + Resource  
- [`examples/v1/blinq-extras`](../../examples/v1/blinq-extras/) — MERGE / Mongo / Elastic / GraphQL / migrate  
- [`examples/v1/db-linq`](../../examples/v1/db-linq/) — mesmo CRUD (nome legado; já é BLinq)  
- [`blinq/README.md`](../../blinq/README.md)
