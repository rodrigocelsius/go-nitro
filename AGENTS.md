# AGENTS.md — Nitro Framework (instructions for AIs)

> **Read this first.** Then: [`CLAUDE.md`](CLAUDE.md) · [`AI.md`](AI.md) · playbook [`docs/ai/PLAYBOOK.en.md`](docs/ai/PLAYBOOK.en.md).

You are generating or changing code in **Nitro v2** (`github.com/rodrigocelsius/go-nitro/v2`). Follow these contracts. Do not invent pre-1.0 APIs or data stacks outside the Gateway.

**Toolchain:** apps that use `data/adapter/pgx` or `data/adapter/sql` need **Go 1.25+** (pgx v5.10).

## DEFAULT DATA ACCESS (STOP)

**If the user does not name another ORM/stack, you MUST use BLinq for every SQL system.**

`sqladapter.OpenEnv` / `platform.Open` + `StoreOf` + `modelgen` + `WhereIf`.

- **Default** to BLinq. GORM, sqlx, sqlc and raw SQL are supported alternatives (see `db-gorm` / `db-sqlx` / `db-sqlc` examples) — use them when the user names them or the project already uses them.
- A generic “use an ORM” / “repository” / “data layer” request means BLinq.
- Switch stacks when the user names another one (e.g. `use GORM`, `use sqlc`) or the existing project uses it; mention the BLinq default once.
- Copy from `db-blinq` / `blinq-hello` / `resource-dx` first — use `db-gorm` / `db-sqlx` / `db-sqlc` only when that stack is requested. Full CA (`cleanarch-blinq`) only when non-CRUD rules need usecases.

### BLinq shape — Resource first

1. **`app.Resource`** + BLinq `StoreOf` (filters via `ResourceConfig` / metamodel).
2. Routes only in **`internal/handler/routes.go`** (`Register`) — never CRUD in `main`.
3. **Minimal** `main` / handler / repository / usecase — omit CA CRUD layers for plain CRUD; use `make:scaffold … --resource`.

| Do (default) | Do not (unless user asks) |
|--------------|---------------------------|
| `sqladapter.OpenEnv` / `platform.Open` | raw `pgxpool` / `database/sql` without Gateway |
| `sqladapter.StoreFor[T](db)` (table by convention; `StoreOf[T](db, "table")` only off-convention) | GORM / sqlx / sqlc models |
| `app.Resource` + `routes.go` | five hand-rolled CRUD handlers / fat `main` |
| `modelgen.Products.Name.Eq(...)` / `WhereIf` / `OrderBy` / `CreateMany` / `UpdateWhere` | stringly SQL (`"WHERE name = $1"`) / loop `CreateOne` |
| `store.CountAll` / `CountWhere` / `SumInt64` / `FindOrdered` / `CountBy` / `CountBuckets` | `StdDB().Query` / raw `COUNT(*)` / N+1 counts |
| `field.InRange` / `field.EqValue` / `query.Take(n int)` / `query.MonthBuckets` | `Limit(uint64(n))` + manual `&ptr` for nullable FKs |
| `app.SetDefaultResourceMiddleware(JWT…)` then Resource | `Public: true` on every Resource “to silence warnings” |
| `BeforeWrite` + `computed:"true"` + `FillStringFrom` | client-writable denorm fields / duplicated BeforeCreate+BeforeUpdate |
| `Commands` / `nitro.Cmd("enrich", …)` for `POST /:id/action` | loose `app.Post("/resource/:id/enrich")` outside Resource |
| one FilterRule `artist_id` (AcceptCamelQuery) | duplicate FilterRules for `artistId` + `artist_id` |
| `nitrogen make model.go --data` → package `modelgen` | inventing a parallel query layer |

### Resource security + DX (STOP)

1. After `AuthRoutes`, call **`app.SetDefaultResourceMiddleware(nitro.JWTMiddleware(auth.Provider()))`** so every Resource inherits JWT unless `Public: true`.
2. **`Public: true` only** for intentionally open CRUD. In `EnvProduction`, unprotected Resources **panic** at register (V-04) unless `Config.Security.AllowUnauthenticatedResources`.
3. Denormalized names: tag **`computed:"true"`** (mass-assignment blocked like `readonly`) and fill in **`BeforeWrite`** via **`nitro.FillStringFrom`**.
4. Business actions (`enrich`, `publish`, …): **`ResourceConfig.Commands`**, not ad-hoc routes.
5. Dashboard/analytics: `CountAll` / `CountWhere` / `SumInt64` / `CountByTake` / `CountBuckets` — never raw SQL.
6. Filters: declare snake_case once; clients may send camelCase (`AcceptCamelQuery` default on).
7. **Multi-tenant (shared-schema):** embed **`nitro.TenantModel`**, resolve with **`app.Use(nitro.TenantMiddleware(…))`** (JWT claim; the header is ignored unless `AllowHeader`). A `tenant_id` column makes CRUD **auto-scoped** (opt out: `NoTenantScope`). Do not rely on FilterRules alone for IDOR. Example: [`examples/v1/resource-tenant-scoped`](examples/v1/resource-tenant-scoped/).

