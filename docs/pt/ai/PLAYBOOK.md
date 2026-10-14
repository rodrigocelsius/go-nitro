# Nitro v1 — Playbook para IAs

**Idioma:** [Português](PLAYBOOK.md) · [English](../../ai/PLAYBOOK.en.md)

Documento operacional: ler e **implementar** um sistema completo nos padrões do framework.  
Entrada curta: [`AGENTS.md`](../../../AGENTS.md) · [`CLAUDE.md`](../../../CLAUDE.md) · [`AI.md`](../../../AI.md) · CLI [`docs/pt/guide/cli.md`](../guide/cli.md).

**Público:** agentes de código (Cursor, Claude, Codex, etc.).  
**Linguagem do código:** Go 1.25+ para apps SQL (`data/adapter/sql` / pgx).  
**Import canônico:** `nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"`.  
**Padrão de dados (OBRIGATÓRIO):** **BLinq** via `sqladapter.OpenEnv` / `platform.Open` (`make … --data` → `modelgen`, `StoreOf`, `WhereIf`).  
GORM / sqlx / sqlc / SQL cru **só** se o usuário pedir explicitamente para **não** usar BLinq — e ainda via Gateway.  
Exemplos: `examples/v1/cleanarch-blinq`, `examples/v1/db-blinq`. Guia: [`docs/pt/guide/blinq.md`](../guide/blinq.md).

---

## 0. Missão e ordem de trabalho

Ao receber um pedido de sistema (API, CRUD, auth, etc.):

1. Classificar o domínio (recursos, regras, auth, DB).
2. **Gerar com a CLI** (`nitrogen new` → `install` → `make:scaffold` / `make:*`) — não inventar árvore.
3. Escolher a **receita** (§1) só para o que a CLI não cobre (Resource vs handlers, auth…).
4. Completar wiring (`cmd/server`, `handler/routes.go`), **BLinq** (`make … --data`) + migrate se SQL.
5. Validar checklist (§11) + `nitrogen doctor`.
6. Não inventar APIs fora deste playbook; copiar dos exemplos listados. **BLinq é o default** — avisar / recusar outras stacks se o pedido for genérico (“use um ORM”).

---

## 1. Receitas (quando usar o quê)

### R1 — API sem banco (protótipo)

- `nitrogen new app --basic` **ou** CA padrão + `make:scaffold` com repository memory.
- `nitro.New()` + rotas **ou** Resource + `MemoryRepository`.
- Exemplo: `examples/v1/basic`, `examples/v1/cleanarch`, `examples/v1/crud`.

### R2 — CRUD Postgres (preferido)

1. `nitrogen new app` + `nitrogen install postgresql` (ou `mysql` / `sqlite`).
2. CRUD:
   - **Tabela simples:** `nitrogen make:scaffold Product name:string price:float64 active:bool --resource` → `app.Resource` + filtros (`?q=` / `?name=` / `sort=`).
   - **Domínio / usecases:** omita `--resource` (handlers CA + repository).
3. Opcional: `--with-seed` / `--with-factory`.
4. `nitrogen up` ou `go run ./cmd/server`.
5. Exemplo base: `examples/v1/blinq-hello` → `db-blinq`; CA + BLinq: `examples/v1/cleanarch-blinq`; CA memória: `examples/v1/cleanarch`.

Filtros: `blinq:"index"` no model → Filterable + Sortable no Resource; List CA usa `WhereIf` + `?sort=`.
Query params aceitam **snake e camel** por default (`AcceptCamelQuery`; ex. `?artistId=` ≡ `?artist_id=`).

### R2b — Resource seguro + DX (obrigatório com auth)

