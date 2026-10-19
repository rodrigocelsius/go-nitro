# Changelog

All notable changes to Nitro will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [2.0.8] - 2026-10-19

### Fixed
- **Lowercase request headers were invisible** (`c.Header("Authorization")` returned "" for `authorization: Bearer …`): the server disabled header-name normalization, so lookups became case-sensitive. HTTP field names are case-insensitive (RFC 9110 §5.1) and Node `fetch`, browsers over HTTP/2 and most proxies send lowercase names — every JWT request coming through a BFF/proxy was rejected with 401. Normalization is back on; regression test runs a real server with raw lowercase/uppercase headers.

## [2.0.7] - 2026-10-19

### Added
- `ResourceConfig.DefaultSort` (e.g. `"-created_at"`): ordering applied when the request has no `?sort`, so audit logs and feeds are newest-first without every client remembering it. An explicit `?sort` still wins.

## [2.0.6] - 2026-10-19

### Added
- **Resource hooks can reach the request:** `BeforeWrite`/`BeforeCreate`/`BeforeUpdate`/`AfterCreate`/`AfterUpdate`/`AfterDelete`/`OnMutate` only received a bare `context.Context`, so they could not read the tenant, the authenticated user, the IP or headers (needed for audit trails, per-tenant side effects, outbox events). Resource handlers now attach the request; read it with `nitro.RequestFrom(ctx)` (or `domain.RequestFrom`).

## [2.0.5] - 2026-10-19

### Added
- **`Store.CountBucketsWhere(ctx, col, buckets, preds...)`**: time-series counts in a **single query** (`COUNT(CASE WHEN …)` per bucket, scanning only the overall window), scoped by any predicates — tenant, status, soft delete. `CountBuckets` ran one query per bucket and could not be filtered, so it was unusable for multi-tenant dashboards. Works with numbered (`$n`) and positional (`?`) placeholders; covered by Postgres (integration) and SQLite tests.
- `query.DayBuckets(n, now)` and `query.HourBuckets(n, now)` next to `MonthBuckets`.

## [2.0.4] - 2026-10-19

### Fixed
- **Resource PATCH lost updates:** the merge path wrote every column back, so a value changed concurrently (another request, a worker, another service sharing the table) between the read and the write was silently reverted — including `readonly` columns. PATCH now writes only the fields the request changed (+ `updated_at`), and nothing when nothing changed. New optional `domain.ResourcePartialUpdater`, implemented by the pgx/sql stores and forwarded by the Resource cache. PUT/replace mode is unchanged. Covered by a Postgres integration test (`-tags integration`, `NITRO_PG_DSN`).
- **`nitrogen make:middleware` generated code that did not compile** (`return c.Next()` — `nitro.Ctx` has no `Next`). The template now returns `nil` and documents the chain contract.

### Added
- `nitro.Slugify` ("Café & Cia. São Paulo" → `cafe-cia-sao-paulo`) and `nitro.FoldAccents`, plus `transform:"slug"` / `transform:"unaccent"`.

## [2.0.3] - 2026-10-19

### Fixed
- **BLinq/data codegen ignored `func (T) TableName() string`** and emitted the conventional table (e.g. `organizations` for a model whose runtime table is `app_organizations`), so typed queries hit the wrong table. A literal `TableName()` is now honored; `//nitro:table=` still wins.
- **`nitrogen version` always printed 2.0.0**: it now reports the module version stamped by `go install …@vX.Y.Z` (local builds report `-dev`).

## [2.0.2] - 2026-10-19

### Fixed
- **`go install github.com/rodrigocelsius/go-nitro/cmd/nitrogen/v2@…` failed** ("go.mod … must not contain replace directives"): the CLI module now has no `replace`; local development uses `cmd/nitrogen/go.work`.
- **`scripts/release.sh` rejected every version except v2.0.0**: the internal-require check grepped test fixtures recursively. It now checks each module's own `go.mod` and tags the CLI after the libraries it depends on.

## [2.0.1] - 2026-10-19

### Fixed
- **`nitrogen make <model>.go --data` generated code that did not compile** when a column used a Nitro JSONB wrapper (`nitro.Attrs`, `nitro.StringSlice`, `nitro.JSON[T]`): the `nitro` package was referenced without an import. JSONB columns now map to plain Go types in the DSL (`map[string]any`, `[]string`, `any`) and any other qualified column type gets its import emitted.
- **Columns typed with a model-package type** (e.g. `map[string]Address`) produced undefined identifiers in the `modelgen` package; they now fall back to `Field[any]`.
- **Embedded base structs declared in a sibling file were ignored** by the BLinq/data codegen: their fields (e.g. a shared `InstanceID`) were missing from the metamodel. Embeds are now resolved across the package, recursively.
- **`nitro.HardModel` / `nitro.TenantModel` embeds were not recognized** by codegen: a `HardModel` struct got a phantom `deleted_at` soft-delete column, and `TenantModel` did not expose `tenant_id`.
- **Abstract base structs became tables:** a struct only used as an embedded base (without `//nitro:table=…`) no longer generates a metamodel/table.
- **`nitrogen new` / `install` wrote absolute `replace` paths** (`=> /home/user/...`) into `go.mod`, breaking Docker builds, CI and other machines. Local Nitro replaces are now relative to the module (`=> ../../go-nitro/pkg`).

## [Unreleased]

> Versioning was reset: the first published release of this repository is **v2.0.0** (module path `github.com/rodrigocelsius/go-nitro/.../v2`). Entries below the 2.0.0 section describe the code's earlier internal history.