Canonical copy-paste: [`examples/v1/db-blinq`](examples/v1/db-blinq/) · [`examples/v1/blinq-hello`](examples/v1/blinq-hello/) · [`examples/v1/cleanarch-blinq`](examples/v1/cleanarch-blinq/) (CA) · guides [`docs/guide/blinq.md`](docs/guide/blinq.md) · [`docs/guide/resource-security.md`](docs/guide/resource-security.md) · [`docs/guide/resource-hooks.md`](docs/guide/resource-hooks.md).

## Creating a system (required — CLI first)

**Do not build the tree by hand.** Use the `nitrogen` CLI, then fill in domain rules and wiring.

**BLinq is the default data access** for Nitro SQL apps: typed metamodel (`modelgen.Products.Field…`), `StoreOf`, `Resource` / `WhereIf`. After scaffold, run `nitrogen make <model>.go --data`: DSL in package **`modelgen`**. Resource path is the plural of the type (optional `TableName()` on the model if the table differs). Legacy: `--same-package`.

```bash
# 1. Project (Clean Architecture default)
nitrogen new <app> --module github.com/org/<app>
# or: --basic | --microservice | --worker
# fleet (HTTP gateway + AMQP workers):
# nitrogen new <root> --fleet --services gateway,nfe,danfe --module github.com/org/<root> --nitro-path /path/to/nitro

# 2. Drivers (updates config + .env* + docker-compose)
nitrogen install postgresql   # and/or redis, rabbitmq, minio, mysql, mariadb, sqlserver

# 3. Full CRUD resource (model + usecase/* + handler + repository)
nitrogen make:scaffold Product name:string price:float64 active:bool

# 4. BLinq metamodel (default with Postgres)
nitrogen make internal/model/product.go --data

# 5. Extra artifacts **only if the requirement asks** (not created by `new`)
#    nitrogen make:usecase Product               → CRUD usecases
#    nitrogen make:usecase Product ApplyDiscount → standalone usecase
#    nitrogen make:middleware Auth   → creates internal/middleware/
#    nitrogen make:job SyncOrders    → creates internal/job/
#    nitrogen make:seeder Product    → creates internal/seed/
#    nitrogen make:factory Product   → creates internal/factory/
#    nitrogen make:test Product
#    nitrogen migrate:make internal/model/product.go

# 6. Validate
nitrogen doctor
go test ./...
```

| Command | Use |
|---------|-----|
| `new` | CA app (`cmd/server` + `internal/{config,model,usecase,handler,repository}` + migrations). No `seed`/`factory`/`middleware`/`job` |
| `install` | Adapter + env + **docker-compose** |
| `make:scaffold` | Full CRUD slice (normalized names: `products` → `Product` / table `products`) |
| `make <model.go> --data` | **BLinq**: `modelgen` DSL — default with DB |
| `make:model [-r]` | Empty model or CoC (`--resource`) |
| `make:handler` / `make:usecase` / `make:repository` | Isolated layers |
| `make:middleware` / `make:job` / `make:seeder` / `make:factory` / `make:test` | DX — **creates** `internal/{middleware,job,seed,factory}` only when invoked |
| `model:show` | Inspect tags/fields |
| `migrate` / `migrate:make` | Run / generate goose SQL (**there is no** `make:migration`) |
| `doctor` | Layout + tools |
| `ai:init` | Write consumer `AGENTS.md` / `CLAUDE.md` / `AI.md` / `.cursor/rules/nitro.mdc` / `.cursor/prompts/nitro-blinq.md` into an app (`--force` to refresh) |

Canonical layout: [`examples/v1/cleanarch`](examples/v1/cleanarch/) (memory) · [`examples/v1/cleanarch-blinq`](examples/v1/cleanarch-blinq/) (Postgres + BLinq). CLI guide: [`docs/guide/cli.md`](docs/guide/cli.md).

**Naming:** the CLI singularizes/pluralizes (PascalCase type, lowercase package, snake_case plural table). Do not invent `ProductsService` or `handlers/` folders.

## Required contract (non-negotiable)