```go
auth := nitro.NewAuth(nitro.AuthConfig{JWTSecret: secret}).WithMemoryStore()
app.AuthRoutes("/auth", auth)
app.SetDefaultResourceMiddleware(nitro.JWTMiddleware(auth.Provider()))

app.Resource("/api/catalog/albums", &model.Album{}, stores.Albums, &nitro.ResourceConfig{
    NoCache: true,
    FilterRules: []nitro.FilterRule{
        {Param: "artist_id", Field: "artist_id", Op: "eq"}, // também aceita ?artistId=
        {Param: "q", Field: "name", Op: "icontains"},
    },
    BeforeWrite: func(ctx context.Context, entity any) error {
        a := entity.(*model.Album)
        return nitro.FillStringFrom(ctx, stores.Artists, a.ArtistID, &a.ArtistName,
            func(ar *model.Artist) string { return ar.Name })
    },
    Commands: nitro.Commands(
        nitro.Cmd("enrich", func(c *nitro.Ctx, album *model.Album) error {
            return c.OK(nitro.Map{"ok": true, "id": album.ID})
        }),
    ),
})
```

Regras: `SetDefaultResourceMiddleware(JWT)`; `Public:true` só se intencional; `computed`+`BeforeWrite`; ações via `Commands`; agregações via `CountAll`/`CountBuckets`. Guias: `resource-security.md`, `resource-hooks.md`, `business-resource.md`.

### R2c — Resource multi-tenant (shared-schema)

```go
app.Use(nitro.TenantMiddleware(nitro.TenantConfig{Require: true}))
app.Resource(&model.Order{}, store, &nitro.ResourceConfig{
    TenantScope: nitro.TenantFromCtx(),
})
```

- Embed `nitro.TenantModel`; `User.TenantID` no login (claim JWT `tenant_id`) ou header `X-Tenant-ID`.
- List filtra + Get/Update/Delete checam + Create carimba — **não** use só FilterRules (IDOR).
- Exemplo: `examples/v1/resource-tenant-scoped`.

### R3 — Queries custom (filtros dinâmicos)

- Handlers + `store.Query().WhereIf(...).OrderBy(...).Limit(...)`.
- Exemplo: `examples/v1/db-blinq-handlers`.

### R4 — Auth

- Preferir CLI: `nitrogen install auth-jwt` ou `nitrogen install auth-keycloak` (env + stub `internal/auth` + rota protegida de exemplo).
- Local: `NewAuth` + `AuthRoutes` + `JWTMiddleware` / `RequireRole` (`examples/v1/auth`).
- Keycloak/OIDC: `nitro.NewKeycloak` / `nitro.OIDCProvider` + `JWTMiddleware` (`examples/v1/auth-keycloak`).
- Cache L2: `nitro.RedisCache(rdb)` (`adapters/cache`).
- OpenAPI: `nitrogen openapi --bearer` (estático) ou `app.OpenAPI` / `app.Swagger` em runtime.
- Troca de engine para testes locais: `nitrogen install sqlite` e depois `nitrogen doctor --db`.

### R5 — Outra stack SQL (GORM / sqlx / sqlc)

- **Só se o requisito pedir.** O padrão continua sendo BLinq + Resource/Store.
- Ainda assim: `pgxadapter.Open` → `db.SQL()` (GORM/sqlx) ou `db.DBTX()` (sqlc).
- Exemplos: `examples/v1/db-gorm`, `db-sqlx`, `db-sqlc`.

### R6 — Clean Architecture

- Layout: `cmd/server` + `internal/{config,model,usecase,handler,repository}` (+ `migrations`).
- Usecase por operação: `usecase/product/{create,get,list,update,delete}.go`.
- Service + rota manual para regra além do CRUD (ex.: discount).
- **Não** criar `internal/{seed,factory,middleware,job}` no scaffold inicial — só via `nitrogen make:seeder|factory|middleware|job` quando o requisito pedir. Ao gravar conteúdo, a CLI remove `.gitkeep` se existir.
- Exemplo: `examples/v1/cleanarch` (memória) ou `examples/v1/cleanarch-blinq` (Postgres + BLinq). CLI: `nitrogen new` / `make:scaffold`.

