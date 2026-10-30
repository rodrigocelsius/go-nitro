# CLI — nitrogen (v1.4)

**Language:** [English](cli.md) · [Português](../pt/guide/cli.md)

Official tool for Nitro scaffold and DX. Binary: `cmd/nitrogen`.

> **Data default:** SQL apps use **BLinq** via `sqladapter.OpenEnv` (`NITRO_DB_DRIVER` + `DATABASE_URL` / `DB_DSN`). Postgres uses pgx under the hood; MySQL/MariaDB, SQL Server, and SQLite use `database/sql`. `nitrogen make … --data` generates `modelgen`. Do not emit raw SQL or pick another ORM unless the requirement asks for it.

```bash
go install ./cmd/nitrogen   # or: go run ./cmd/nitrogen …
nitrogen version            # 2.0.0+
```

Agents: when **creating a system**, use these commands before writing files by hand — see [`AGENTS.md`](../../AGENTS.md).

## Project

| Command | Description |
|---------|-----------|
| `nitrogen new <name>` | Clean Arch: `cmd/server` + `internal/{config,model,usecase,handler,repository}` + migrations + envs + Dockerfile + Makefile. Does **not** create `seed`/`factory`/`middleware`/`job` |
| `nitrogen new <name> --basic` | Minimal `main.go` only |
| `nitrogen new <name> --microservice` | CA + `docker-compose` + health/service name |
| `nitrogen new <name> --worker` | AMQP worker: `cmd/worker` + `internal/consumer` + health `:8081` |
| `nitrogen new <name> --fleet --services a,b` | Monorepo: `go.work` + compose + Makefile; `gateway`→HTTP, others→workers |
| `--module <path>` | Go module path |
| `--nitro-path <dir>` | local `replace` for the Nitro monorepo (optional: auto-detected from parent checkout or `NITRO_PATH`) |

Generated env files: `.env` and `.env.example` only.  
Also: `.gitignore`, `.dockerignore`, and **AI agent rules** (`AGENTS.md`, `CLAUDE.md`, `AI.md`, `.cursor/rules/nitro.mdc`, `.cursor/prompts/nitro-blinq.md`) on every tier.

## AI agents

```bash
nitrogen ai:init              # cwd
nitrogen ai:init --dir ./app
nitrogen ai:init --force      # overwrite existing files (refresh BLinq STOP policy)
```

Writes a **consumer** contract (English): **unspecified ORM ⇒ BLinq**, plus `Parse`, `c.OK`, and CLI-first generators. Includes a pasteable BLinq reminder under `.cursor/prompts/nitro-blinq.md`. `nitrogen new` runs this automatically. Idempotent without `--force`. `nitrogen doctor` warns on GORM/sqlx/sqlc without an opt-out doc.

Tx without Docker: [`examples/v1/sql-tx`](../../examples/v1/sql-tx/).

## Install (drivers)

```bash
nitrogen install postgresql   # aliases: postgres, pg
nitrogen install mysql | mariadb | sqlserver | sqlite
nitrogen install valkey        # BSD-3 fork of Redis 7.2 (recommended)
nitrogen install redis
nitrogen install rabbitmq
nitrogen install minio
nitrogen install auth-jwt       # aliases: jwt, auth
nitrogen install auth-keycloak  # alias: keycloak
```

Effects:

1. Structs in `internal/config` (`DBConfig`, `RedisConfig`, `JWTSecret` / `KeycloakConfig`, …)
2. `NITRO_DB_DRIVER` + `DB_DSN` / `DATABASE_URL` in `.env` / `.env.example` (SQL); `JWT_SECRET` or `KEYCLOAK_*` for auth
3. SQL open + optional Redis L2 in `cmd/server` (`sqladapter.OpenEnv`, `REDIS_ADDR` → `SetCache`); `internal/platform` only for rabbitmq/minio helpers
4. Matching service in `docker-compose.yml` (skipped for sqlite)
5. Local `go.mod` `replace`s for adapters when `--nitro-path` / existing pkg replace is present
6. `cmd/server/main.go` wired with `MigrateUp` + health ping (SQL)
7. **Auth:** `internal/auth/{jwt,keycloak}.go` + protected sample route (`/profile` or `/api/me`) wired in `main`

### Engine switch (SQL)

Re-running `nitrogen install <other-sql>` **upserts** driver/DSN keys (does not append duplicates), warns when `migrations/` may be dialect-specific, and comments rival SQL services in `docker-compose.yml`.

```bash
nitrogen install postgresql
# … later, local tests without Docker:
nitrogen install sqlite
nitrogen doctor --db
```

### OpenAPI