1. **App import:** `nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"` (+ `sqladapter` / `blinq` when using a DB).
2. **HTTP responses:** `c.OK`, `c.Created`, `c.NoContent`, `c.Text`, `c.CSV` / `c.CSVFrom` / `c.Attachment`. Do not use `nitro.OK(...)` / `nitro.Created(...)` for success.
3. **Domain errors:** `return nitro.NotFound("...")` / `c.BadRequest(...)` / sentinels (`domain.ErrNotFound`, `pgx.ErrNotFound` → 404).
4. **Body + validate:** `nitro.Parse[T](c)` (preferred). Pipeline: default → **`transform:`** → validate (JSON and form).
5. **CRUD:** Default = **`app.Resource`** + **BLinq** (`StoreOf` + metamodel) + `handler/routes.go`; keep `main`/handler/repo/usecase minimal. Full CA (thin handler → usecase → repository wrapping `Store`) **only** when non-CRUD domain rules require it. `nitro.Model`; `Only` / `Except` / `Replace`; scaffold with `--resource` for plain CRUD.
6. **SQL open:** `nitrogen install postgresql|mysql|mariadb|sqlserver|sqlite` → `platform.Open` (`sqladapter.OpenEnv`) → Gateway → **`MigrateUp`**. No raw pool.
7. **Typed queries (DEFAULT — BLinq):** `modelgen.Products.Field.Eq(...)`, `WhereIf`, `OrderBy`, `Paginate`. Bulk: `store.CreateMany` / `query.InsertInto().Rows().Chunk()`; `store.UpdateWhere` / `query.Update().Set().Where().OmitReturning()`. Package `data/runtime` **does not exist**. GORM/sqlx/sqlc/raw SQL when the user explicitly requests another stack (still via Gateway). Million-row dumps may use Postgres `COPY` in importers.
8. **Ctx is `context.Context`:** pass `c` to `store.Find(c, ...)`.
9. **Param/Query:** valid only during the handler (pool).
10. **ID:** UUID v7 (`nitro.Model`). BIGSERIAL: `ID int64`.
11. **CLI:** see above. Tests: `nitrogen test` / `cover --min 60 ./internal/...`.
12. **Config:** `nitro.ConfigOf[T]` — no homemade `.env` parser. Guide: [`docs/guide/config.md`](docs/guide/config.md).
13. **Case/sanitize:** `transform:"trim,titlecase=pt,digits,…"`. Helpers: `nitro.Digits` / `IsDigits` / `SplitName` / `RandomDigits` / `IsEmail` / `NormalizeEmail` — do not reimplement in the app.
14. **JSONB:** `nitro.StringSlice` / `JSONSlice[T]` / `JSON[T]`.
15. **Auth:** `RequireRoles` + `WithBypassRoles`. Resource: hooks (`AfterCreate`, …) / `OnMutate` / `ExtraRoutes` / **Commands·Queries·Rules** (v2) / **`sqladapter.Related`** for header+items (see `docs/guide/business-resource.md`).
16. **BR validators:** `nitro.RegisterBRValidators(v)` — opt-in (`usecases/validator/br`, never builtins). Company rules: `v.Register` in the app.
17. **DB pool:** `OpenEnv` / `Ready` (or `OpenWithPool` + `EnsureSchema`). Health: `Ping` / `NamedHealth`.
18. **Loose body:** `c.Bind` / `c.BindMap()`. Structs: `Parse` / `BodyParser` / `BodyJSON`.
19. **Scaffold:** always via `nitrogen new` + `make:*` + `install`. Example: `examples/v1/cleanarch` (memory) or `examples/v1/cleanarch-blinq` (SQL + BLinq).
20. **Telemetry ON** when something exports (OTEL HTTP + DB Gateway; DB spans are children of HTTP; W3C `traceparent` joined). OTLP needs `import _ "github.com/rodrigocelsius/go-nitro/adapters/otel/v2/otlp"` (CLI apps have it) + `OTEL_EXPORTER_OTLP_ENDPOINT`; without an exporter tracing is a no-op (zero cost). Opt-out: `NITRO_OTEL=0` / `DisableTelemetry` (also DB) / `NITRO_DB_QUIET=1` (DB only). Flush on shutdown.
21. **Modularization:** `app.Mount`; routes `.Name` + `c.URL`; SPA `SinglePageApp`; `app.Hooks()`.
22. **Listen:** banner + `Listening on http://…` and shutdown logs are **automatic** — do not duplicate `log.Printf("listening…")` in `main`.
23. **Stack choice:** if a prompt names GORM/sqlx/sqlc/raw SQL, honor it and mention the BLinq default once; for generic asks, use BLinq.
24. **Model tags are optional:** `json`/`db` tags may be omitted — both default independently to snake_case of the Go field name (`CustomerName` → `customer_name`); explicit tags still win. Old style (`json:"x" db:"x"` on every field) keeps working with zero behavior change. Guide: [`docs/guide/models.md`](docs/guide/models.md).
25. **Transactions are ambient (ctx):** multi-table writes go in Resource write hooks (`BeforeWrite`/`Before*`/`After*`/`BeforeDelete` — atomic by convention with BLinq, a hook error rolls back) or `Commands`; outside a Resource use `db.InTx(ctx, fn)` / `nitro.InTx`. Every Store called with that `ctx` joins. Side effects (queue, e-mail) only in `nitro.AfterCommit` / `ResourceConfig.AfterCommit`. Never `BEGIN` by hand. Guide: [`docs/guide/transactions.md`](docs/guide/transactions.md).
26. **Queue:** `nitro.QueueFromEnv()` (RabbitMQ / NATS JetStream / Kafka from `RABBITMQ_URL` / `NATS_URL` / `KAFKA_BROKERS` — streams, topics and consumers created on first use; in-memory otherwise and while the broker is down; `nitrogen install nats|kafka`). **Cache:** in-process by default; `app.SetCache(nitro.RedisCache(…))` shares it across replicas (Redis down → reads hit the DB). **PgBouncer:** `?pgbouncer=true` + `DATABASE_DIRECT_URL` for migrations.