---

## 2. Layout mental do monorepo

```
pkg/                 → facade (use sempre)
data/adapter/pgx/    → Open / OpenEnv, StoreOf, AdaptStore (opcional), MigrateUp, SQL, DBTX
data/gateway/        → telemetria SQL (automática via Open)
blinq/               → DSL, migrate runner
codegen/             → blinq metamodel, migration SQL
cmd/nitrogen/        → CLI make:* / install / new / migrate
examples/v1/         → verdades canônicas (copie daqui)
domain/, usecases/   → internos do framework (app não precisa no happy path)
```

Apps de usuário **não** devem depender de `usecases/` diretamente; usam `pkg`.

---

## 3. HTTP v1 — contrato do Context

### Princípio

Tudo começa e termina no `c` (`*nitro.Ctx`).

### Entrada

| Método | Quando |
|--------|--------|
| `c.Param("id")` / `c.Param("id", "0")` | path (default opcional) |
| `c.Int64Param("id")` | path obrigatório int64 → `(int64, error)` |
| `c.Query("q")` / `c.Int("page", 1)` / `c.Bool("active")` | query |
| `c.Sort("created_at")` | `?sort=-price` → `(field, desc)` |
| `nitro.Parse[T](c)` | body + validate por Content-Type (JSON ou form; preferido) |
| `c.BodyParser(&v)` | idem; pipeline default → transform → validate |
| `c.BodyJSON(&v)` | só JSON + validate |
| `c.Bind(&v)` | decode sem validate (webhooks) |
| `c.URL` / `app.GetRouteURL` | reverse routing (rota `.Name(...)`) |
| `c.UserUUID()` / `c.UserInt64()` | claims tipados |

### Saída

| Método | Status |
|--------|--------|
| `c.OK(v)` | 200 |
| `c.Created(v)` | 201 |
| `c.NoContent()` | 204 |
| `c.Text("…")` / `c.JSON(status, v)` | texto / status explícito |
| `c.File` / `c.CSV` / `c.Attachment` / `c.Stream` / `c.Redirect` | especiais |

### Erros

```go
return nitro.NotFound("user not found")
return c.BadRequest("invalid id")
return err // se err == pgx.ErrNotFound / domain.ErrNotFound → 404
```

Erros não tratados → **500** com `{"error":"internal server error"}`. Em **development** (`Config.Env` / `NITRO_ENV=development`), o body também inclui `request_id` e uma dica (`NITRO_OTEL=0` para silenciar telemetria). Produção não vaza detalhes internos.

### Context

`*nitro.Ctx` implementa `context.Context`. Use `c` (ou `c.Context()` se a API exigir `context.Context` nomeado).

**Atenção:** strings retornadas por Param/Query vivem no pool do request — não guarde após o handler.

---

## 4. Resource (CRUD Convention over Configuration)

```go
app.Resource(&Product{}, store, &nitro.ResourceConfig{
    NoCache: true,
    // Filterable/Sortable: omitir para inferir de blinq:"index"
    FilterRules: []nitro.FilterRule{
        {Param: "min_price", Field: "price", Op: "gte"},
        {Param: "q", Field: "name", Op: "icontains"},
    },
})
// path explícito ainda válido:
app.Resource("/products", &Product{}, store, &nitro.ResourceConfig{NoCache: true})
// memória tipada:
nitro.Resource(app, "/products", repo, &nitro.ResourceConfig{NoCache: true})
```

### `app.Resource` (única API)

```go
app.Resource(&Product{}, store) // → /products (TableName / plural)
app.Resource("/users", &UserController{})
app.Resource("/users", &User{}, repo)
app.Resource("/users", &User{}, repo, &nitro.ResourceConfig{NoCache: true, Filterable: []string{"name"}})
```