### Security
- **Resource mass assignment of system fields:** `ID`, `CreatedAt`, `UpdatedAt` and `DeletedAt` are now server-owned on create/update (a client could choose the primary key — overwriting another tenant's row with `MemoryRepo` — back-date timestamps, or create soft-deleted rows). `readonly`/`computed` tags inside embedded structs are honored.
- `MemoryRepository.Create` returns 409 on a duplicate ID instead of overwriting the row. New `SafeMap.SetIfAbsent`.

### Fixed
- **SQLite apps failed on the second boot** (`MigrateUp`: `Scan error … "applied_at"`): SQLite now stores `applied_at` as RFC 3339 text, and `applied_at` is read tolerantly (also fixes databases written by the old runner and MySQL without `parseTime`).
- **`go run ./cmd/server` could not open the DB** (`missing DSN`) although `.env` had it: generated `config.Load` calls `nitro.LoadDotEnv()` first; `nitrogen install` upgrades older `config.go`; the `missing DSN` error points at an unloaded `.env`.
- **CLI: a second `make:scaffold` broke the build** — the usecase import was not added to `routes.go`, `Register<Type>` was not wired in `main`, and the generated test called `Register`. Imports are now added with `go/parser`; mixed CA and `--resource` scaffolds compile.
- **CLI: migrations generated in the same second shared a version** (duplicate key in `MigrateUp`); new `migration.NextMigrationVersion`.
- **CLI: `nitrogen routes` / `nitrogen openapi` found nothing without `--dir`** (the default root `.` was skipped as a hidden directory).

### Changed
- **405 Method Not Allowed:** a request whose path is registered only for other methods now gets `405` with an `Allow` header instead of `404` (e.g. `POST` on a Resource with `Only: List, Get`). Unknown paths still return 404. New `domain.ErrMethodNotAllowed`.
- **One log format for HTTP and DB:** `NITRO_LOG_FORMAT=text|json` and `NITRO_LOG_LEVEL` drive both the access log and the DB gateway. The text access log is tagged by status (`[INFO]`, `[WARN]` 4xx, `[ERROR]` 5xx — was always `[DEBUG]`); JSON access lines carry `level` and `msg` like slog. DB gateway text lines share the access-log layout.
- **Per-query DB logs moved to DEBUG**; queries at or above `NITRO_DB_SLOW_MS` (default 200 ms, `gateway.WithSlowQuery`) log at WARN as `db slow query`.

### Security (auth & HTTP hardening)
- **JWT TTL defaults:** `NewAuth` without `AccessTokenTTL`/`RefreshTokenTTL` minted tokens that expired at issuance (while answering `expires_in: 900`); zero TTLs now use the documented 15 min / 7 days, and `expires_in` reflects the real TTL.
- **AuthRoutes:** e-mails are case-insensitive (`Ana@x.com` ≡ `ana@x.com`, no duplicate accounts); `MemoryUserStore.Create` claims the e-mail atomically; passwords over 72 bytes (bcrypt limit) are rejected with 422; store/hash failures are a generic 500 instead of "409 EMAIL_EXISTS" with the internal error text; per-IP rate limit (`DefaultAuthRateLimit` = 20/min, `Auth.WithRateLimit`); new `POST /auth/logout` revokes the refresh and bearer tokens.
- **Content-Disposition** (`c.File`/`c.Attachment`/`c.CSV`): control characters, quotes and backslashes are dropped, `/` neutralized, and non-ASCII names get an RFC 6266 `filename*` (UTF-8).
- `golang.org/x/net` → v0.60.0 in every module (GO-2026-6617, GO-2026-6612 reachable from `net/http` users).

### Performance
- **Telemetry is a no-op when nothing exports** (no OTLP endpoint / `NITRO_OTEL_STDOUT`): the SDK and HTTP tracing middleware are skipped (~3.4× cheaper hello-world request: 5.6 µs/70 allocs → 1.7 µs/9 allocs).
- **Hello world binary 28.0 MB → 13.5 MB** (9.2 MB with `-s -w`): the OTLP protobuf exporters (~13 MB each) moved to `adapters/otel/v2/otlp`, imported by CLI-generated apps; a startup warning fires if `OTEL_EXPORTER_OTLP_ENDPOINT` is set without it. Generated Makefile/Dockerfile and `nitrogen build` use `-trimpath -ldflags="-s -w"` (`nitrogen build` also targets `./cmd/server` in the CA layout; `--debug` keeps symbols).
- Resource request path: cached reflect field lookups (`domain.FieldByName`), cached query-param / wire-name variants, allocation-free timestamp defaults, single-allocation `UUID`/`DateTime` JSON, shape-based `DateTime` parsing (SQLite scans no longer allocate a parse error per column). In-process: Memory list −49% (493 → 120 allocs), create −32%, SQLite list −17%, SQLite filtered list −32%.
- **BLinq `PaginateCollect` skips `COUNT`** when the page is short (the total is exact); full or past-the-end pages still count.

### Fixed (telemetry)
- **OTLP export never worked with the current SDK:** `resource.Merge` failed with "conflicting Schema URL" (SDK semconv vs v1.24.0), silently disabling telemetry. The service name is now merged schemaless.
- Incoming W3C `traceparent`/`tracestate`/baggage are extracted, so HTTP spans join the caller's trace (propagator installed with the SDK).

### Added
- **`sqladapter.StoreFor[T](db)`**: `StoreOf` with the table name from the shared convention (`TableName()` or snake_case plural).
- **`nitrogen openapi` emits schemas** for Resource routes from the model struct: `<T>` (server-owned fields `readOnly`), `<T>Input`, `<T>Patch`, `<T>List`, an `Error` envelope on 4xx/5xx, exact success codes (201 create, 204 delete) and list query params. `codegen/openapi`: `AddSchema`, `UseErrorSchema`, `RefSchema`, `RouteInfo.ResponseSchema` / `SuccessStatus`.
- **`nitrogen doctor` compiles the project** (`go build` + `go vet`) and fails when it does not build; `--no-build` skips it.
- **CI `examples` job** (every example: `go mod tidy -diff`, vet, tests) and `make test-examples`; all example `go.mod` files tidied (`blinq-extras` was missing replaces).

## [2.0.0] - Unreleased

### Breaking / migration
- **Module path:** the repository is now `github.com/rodrigocelsius/go-nitro`; every module carries the `/v2` suffix (`github.com/rodrigocelsius/go-nitro/pkg/v2`, `.../adapters/fasthttp/v2`, …). Update imports. Nested modules are tagged `<dir>/v2.0.0` (see `RELEASING.md`, `scripts/release.sh`).
- **WebSocket:** same-origin by default; set `WSConfig.AllowedOrigins` / `CheckOrigin` for cross-origin clients. Messages are capped at `WSConfig.MaxMessageSize` (default 1 MiB).
- **Tenant header:** `X-Tenant-ID` is ignored unless `TenantConfig.AllowHeader` is explicitly true.
- **`SetupPprof(app, mws...)` and `Dashboard(prefix, mws...)`** require auth middleware in production (they panic otherwise).
- **IP allow-list:** an empty list now denies everyone (was: allow everyone).
- **`ClientIP()`** uses the rightmost untrusted `X-Forwarded-For` entry and only honors forwarding headers from trusted proxies (`SetTrustedProxies`).
- **OIDC/Keycloak:** issuer and audience are required (opt out with `AllowAnyAudience`), `exp` is mandatory, only RS256/384/512 accepted.
- **Auth usecases:** login/OAuth reject inactive users; `RefreshToken` re-validates the user and rotates the pair.

### Security
- Per-key bounded sliding-window rate limiter (fail-closed on overflow); no unbounded key growth.
- WebSocket: Cross-Site WebSocket Hijacking origin check, frame-size limits, unmasked/oversized control frames rejected, panics in callbacks recovered.
- Tenant IDOR: `Update` re-stamps the tenant so the owner cannot be changed via mass assignment.
- BFF cache: keys isolated per `Authorization` + tenant; responses with `Set-Cookie`/`private`/`no-store` are never cached; bounded entries and upstream body size.
- pprof: capped profile duration, one concurrent profile. Keycloak admin client escapes path segments. JWKS decode limited to 1 MiB.
- Data race fixed in `Server` start/shutdown. CI: per-module matrix, `-race`, gofmt, `govulncheck`, Dependabot.

### Performance
- Per-request map allocation removed from the router (`Ctx.SetRoute`): `/ping` 400 B/4 allocs → 48 B/1 alloc.
- JSON bind skips the extra `map[string]any` decode when the type has no `default:` tags (`/echo` 31 → 9 allocs, ~2.2× faster in-process).
- `c.JSON` reuses a pooled encoder+buffer; `UUID.String` formats on the stack (request IDs stay time-ordered UUIDv7); access-log timestamp cached per second.
- HTTP, same machine, 1 core server: `/echo` 66k → 79k req/s (+19%), `/ping` 103k → 106–114k.

### Conventions & migration (2.0.0)
- **Config errors are deferred, not panics:** misconfiguration in the chain is collected and returned by `app.Err()` / `Listen`. Must-style panics remain only in pure constructors.
- **Multi-tenant by convention:** models with a `tenant_id` column are auto-scoped; tenant claim names `tenant_id`, `tenantId`, `tid`, `org_id`, `organization_id`; opt out with `NoTenantScope`.
- **Client IP by convention:** private/loopback peers are trusted by default (`Config.TrustedProxies` to override); rate limiting uses `c.ClientIP()`.
- **Chainable app middleware:** `app.CORS()`, `app.Compress()`, `app.Timeout()`, `app.RateLimit()`, `app.ETag()`, `app.ResponseTime()`; new `nitro.Skip`, `BasicAuth`, `APIKeyAuth`, `RedirectHTTPS`.
- **UUIDv7:** request IDs/`uuid.New` are v7; generator stays monotonic when the clock steps backwards and uses one entropy read per millisecond.
- **Env by convention:** `nitro.New()` reads `NITRO_ENV` when `Config.Env` is unset; new `nitro.ParseEnv`; CLI templates no longer hard-code `EnvDevelopment`.
- **Query binding by convention:** untagged scalar fields bind from `?snake_case` (`c.BindQuery`, `nitro.Handle`).
- `app.Resource(&M{}, nitro.NewMemoryRepo[M]())` accepts a memory repo directly (`AsResourceRepository`); previously only `AdaptRepo`/`nitro.Resource` worked.
- **Resource filters:** `<field>_gt|gte|lt|lte|contains|icontains` on any `Filterable` field; the in-memory repo now honours operators, `FilterRules`, snake_case names and embedded `nitro.Model` fields, and sorts numbers numerically.
- **CI:** `sql-integration` (Postgres/MySQL/MariaDB/SQL Server; `NITRO_SQL_REQUIRE=1` fails instead of skipping) and `fuzz-smoke` jobs. Docs: `cache.md`, `production.md`. `nitrogen doctor` flags stale `modelgen`.
- **CLI:** generated Go is gofmt-formatted; `make:scaffold --tenant`; a second `--resource` scaffold is now wired into `main`; memory scaffolds emit a CRUD test; AI rule files (`ai:init`) carry the v2 conventions. New examples: `cli-shop`, `conventions`.
- Removed duplicate/deprecated names and their replacements: see [`docs/guide/conventions.md`](docs/guide/conventions.md). Executable contract: `pkg/coc_contract_test.go`.

### Added (this release)
- `c.Detach()`: request-scoped `context.Context` that is safe to use after the handler returns (the `*Ctx` itself is pooled; never retain it).
- `nitro.Adapt` / `nitro.AdaptFunc`: mount any `net/http` handler (e.g. `promhttp`).
- `nitro.ProblemDetails()` / `usecases.ProblemDetails()`: opt-in RFC 9457 (7807) `application/problem+json` error handler (`Config.ErrorHandler`); never leaks internal errors.
- `nitro.Handle[In, Out]` typed handlers (path/query/body binding + validation), `nitro.Empty`; `nitro.Deprecated` (Deprecation/Sunset/Link headers). Docs: [`docs/guide/typed-handlers.md`](docs/guide/typed-handlers.md).
- `benchmarks/`: reproducible HTTP comparison (`run.sh`) against fasthttp/Fiber/Gin/Echo/chi/net/http; in-process `Perf*` benchmarks. See [`docs/guide/performance.md`](docs/guide/performance.md).
- `LoggerConfig.Async` now batches writes (one syscall per ≤64 KiB); `LoggerDropped()` counter.
- `nitro.RegisterPlural` and built-in irregular plurals (`Person → people`, `Child → children`) for the resource-path convention.
- AI docs condensed (`AI.md`, `CLAUDE.md`); BLinq is the default but other stacks are no longer "refused".


### Added
- **Shared-schema multi-tenant Resource:** `Claims.TenantID` / `c.TenantID()`, `nitro.TenantMiddleware`, `ResourceConfig.TenantScope` + `TenantFromCtx` / `FillTenantID` (List filter, Get/Update/Delete check, Create stamp). Example: [`examples/v1/resource-tenant-scoped`](examples/v1/resource-tenant-scoped/). Docs: [`docs/guide/resource-security.md`](docs/guide/resource-security.md).
- **Optional `json`/`db` model tags:** fields without an explicit tag default to snake_case of the Go field name (independently for the JSON key and the DB column); explicit tags keep taking priority, zero behavior change for fully-tagged models. Runtime rename engine: `usecases/jsontag` (used by `c.OK`/`c.Created`, `nitro.Parse[T]`/`c.BodyJSON`, PATCH); persistence fallback in `data/adapter/pgx`; `codegen/json` fast path (`AppendJSON`) updated to match. Docs: [`docs/guide/models.md`](docs/guide/models.md).
- **Resource DX:** `nitro.FillStringFrom` for denormalized fields in `BeforeWrite`; BLinq/pgx store aggregation helpers (`CountAll`, `CountBuckets`, …).

### Fixed
- **`AdaptRepo` / `CacheRepo` QueryableLister:** only advertise `ListQuery` when the inner repo implements it, so MemoryRepo + `TenantScope` / FilterRules still post-filter in Resource List (IDOR).

### Changed
- **Release branding:** Nitro / `nitrogen` banner and CLI report **2.0.0**.

## [pre-repo 2.0.0 line] - 2026-08-28

### Added
- **Business Resource:** `ResourceConfig.Commands`, `Queries`, and `Rules` on top of existing CRUD (`nitro.Cmd`, `nitro.Q`, `nitro.Rules`, `nitro.Page`).
  - Item commands: `POST /:id/{name}`; collection commands: `POST /{name}`; queries: `GET /queries/{name}`.
  - Conditional `.When(fn).Roles(...)` on commands; global rules before item commands.
- **Examples:** `examples/v1/order-business` (commands + multi-entity usecase), `examples/v1/resource-aftercreate` (`AfterCreate` hook).
- **Docs:** [`docs/guide/business-resource.md`](docs/guide/business-resource.md), [`docs/guide/resource-hooks.md`](docs/guide/resource-hooks.md).

### Changed
- **Release branding:** Nitro / `nitrogen` banner and CLI report **2.0.0**.

## [1.4.1] - 2026-08-26

### Changed
- **Release branding:** Nitro / `nitrogen` banner and CLI report **1.4.1**.
- **Fase 6 (maintenance):** dependency bumps (fasthttp `v1.73`, OTel `v1.46`, go-redis `v9.22`, `x/crypto` ≥`v0.54`, oauth2; `go 1.25` on touched modules); split `pkg/nitro.go` into `errors` / `middleware` / `bff` / `app_routes` / `resource`; BLinq `MaxPlaceholders` + `CreateMany` multi-row without RETURNING + quoted upserts + `Order.Dir` validation; more Keycloak/cache/SafeMap/CLI e2e tests.
- **Fase 5 (API/DX):** `Resource()` computes `toOpts` once; deprecate `ResourceOpts` helpers / `SerialID` / package-level `OK`·`Created`; `ConfigOf` no longer `os.Setenv`; not-found without substring matching; CLI unknown flags rejected + scaffold `--force` + migration dedupe + `UserID`→`user_id`; OTel `SpanKindServer`, `http.route` template, `request.id`, ≥500 as error.

### Security / reliability (Fases 1–4, shipped in tree)
- JWT HS256 + issuer/audience hardening; AMQP reconnect + panic recover on workers; Gateway cache key fix; MemoryRepo/SafeMap race fixes; rate-limit XFF rightmost-untrusted; access-log JSON escaping; migration advisory lock + `WithRetry` for deadlocks.

## [1.4.0] - 2026-08-24

### Added
- **AI / BLinq compliance:** policy *unspecified ORM ⇒ BLinq* + **Resource-first** shape (`app.Resource` + `routes.go`, minimal main/handler/repo/usecase) in `AGENTS.md`, `AI.md`, Cursor rules, playbooks, and `ai:init` templates; pasteable `.cursor/prompts/nitro-blinq.md`; `nitrogen doctor` warns on GORM/sqlx/sqlc without opt-out and on SQL apps missing `modelgen` / `StoreOf`.
- **Example `cleanarch-blinq`:** regenerated with `make:scaffold … --resource` (CA folders + BLinq `app.Resource`, thin `main` / `routes.go`).
- **Resource list anti-DoS:** `ResourceConfig` / `ResourceOpts` fields `DefaultPerPage`, `MaxPerPage` (default 100), `MaxPage`, `MaxFilters` (16), `MaxFilterValueLen` (200), `MaxInMemoryFetch` (2_000). List clamps `?page` / `?per_page` and truncates filter values.
- **`nitro.RateLimitWith(max, window, keyFunc)`** plus `IPKeyFunc` / `APIKeyKeyFunc` / `CompositeKeyFunc` / `HeaderKeyFunc` for keyed buckets (e.g. IP + `X-API-Key`).

### Ergonomics (Phases 0–5)

One happy path from empty repo to filterable CRUD + SQL:

```bash
nitrogen new shop --module github.com/acme/shop
nitrogen install sqlite   # or postgresql | mysql | …
nitrogen make:scaffold Product name:string price:float64 active:bool --resource
nitrogen up
```

Docs and AI playbooks now point at **`sqladapter.OpenEnv` / `platform.Open`**, CLI lifecycle (`up` / `down` / `doctor --db` / `seed` / `openapi` / `install auth-*`), and Go **1.25+** for SQL adapters. See [`ERGONOMICS_ROADMAP.md`](ERGONOMICS_ROADMAP.md) and [`examples/README.md`](examples/README.md).

- **Multitenant helpers:** `nitro.TenantModel` (Model + `tenant_id`), `nitro.Attrs` (`JSON[map[string]any]`), `nitro.FieldDef` + `ValidateAttrs` for per-tenant custom fields.

- **`sqladapter.OpenTest` / `OpenTestMigrate` / `OpenTestFile`**: in-memory (or temp-file) SQLite for tests without Docker.
- **`nitrogen install sqlite`** plus `NITRO_DB_DRIVER` / `DATABASE_URL` on every SQL install. Writes `internal/platform/db.go` and wires `cmd/server`.
- **`nitrogen install auth-jwt` / `auth-keycloak`**: env keys, config fields, `internal/auth` stub, protected sample route wired in CA `main`.
- **SQL engine switch**: re-`install` upserts driver/DSN, warns on dialect migrations, comments rival compose SQL services.
- **`nitrogen openapi`**: static OpenAPI 3 JSON from route/Resource/AuthRoutes scan (`--bearer`, `-o`).
- **Dev 500 UX**: development responses include `request_id` + OTEL quiet hint (`NITRO_OTEL=0`).
- **`make:scaffold`** emits real CA handlers (`Parse`, `c.OK`), `Register(app, repo)`, Filter + pagination, BLinq repo when SQL is present, `--data` + `migrate:make` in one step.
- **`make:scaffold --resource`**: model + `app.Resource` (no CA usecase stack). `--with-seed` / `--with-factory` invoke the matching generators.
- CA List: `?q=` / `?active=` / `?sort=` / `page` / `per_page` → Filter → BLinq `WhereIf` + `OrderBy`.
- **`sqladapter.DB.Tx`**: dialect-agnostic transactions; `StoreOf` inside the callback shares the tx.
- **BLinq bulk:** `store.CreateMany` + `pgx.WithChunk`; `store.UpdateWhere`; DSL `query.InsertInto().Rows().Chunk()` and `query.Update().OmitReturning()`.
- **BLinq codegen:** pointer columns (`*string`, `*int64`, …) and `nitro.UUID` FKs emit `Field[*T]` / `Field[string]` in `modelgen`.
- **`nitrogen make:relation`**: `belongs_to` / `has_many` → FK field, `modelgen/relations_gen.go`, migration stub. Runtime: `field.BelongsTo`.
- **`nitrogen seed`**: runs `./cmd/seed` (auto-writes runner from `Seed*` + `sqladapter` when SQL is installed).
- **`make test-sql-integration`**: live CRUD smoke (Postgres, MySQL, MariaDB, SQL Server, SQLite) via Docker Compose.
- **`nitrogen up` / `down`**: compose up → wait DB → MigrateUp → run `cmd/server`; `down` tears compose down (`--volumes` optional). Flags: `--no-dev`, `--seed`, `--migrate-only`.
- **`nitrogen doctor --db`**: ping via app-module `sqladapter.OpenEnv` with actionable tips.
- **CI** `nitrogen-sqlite-smoke`: `new` → `install sqlite` → `scaffold --resource` → `up --migrate-only` → `go test` (OpenTestMigrate).
- Example [`sql-tx`](examples/v1/sql-tx/): parent+child in `db.Tx` via `OpenTest` (no Docker).
- Scaffold / `make:test`: real `*_store_test.go` using `OpenTestMigrate` (path-aware of `go test` cwd).
- **`install auth-jwt` / `auth-keycloak`**: sample `RequireRole` admin route.
- Example [`cleanarch-blinq`](examples/v1/cleanarch-blinq/) uses `platform.Open` + `sqladapter.StoreOf` (same path as CLI apps).
- README / welcome / FAQ: example matrix, Go 1.25+ for SQL, deprecated-alias table.

### Changed
- **Release branding:** Nitro / `nitrogen` banner and CLI report **1.4.0**.
- **AI agent guidance:** BLinq is the **mandatory default** SQL stack for agents (`AGENTS.md`, `AI.md`, `CLAUDE.md`, Cursor rules, playbooks, `nitrogen ai:init` templates). GORM/sqlx/sqlc/raw SQL only when the user explicitly opts out of BLinq. Prefer `sqladapter.OpenEnv` / `platform.Open` + `modelgen` / `WhereIf`.
- **`make:scaffold` field tags:** `blinq:"index"` is no longer added to every scalar. Only common filter/FK names (or explicit `name:string:index`) get an index — avoids bloating migrations with indexes on `description`/`notes`/`password`/etc.
- **SQL + cache wiring:** `cmd/server` opens the DB with `sqladapter.OpenEnv` and optional Redis L2 via `REDIS_ADDR` inline — no `internal/platform/db.go` / `MaybeCache` for SQL.
- **`nitrogen doctor`**: warns when Go < 1.25 and SQL adapters are required; flags stale `*_tablename_gen.go`.
- **`nitrogen make … --data`** no longer writes `*_tablename_gen.go`. Resource CoC uses the plural of the type; custom tables: `func (T) TableName() string` on the model. Stale generated tablename files are removed on regenerate.
- Goose SQL skips `pgcrypto` for non-Postgres dialects. Migration metadata table is dialect-aware. Indexes omit partial `WHERE` on MySQL; SQL Server uses `IF NOT EXISTS` for indexes.
- Timestamp defaults are dialect-aware (`NOW()` / `CURRENT_TIMESTAMP` / `SYSUTCDATETIME()`).
- `nitro.DateTime` implements `driver.Valuer` as `time.Time` (MySQL/MariaDB/SQL Server); Scan still accepts RFC3339 and Go `time.Time.String()`.
- MSSQL pagination: `TOP (n)` for limit-only; `OFFSET/FETCH` only with a positive offset (auto `ORDER BY (SELECT NULL)` when needed).
- **Go 1.25+** required for `data/adapter/pgx` and `data/adapter/sql` (pgx v5.10).
- Integration compose binds Postgres on `127.0.0.1:15432` (avoids host port clashes).
- Docs/playbook prefer `sqladapter.OpenEnv` / `platform.Open` as the app DB entry point.

## [1.3.0] - 2026-08-10

### Added
- **RabbitMQ adapter** (`adapters/amqp`) implementing `queue.Queue` with topic exchange, prefetch, retry republish, DLQ
- **Queue DX** on facade: `NewInternalQueue`, `NewRabbitQueue`, `NewRabbitQueueFromEnv`, `Publish` / `PublishWithOptions`, `Subscribe` / `SubscribeRoute`, `WithTenantID`
- **Resilience facade**: `NewResilience`, `Execute` / `ExecuteNamed`, named circuit breakers, optional Redis store (`NewRedisBreakerStore`)
- **CLI** `nitrogen new --worker` (cmd/worker + consumer stub + health `:8081`)
- **CLI** `nitrogen new --fleet --services …` (go.work, compose, Makefile, gateway HTTP + workers)
- **CLI** `nitrogen install rabbitmq|minio`
- Guide [`docs/guide/messaging.md`](docs/guide/messaging.md) + example [`examples/v1/worker-amqp`](examples/v1/worker-amqp/)

## [1.2.1] - 2026-08-10

### Security
- **BFF**: inject Authorization / X-User-ID on the **request** (not response)
- **Web BFF CSRF**: `crypto/rand` tokens + constant-time compare; typed Secure/SameSite cookies
- **OAuth**: persist `state` in HttpOnly cookie across redirect/callback
- **JWT**: reject short secrets; refresh rotation + in-memory JTI revoke; preserve email/roles on refresh
- **Resource/SQL**: empty `Sortable` disables sort; unknown filter/sort fields ignored; `QuoteIdent` escapes quotes
- **Static**: path containment via `filepath.Rel`
- **ClientIP**: ignore XFF unless `SetTrustedProxies`; add `RemoteIP` / `SafeRedirect`
- **InternalError**: generic client message
- **Request-ID**: sanitize client-supplied values
- **PublicStack**: warn when used in production
- **CLI install**: random DB passwords; bind compose ports to `127.0.0.1`

## [1.2.0] - 2026-07-29

### Added
- **`make:usecase <Name> [OpName]`** — CRUD usecases or a single extra op; Makefile target `usecase`

### Changed
- **`nitrogen new` / `install`**: write/update only `.env` and `.env.example` (no `.env.dev`/`.stage`/`.prod`)
- Banner/CLI version: **1.2.0**

## [1.1.0] - 2026-07-29

### Added
- **CLI Clean Arch**: `nitrogen new` (default CA / `--basic` / `--microservice`) with `cmd/server` + `internal/{model,usecase,handler,repository}`, envs (`.env`/`.env.dev`/`.env.stage`/`.env.prod`), Dockerfile, Makefile, health
- **`make:scaffold`**, `make:model` (`-r`/`--resource`), `make:handler`, `make:usecase`, `make:repository`, `migrate:make`
- **`make:seeder`**, `make:factory`, `make:middleware`, `make:job`, `make:test`; `model:show`; richer `doctor`
- **`nitrogen install`** `postgresql|mariadb|mysql|sqlserver|redis` — config + env + **docker-compose**
- Listen banner with `Listening on http://…` plus automatic shutdown logs
- Project/CRUD name normalization (Go singular/plural)
- Agent docs: `AGENTS.md` (CLI first), `CLAUDE.md`, `AI.md`, `docs/guide/cli.md`
- Example [`examples/v1/cleanarch`](examples/v1/cleanarch/)

### Removed
- `nitrogen g` / `generate resource`
- `nitrogen new --stack` / `--standard` / `--complex`
- `make:migration` (use `migrate:make`)

## [Unreleased]

### Added
- **Text/code helpers** on the `nitro` facade: `IsDigits`, `DigitsOnly`, `SplitName`, `NormalizeEmail`, `IsEmail`, `RandomDigits`, `RandomAlphaNum`, `RandomHex`, `IfBlank`, `Ptr`, `Deref`, `Contains`
- **Portable types**: `nitro.UUID`, `Date`, `TimeOfDay`, `DateTime`, `Money`, `Measure`, `Decimal`, `Latitude`/`Longitude`/`LatLng` (WGS84, `NUMERIC(12,9)` per dialect); `pkg/convert`; locale; guides + `examples/v1/portable-types`

### Changed
- **`nitro.Model`**: `ID` is `nitro.UUID`; timestamps are `nitro.DateTime` (UTC) — no longer `string`/`time.Time`
- **JSON migrations**: column type per dialect (JSONB / JSON / TEXT / NVARCHAR), not Postgres JSONB only
- **Release branding**: Nitro is **v1.0** — `examples/v1`, docs `philosophy` / `api-changelog`, `fasthttp.Version = "1.0.0"`. Old “Nitro v2” / `examples/v2` references updated; pre-1.0 APIs treated as legacy.

### Added
- **CoC / DRY sync**: `OpenEnv` / `Open("")`; `Config.EnableLogger` / `EnablePrometheus`; `app.Resource(&M{}, store)` path plural + `TableName()` from `//nitro:table=` (blinq gen); `blinq:"index"` → Filterable/Sortable; `nitrogen g resource`; store sem `AdaptStore` (`AsResourceRepository`); `c.UserUUID` / `UserInt64`
- **BodyParser form pipeline**: form/query agora aplica `default:` + `transform:` + validate (igual JSON)
- **OnRoute + Name**: `.Name(...)` dispara `OnRoute` de novo com `RouteInfo.Name`
- **Fiber-inspired HTTP DX**: `c.BodyParser` / `Parse` Content-Type-aware (JSON, form, multipart); `app.Mount`; `app.Hooks` (`OnListen`/`OnShutdown`/`OnRoute`); named-route `GetRouteURL`/`URL`; `StaticConfig{SinglePageApp}`; `c.Param`/`Query`/`Header` defaults — guides [`request`](docs/guide/request.md) · [`routing`](docs/guide/routing.md)
- **Cache DX** `nitro.Remember[T]`, `nitro.Bust` — list cache sem Get/Set manual
- **Validate** `nitro.MustValidate` — validate after `UnmarshalJSONPatch` (not after `Parse`)
- **Store** `GetBy(ctx, column, value)` — lookup por coluna sem `field.Col` manual
- **DX CoC** `nitro.Ping` / `PingTimeout`, `nitro.Me` / `App.MeRoute`, `nitro.AppInfo` / `App.Info`
- **DX CoC** `pgx.OpenFrom` + `db.Ready` (EnsureSchema + MigrateUp)
- **DX CoC** `c.CSVFrom(filename, rows, cols...)` + `nitro.JSONCell`
- **Resource** `Only` / `Except` / `Replace` — declarar subset de CRUD ou trocar um handler
- **DX-1** `nitrogen new --stack postgres,redis,keycloak` (ConfigOf + OpenWithPool/EnsureSchema + Redis + Keycloak)
- **DX-2** `nitrogen make:dto` — `{Type}Public` + `ToPublic` via `nitro:"public"` / `//nitro:public`
- **DB** `db.EnsureSchema`, `nitro.FilterActive` / `ActiveStatus` / `SoftDeleteSQL`
- **P1/P2** NamedHealth, Resource hooks/OnMutate/ExtraRoutes/Aliases, RequireRoles+bypass, BindMap, PATCH merge, RegisterBRValidators, OpenWithPool/SearchPath, config overwrite/Duration/URL — exemplo [`examples/v1/resource-dx`](examples/v1/resource-dx/)
- **`c.CSV` / `c.Attachment` (IO-1)**: export CSV + download helper; `File` usa `filename="…"` quoted
- **Resource middleware + Group.Resource (RS-1/RS-2)**: `ResourceConfig.Middleware` / `ReadMiddleware` / `WriteMiddleware`; `group.Resource(path, model, …)` inherits group middleware
- **ConfigOf CFG-1/2/4**: nested structs (+ `env:"PREFIX_"`), `[]string` CSV, aliases `env:"A,B"`; guide [`docs/guide/config.md`](docs/guide/config.md)
- **JSONB first-class** (`nitro.JSONSlice[T]`, `nitro.StringSlice`, `nitro.JSON[T]`): Valuer/Scanner + JSON HTTP; migration codegen → `JSONB` + default `'[]'::jsonb`; tags `db:",jsonb"` / `blinq:"gin"`; AST reconhece `nitro.JSONSlice[…]`
- **`transform:` tags** (`usecases/transform`): `titlecase`/`titlecase=pt`, `digits`, `trim`, `upper`/`lower`, `camelcase`/`pascalcase`, `snake`/`kebab`, `squeezespace` — pipeline **default → transform → validate** em `Parse`/`BodyJSON`/Resource; helpers `nitro.ApplyTransforms`, `TitleCase`, `Digits`, …; exemplo [`examples/v1/transform`](examples/v1/transform/)
- **Redis cache L2** (`adapters/cache`): `NewRedisCache` / `nitro.RedisCache` (JSON + Invalidate/Clear por prefixo)
- **OIDC JWKS** (`adapters/auth/oidc`): `TokenProvider` RS256 via JWKS; flatten `realm_access` / `resource_access` → `Claims.Roles`
- **Keycloak** (`adapters/auth/keycloak`): password grant, Admin API, `nitro.NewKeycloak` + exemplo [`examples/v1/auth-keycloak`](examples/v1/auth-keycloak/)
- **blinq/scan**: `sql.Scanner` support in `MapToStruct` (JSONB / custom types)
- Tag `default` / `validate:"default=…"` on bind (`Parse`/`BodyJSON`): defaults only when the JSON key is absent
- Unified Gateway bootstrap: `pgx.Open` / `DSN` / `DB.SQL()` / `DBTX()` / `StoreOf` (`NITRO_DB_QUIET=1` automatic)
- Gateway para GORM/sqlx/sqlc: `gw.Instrument`, `OpenSQL`, `AdaptDBTX` (+ exemplos `db-gorm`/`db-sqlx`/`db-sqlc`)
- Exemplo [`examples/v1/blinq-extras`](examples/v1/blinq-extras/) (MERGE / Mongo / Elastic / GraphQL / migrate); `pgx.AdaptMigrate`; `GooseFSSource`; testes pool/migrate/scan/cache/field/expr
- **BLinq Fase 11**: MSSQL `MERGE` upsert; `provider/elastic` + `elastic/client`; `provider/graphql`; `provider/mongo/driver` (submodule); `blinq/migrate` (FSSource/Runner)
- **BLinq Fase 10**: OuterRef/RawPred/JSON ops, query DX (`Any`/`All`/`After`/`Into`/`CollectTyped`), QuoteIdent + MSSQL dialect, `SuggestIndexes`, `sql.WithCache`, mongo `$lookup`, Resource `FilterRules`/`ListFilter.Op`, `provider/sql/postgres`, `BaseRepository`→`Store`
- **BLinq** (`blinq/`): DSL AST + providers (memory, SQL Postgres/SQLite/MySQL sobre Gateway)
  - Soft/Tenant, aggregations, Paginate, Join/GroupBy, Upsert, Bulk+Chunk, Inc
  - Concurrent memory (`WithConcurrency` / `WithUnordered`), `Cache`/`CacheTag` + `provider.Wrap` (hooks/telemetry)
  - Include anti-N+1 (`CollectMaps`), Exists/InQuery, `ForEach`, `provider/mongo` + `provider/api` (subset)
  - Nitro Resource: `pgx.AdaptStore` + `QueryableLister` (filtros List → BLinq)
  - Typed codegen: `codegen/blinq` (`nitrogen make … --data`)
  - Guide [`docs/guide/blinq.md`](docs/guide/blinq.md) + exemplo [`examples/v1/db-blinq`](examples/v1/db-blinq/)
- **Validator registry** (`usecases/validator`): `NewValidator`, `Register` / `RegisterAlias` por App, `FieldContext`, `StrictUnknown` (default true)
- Custom validation messages: tags `message` / `msg_<rule>` + placeholders `{field}`, `{param}`, `{value}`, `{rule}`
- Alias `same` / `eq_field` → `eqfield`
- Validator `Options` (`StopOnFirstError`, `StrictUnknown`) + type-meta cache (`sync.Map`) — `BenchmarkValidateStruct` ~2× faster
- Example [`examples/v1/validation`](examples/v1/validation/) (CPF/CNPJ na app) + guide [`docs/guide/validation.md`](docs/guide/validation.md)
- Fasthttp optimization (6 phases): 15-28% performance improvement
- Pre-compiled middleware pipeline (9-12x faster than recursive next())
- Security headers middleware (auto-applied: CSP, HSTS, X-Frame-Options, etc.)
- RequestID middleware (UUID v7, auto-applied)
- Rate limiting middleware (sliding window + token bucket)
- Timeout middleware (cooperative + server-level)
- Recover middleware (env-aware stack traces)
- CORS middleware (preflight handling, origin validation)
- Compression middleware (gzip with sync.Pool)
- JWT middleware with token validation
- Session middleware (memory + Redis)
- Role-based access control (RequireRole)
- OAuth2 support (Google, GitHub, custom providers)
- BFF pattern (Web + Mobile configurations)
- Web BFF with cookie-based auth, CSRF protection
- Mobile BFF with device tracking, compact responses, push notifications
- Aggregation handler for fan-out requests
- Proxy handler for upstream microservices
- Gateway pattern (observability: trace, metrics, log, cache)
- Generic Repository pattern (CRUD + pagination)
- PostgreSQL adapter (pgx v5, connection pool, transactions)
- Code generation (router, data models, BLinq fields)
- Compiled router (zero-allocation dispatch)
- Health check endpoints (/health, /ready, /live)
- Resource auto-registration (CRUD routes from models)
- Graceful shutdown with signal handling
- Auto TLS (Let's Encrypt)
- Structured logging adapter
- OpenTelemetry tracing and metrics adapters

### Changed
- Data layer queries: `data/runtime` LINQ **removed** — replaced by BLinq (`blinq/query`, migrated `Store`/`Repository`)
- Validation engine moved to `usecases/validator` (facade `usecases` / `nitro` unchanged)
- Unknown validate rules now return `Rule: "unknown"` by default (`StrictUnknown`); set `false` for legacy skip
- Renamed `SetParamPublic` to `SetParam` in domain.Ctx
- Improved CORS security (rejects wildcard + credentials)
- Compress middleware skips responses under 1024 bytes

### Fixed
- `blinq/migrate`: default placeholders `$1`/`$2` (Postgres); `TIMESTAMPTZ` schema; optional `BindQMark`
- Compiled router import path (`github.com/rodrigocelsius/go-nitro/v2` → `github.com/rodrigocelsius/go-nitro/pkg/v2`)
- Auth-self example (`c.Context` → `c.Context()`)

## [0.1.0] - Initial Release

### Added
- Core HTTP framework with fasthttp adapter
- Route registration (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)
- Route parameters (up to 8, zero-allocation)
- Route groups with prefix and middleware
- Context API (params, headers, body, JSON, bind)
- Error constructors (BadRequest, Unauthorized, NotFound, etc.)
- Middleware system (global pre/post, route-specific, group)
- Clean Architecture patterns