```bash
nitrogen openapi                     # writes openapi.json from static route scan
nitrogen openapi --bearer -o docs/openapi.json
```

Detects `Get`/`Post`/…, `Resource`, `AuthRoutes` / `KeycloakAuthRoutes` (same scanner as `nitrogen routes`). For a **live** spec at runtime, prefer `app.OpenAPI(...)` / `app.Swagger(...)` after registering routes.

## `make:*` generators

| Command | Effect |
|---------|--------|
| `make:scaffold Product name:string price:float64` | **CA default:** model + usecases + handlers + repo. SQL → BLinq + migrate + `platform.Open` |
| `make:scaffold … --resource` | **Resource mode:** model + `app.Resource` (no 5 usecases). Also auto when the app has **no** `internal/` (e.g. `nitrogen new --basic`) **and** SQL is installed |
| `make:scaffold … --tenant` | Adds a `tenant_id` column ⇒ the Resource is tenant-scoped automatically (JWT claim) |
| `make:scaffold … --with-seed` / `--with-factory` | Also runs `make:seeder` / `make:factory` |
| `make internal/model/product.go --data` | **BLinq**: DSL in `internal/modelgen/` — **default with Postgres** |
| `make … --data --same-package` | Legacy: DSL + `TableName` in the same package (`*_model_gen.go`) |
| `make:model Product` | Empty struct |
| `make:model Product -r` / `--resource` | CoC model (`nitro.Model`, tags, `//nitro:table=`) |
| `make:handler` / `make:usecase` / `make:repository` | Isolated layer (`make:usecase Product [OpName]` for CRUD or a standalone usecase) |
| `make:middleware Auth` | Creates `internal/middleware/` (on-demand) + stub; removes `.gitkeep` if present |
| `make:job SyncOrders` | Creates `internal/job/` (on-demand) + schedule stub |
| `make:seeder` / `make:factory` / `make:test` | Creates `internal/seed` or `factory` only when called; tests |
| `make:relation A belongs_to\|has_many B` | FK on child model + `modelgen/relations_gen.go` + migration stub |
| `make:dto` / `make:controller` | Public DTO / controller from model |
| `model:show Product` | Lists fields and tags |

**CA vs Resource:** use `--resource` for plain table CRUD; keep default CA when you need custom usecases/domain rules.

**Names:** `products`, `Product`, or `Products` → type `Product`, package `product`, table `products`.

Folders `internal/{seed,factory,middleware,job}` do **not** come from `nitrogen new`. They are created only by the matching `make:*` (or if the system requirement needs them). When writing the first real file into a directory that has `.gitkeep`, the CLI removes `.gitkeep`.

## Migrations

```bash
nitrogen migrate:make internal/model/product.go [--name create_products]
nitrogen migrate up          # via goose, if installed
```

Do not use `make:migration` (removed).

## Runtime / quality

| Command | Use |
|---------|-----|
| `up` | `docker compose up -d` (if present) → wait DB → `MigrateUp` → `go run ./cmd/server` (flags: `--no-dev`, `--seed`, `--migrate-only`) |
| `down` | `docker compose down` (`--volumes` optional) |
| `dev` | Hot reload (`air` if available) |
| `routes` | List routes |
| `openapi` | Write `openapi.json` from static route scan (`--bearer`, `-o`). Resource routes get schemas from the model struct: `<T>` (server-owned fields `readOnly`), `<T>Input` (POST body), `<T>Patch` (PATCH body), `<T>List` (paginated envelope), plus `Error` on 4xx/5xx; JSON names follow the snake_case convention |
| `test` / `cover` | Tests and coverage |
| `doctor` | Go/git + layout + AI rules; Go 1.25+ when SQL present; warns on `*_tablename_gen.go`; runs `go build` + `go vet` and fails if the project does not compile (`--no-build` skips) |
| `doctor --db` | Ping via `sqladapter.OpenEnv` (actionable tips) |
| `seed` | Runs `cmd/seed` (auto-generated from `Seed*` when missing) |
| `audit` / `security` / `stress` / `profile` | Ops |

Happy path after install + scaffold:

```bash
nitrogen up
```

## Generated example

See [`examples/v1/cleanarch`](../../examples/v1/cleanarch/) (memory) and [`examples/v1/cleanarch-blinq`](../../examples/v1/cleanarch-blinq/) (Postgres + BLinq `products`) — `new` + `install postgresql` + `make:scaffold` now also runs `--data` and `migrate:make` when SQL is installed.

## Listen (Nitro runtime)

`app.Listen(":8080")` prints the banner (with **PORT**) and `Listening on http://…`. On graceful shutdown: `Nitro: shutting down…` / `Nitro: server stopped`. Do not duplicate those logs in `main`.