Um único parâmetro de config: `*ResourceConfig`.
`*pgx.Store` implementa `AsResourceRepository` — **não** precisa `AdaptStore` no happy path.
Padrão de model: `nitro.Model` (`nitro.UUID` + `nitro.DateTime` + soft delete). BIGSERIAL: `ID int64` (AutoID já infere false; `AutoID: nitro.Ptr(false)` para forçar).
Use `nitro.Date` / `TimeOfDay` para dias/horários civis; **não** `string`+`VARCHAR` nem `time.Time` no Resource.
Ver guides: [uuid](../guide/uuid.md) · [temporal](../guide/temporal.md) · [json](../guide/json.md) · [money](../guide/money.md) · [units](../guide/units.md) · [locale](../guide/locale.md) · [convert](../guide/convert.md).

`nitrogen make … --data` gera a DSL BLinq em **`modelgen`**. O path do Resource é o plural do tipo; se `//nitro:table=` for diferente, adicione `func (T) TableName() string` no model.

`nitro.Resource(app, path, repo, opts...)` — variante genérica para `MemoryRepository[T]`.

Rotas geradas: `GET /`, `GET /:id`, `POST /`, `PATCH /:id`, `DELETE /:id`.

**Regra:** não escreva esses 5 handlers à mão se Resource + filtros bastam.

---

## 5. Data layer + Gateway

```
App → Store (BLinq) → Gateway → pgx | database/sql → PostgreSQL / MySQL / MariaDB / SQL Server / SQLite
```

```go
ctx := context.Background()
db, err := sqladapter.OpenEnv(ctx) // NITRO_DB_DRIVER + DATABASE_URL | DB_DSN
// Apps da CLI: platform.Open(ctx) → sqladapter.OpenEnv
defer db.Close()

if err := db.MigrateUp(ctx); err != nil { /* ./migrations */ }

store := sqladapter.StoreOf[Product](db, "products")
app.Resource(&Product{}, store, opts)
```

Testes (sem Docker):

```go
db, err := sqladapter.OpenTest(ctx)                    // SQLite em memória
db, err := sqladapter.OpenTestMigrate(ctx, "migrations")
```

| API | Uso |
|-----|-----|
| `sqladapter.OpenEnv` / `platform.Open` | **preferido** na app |
| `MigrateUp` | `./migrations` goose (**preferido**) |
| `MigrateDir` / `MigrateFS` | path custom / embed |
| `OpenTest` / `OpenTestMigrate` | testes sem Docker |
| `NITRO_DB_QUIET=1` | desliga logger SQL |

**Proibido:** abrir pool sem Gateway (`sqladapter` / `pgxadapter.Open`). `pgxadapter.Open` permanece válido em caminhos só-Postgres.
### Transações

```go
exec := pgxadapter.NewExecutor(db.Pool)
tx, err := exec.Begin(c)
if err != nil {
    return err
}
defer tx.Rollback(c)

// usar tx.Exec / Query no escopo atômico
if err := tx.Commit(c); err != nil {
    return err
}
```

Detalhe: [`docs/guide/data-layer.md`](../guide/data-layer.md) (seção Transactions).

---

## 6. BLinq — model vs metamodelo

| Artefato | Quem escreve | Papel |
|----------|--------------|--------|
| `type Product struct` (pacote `model`) | você | row / JSON / scan |
| `modelgen.Products` / `ProductModel` | codegen (`nitrogen make … --data`) | DSL tipada |
| `modelgen.ProductEntity` | codegen | nome da tabela |
| `TableName()` (opcional) | você, no model | path do Resource quando **não** for o plural do tipo |

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

Bulk: **insert** = `CreateMany` / `query.InsertInto().Rows().Chunk()`; **update** = `UpdateWhere` (ou `query.Update().Set().Where().OmitReturning()`). Não faça loop de `CreateOne`/`Update` por linha. `CreateMany` devolve `RETURNING *` — chunks modestos. Dumps de milhões: `COPY` no importer é aceitável; o app Go continua em BLinq + `modelgen`.