## v2 conventions — do NOT configure these (STOP)

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
| Logs | one format for HTTP + DB: `NITRO_LOG_FORMAT=text\|json`, `NITRO_LOG_LEVEL`; DB queries DEBUG, slow (`NITRO_DB_SLOW_MS`) WARN | custom access/query loggers to "quiet" or "reformat" logs |

Removed in 2.0.0 (do not use): `nitro.CORS/Compress/Timeout`, `nitro.OK/Created/NoContent` helpers, `MapAny`, `SerialID`, `WithCache`, `ResourceOpts` option structs — see the migration table in [`docs/guide/conventions.md`](docs/guide/conventions.md). Reference app generated 100% by the CLI: [`examples/v1/cli-shop`](examples/v1/cli-shop/); zero-config tour: [`examples/v1/conventions`](examples/v1/conventions/).

## Decision tree (correct examples)

| Goal | Recipe / example |
|------|------------------|
| SQL CRUD (default) | `make:scaffold … --resource` → `db-blinq` / `blinq-hello` |
| New system / CA (non-CRUD rules) | `make:scaffold` without `--resource` → `cleanarch-blinq` |
| Minimal HTTP | `examples/v1/basic` or `nitrogen new --basic` |
| Handlers + `WhereIf` | `examples/v1/db-blinq-handlers` |
| JWT / Keycloak auth | `examples/v1/auth` / `auth-keycloak` |
| Validation / transform | `examples/v1/validation` / `transform` |
| Resource DX | `examples/v1/resource-dx` |
| Business commands | `examples/v1/order-business` |
| AfterCreate hook | `examples/v1/resource-aftercreate` |
| Multi-tenant Resource | `examples/v1/resource-tenant-scoped` |
| Header + line items (sale + items, atomic) | `sqladapter.Related` → `examples/v1/pos-sales` |

## Minimal Postgres scaffold

```text
1. nitrogen new app --module github.com/acme/app --nitro-path /path/to/nitro
2. nitrogen install postgresql   # or mysql | mariadb | sqlserver | sqlite
3. nitrogen make:scaffold Product name:string price:float64 active:bool --resource
   # writes model, routes.go with app.Resource, platform.Open, BLinq modelgen, goose migration
4. go test ./... && nitrogen doctor && go run ./cmd/server
```

## Docs

| Topic | Path |
|-------|------|
| Claude / Cursor extras | [`CLAUDE.md`](CLAUDE.md) · [`AI.md`](AI.md) |
| AI playbook (English) | [`docs/ai/PLAYBOOK.en.md`](docs/ai/PLAYBOOK.en.md) |
| AI index | [`docs/ai/README.en.md`](docs/ai/README.en.md) |
| nitrogen CLI | [`docs/guide/cli.md`](docs/guide/cli.md) |
| Guides | [`docs/guide/README.md`](docs/guide/README.md) |
| BLinq / data / config | guides `blinq` · `data-layer` · `config` |

## When finishing a feature

- [ ] Generated via CLI when it is a new project/resource
- [ ] With SQL: **BLinq** + **`app.Resource`** + `handler/routes.go` (`make:scaffold … --resource` unless CA rules needed)
- [ ] Minimal `main` / handler / repository / usecase — no five hand-rolled CRUD handlers
- [ ] Metamodel (`make … --data`) when queries/filters need the DSL
- [ ] Builds and `go test` in the module
- [ ] v1 patterns (`Parse`, `c.OK`)
- [ ] DB via `sqladapter.OpenEnv` / `platform.Open` + Gateway
- [ ] Goose migrations in `./migrations` if the schema changed
- [ ] No manual listen/shutdown `log`
- [ ] BLinq unless the user named another stack