Stand-in compartilhado: `examples/v1/productmodel` + `productmodel/modelgen`.
Em app real: `nitrogen make internal/model/product.go --data` → `internal/modelgen/`.
Legado (mesmo pacote): `--same-package`.

### Tags do model (migration + runtime)

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

| Tag / diretiva | Efeito |
|----------------|--------|
| `db:"col"` | coluna |
| `validate:"…"` | validação Parse/Resource |
| `default:"…"` | default JSON se chave ausente |
| `transform:"…"` | sanitize/case **antes** do validate (`trim`, `titlecase=pt`, `digits`, `upper`, …) |
| `nitro.StringSlice` / `JSONSlice[T]` / `JSON[T]` | JSON por dialeto (PG JSONB / MySQL JSON / SQLite TEXT); Valuer/Scanner |
| `nitro.UUID` / `Date` / `TimeOfDay` / `DateTime` | tipos portáteis (não `string`/`time.Time` no model) |
| `nitro.Money` / `Measure` / `Decimal` | money (2 cols), unidades SI, decimal |
| `nitro.Latitude` / `Longitude` / `LatLng` | WGS84 float64 + 9 casas; SQL NUMERIC/DECIMAL(12,9) ou SQLite TEXT — [geo](../guide/geo.md) |
| `db:"col,jsonb"` / `blinq:"jsonb,gin"` | força JSON (+ índice GIN se `gin` no PG) |
| `blinq:"index"` | CREATE INDEX **e** Filterable+Sortable no Resource (se listas nil) |
| `blinq:"unique"` / `unique:"true"` | unique |
| `blinq:"pk"` / `primary:"true"` | PK (`id` já é) |
| `blinq:"soft"` / `DeletedAt` / `nitro.Model` | soft delete |
| `HardModel` / `//nitro:nosoftdelete` | DELETE físico |
| `blinq:"tenant"` | tenant |

Sem `notimestamps`/`nosoftdelete`, o gerador de migration **injeta** `id` (UUID), `created_at`, `updated_at`, `deleted_at`.
No runtime, embuta `nitro.Model` para o struct Go ter os mesmos campos.

### CLI migrations

```bash
nitrogen migrate:make models.go --name create_products
# → migrations/YYYYMMDDHHMMSS_create_products.sql
# app: db.MigrateUp(ctx)
```

---

## 7. Validação

```go
in, err := nitro.Parse[Product](c)
if err != nil {
    return err // 400 JSON | 422 fields
}
```

- Defaults: `default:"true"` ou `validate:"default=true"` — só se a chave JSON **não** veio no body.
- Transforms: `transform:"trim,titlecase=pt,digits,upper,emptyslice"` — pipeline **default → transform → validate**. Helpers: `nitro.Digits` / `IsDigits` / `SplitName` / `RandomDigits` / `IsEmail` / `NormalizeEmail`. Ver `examples/v1/transform`. **Não** escrever `toTitleCase` / `isDigits` / `randomDigits` no app.
- Config: `nitro.ConfigOf[T]()` / `MustConfig` / `LoadDotEnv` — nested + CSV `[]string` + aliases `env:"DB_DSN,DATABASE_URL"` + `Duration`/`url.URL` + `env:"X,overwrite"`. Guide `docs/guide/config.md`. **Não** reparsear `.env` nem mapear `flatEnv` na mão.
- Auth: `RequireRoles([]string{"admin"}, WithBypassRoles("super_admin"))`. Resource: `BeforeCreate`/`OnMutate`, `ExtraRoutes`, `Aliases`, `Only`/`Except`/`Replace`, PATCH merge on.
- BR validators: `nitro.RegisterBRValidators(v)` (opt-in). DB: `OpenFrom` + `Ready` (ou `OpenWithPool` + `EnsureSchema`). Health: `Ping` / `NamedHealth`. Body solto: `c.BindMap()`. Soft≠status: `FilterActive` / `SoftDeleteSQL`.
- Handlers padrão: `Me` / `AppInfo`; CSV: `c.CSV` / `c.CSVFrom` + `JSONCell`.
- Scaffold: `nitrogen new app [--basic|--microservice]`; `nitrogen install postgresql`; DTO: `nitrogen make:dto` (`nitro:"public"`). Exemplo: `examples/v1/resource-dx` / `cleanarch`.
- Custom: `nitro.RegisterBRValidators(v)` para documentos BR; `v.Register("sku", …)` só para regras da **app**. Ver `examples/v1/validation`.
- Resource valida Create/Update automaticamente.

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
admin.Resource("/items", &Item{}, repo) // JWT+admin em todas as rotas do Resource
```

**Não** inventar middleware global por path-prefix (`apiAuthGate`) — use `Group` + `WriteMiddleware` / `ReadMiddleware`.

---

## 9. Middleware e defaults

`nitro.New()` já liga: Recover, Logger, MaxBody, RequestID, security headers; CORS amplo em `EnvDevelopment`.

Opt-in comuns: `CORS`, `RateLimit`, `Compress`, `JWTMiddleware`, grupos `app.Group(prefix, mws...)`.

Listen: `nitro.Ok(app.Listen(":8080"))` ou `log.Fatal(app.Listen(...))` — graceful shutdown por default.

---

## 10. Template completo (colar e adaptar)

Estrutura alvo de uma API com um recurso `Note`:

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

### main.go (esqueleto)

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
        // Filterable/Sortable inferidos de blinq:"index"; FilterRules para ops extras:
        FilterRules: []nitro.FilterRule{
            {Param: "q", Field: "title", Op: "icontains"},
        },
    })

    // Rota custom (opcional):
    // app.Get("/notes/search", func(c *nitro.Ctx) error { ... WhereIf ... })

    nitro.Ok(app.Listen(":8080"))
}
```

### Comandos

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
nitrogen make models.go --data
nitrogen migrate:make models.go --name create_notes
go run .
```

### Quando adicionar handler manual

- Agregações, desconto, workflows, payloads que não são o model.
- Padrão: `nitro.Parse[Input](c)` → service → `c.OK` / erros.
- Ver `ApplyDiscountHandler` em `examples/v1/cleanarch`.

---

## 11. Checklist de entrega

- [ ] Sistema/resource gerado com a CLI (`new` / `install` / `make:scaffold` / `make:*`) quando aplicável
- [ ] APIs v1 apenas (`c.OK`, `Parse`, sem `data/runtime`)
- [ ] CRUD via Resource (`--resource`) **ou** CA (`usecase` + `handler`) — não reinventar os 5 endpoints sem motivo
- [ ] DB via `sqladapter.OpenEnv` / `platform.Open` (+ `MigrateUp` / `nitrogen up`) quando houver schema
- [ ] Metamodel gerado (`make … --data` → `modelgen`) ao usar BLinq
- [ ] `nitro.Model` (UUID + soft) ou `HardModel` / serial explícito
- [ ] Tags `validate` / `transform` nos campos de escrita
- [ ] Sem `log` manual de listen/shutdown no `main`
- [ ] Auth só se o requisito pedir (`install auth-jwt` / `auth-keycloak` ou examples)
- [ ] Opcional: `nitrogen openapi`, `nitrogen seed`, `doctor --db`
- [ ] `go build` / `go test` / `nitrogen doctor` OK

---

## 12. Anti-padrões (rejeitar em review)

| Errado | Certo |
|--------|-------|
| `return nitro.OK(x)` sucesso | `return c.OK(x)` |
| `c.BindJSON` / `QueryIntDefault` | `Parse` / `c.Int` |
| Pool pgx / `database/sql` direto | `sqladapter.OpenEnv` / `platform.Open` |
| `data/runtime` / LINQ antigo | BLinq `query` + `field` |
| `product.Price.Gte` no struct row | `Products.Price.Gte` no metamodelo |
| 5 handlers CRUD manuais | `app.Resource` |
| `AutoID: true` + BIGSERIAL | `ID int64` (AutoID desliga sozinho) ou `AutoID: nitro.Ptr(false)` |
| SoftColumn sem Soft() | SoftColumn na Entity já liga soft; `Hard()` para opt-out |
| Duplicar campos id/timestamps | `nitro.Model` / `HardModel` |
| Schema só em `Exec` sem versionar (apps reais) | `migrate:make` + `MigrateUp` |
| CPF/CNPJ no `registerBuiltins` / `NewValidator()` | plugin `usecases/validator/br` + `RegisterBRValidators` |
| `toTitleCase` / `digitsOnly` / `isDigits` / `splitName` / `randomDigits` no app | `transform:"titlecase=pt,digits"` e `nitro.Digits` / `IsDigits` / `SplitName` / `RandomDigits` / `IsEmail` |
| Parser `.env` caseiro | `nitro.ConfigOf` / `LoadDotEnv` |
| CSV + `Fasthttp().SetBody` manual | `c.CSV` / `c.CSVFrom` / `JSONCell` / `c.Attachment` |
| Health ping + timeout manual | `nitro.Ping("name", pingFn)` |
| `/api/me` / root info manuais | `nitro.Me()` / `MeRoute` / `AppInfo` / `Info` |
| EnsureSchema + MigrateUp separados | `OpenFrom` + `db.Ready` |
| Resource sem DELETE (ou List custom) | `Only` / `Except` / `Replace` |
| CPF/CNPJ/phone/CEP na app (copiar algoritmo) | `nitro.RegisterBRValidators(v)` |
| `search_path` / CREATE SCHEMA na mão | `OpenFrom` / `OpenWithPool` + `EnsureSchema` |
| Wrap repo só p/ cache invalidate | `ResourceConfig.OnMutate` |
| DTO público manual | `nitrogen make:dto` / `nitro:"public"` |
| `type X []T` + Valuer/Scanner JSONB caseiro | `nitro.JSONSlice[T]` / `StringSlice` |
| Middleware global path-prefix (`apiAuthGate`) | `Group` + `ResourceConfig.WriteMiddleware` |

---

## 13. Mapa rápido de arquivos

| Precisa de… | Abra… |
|-------------|--------|
| Hello DB | `examples/v1/blinq-hello/` |
| Resource + filtros | `examples/v1/db-blinq/` |
| WhereIf handlers | `examples/v1/db-blinq-handlers/` |
| Clean arch | `examples/v1/cleanarch/` (memória) · `examples/v1/cleanarch-blinq/` (BLinq) |
| Auth | `examples/v1/auth/` |
| Validation custom | `examples/v1/validation/` |
| Transforms | `examples/v1/transform/` |
| Sintaxe Ctx | `docs/guide/context.md` · `docs/guide/request.md` |
| BLinq guide | `docs/guide/blinq.md` |
| Data layer | `docs/guide/data-layer.md` |
| Open/Store/Migrate | `data/adapter/pgx/open_gateway.go`, `migrate_adapt.go` |
| CLI | `cmd/nitrogen/main.go` |

---

## 14. Estilo de código gerado pela IA

- Português só em comentários de exemplo/README se o repo estiver em PT; identificadores em inglês.
- Comentário de cabeçalho no `main` com `export DATABASE_URL`, `go run .` e curls.
- Diff mínimo: não refatore o framework ao implementar uma app.
- Preferir composição igual aos exemplos v1 em vez de abstrações novas.
- Um recurso por migration inicial; migrations seguintes para alterações.

---

**Fim do playbook.** Se houver conflito entre docs antigas (v1) e este arquivo, **este arquivo vence**.
