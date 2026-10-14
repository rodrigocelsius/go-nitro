<div align="center">

# Nitro

**Fast, modular Go web framework** — fasthttp · Clean Architecture · BLinq · batteries included

[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/rodrigocelsius/go-nitro?style=for-the-badge)](https://goreportcard.com/report/github.com/rodrigocelsius/go-nitro)
[![Release](https://img.shields.io/github/v/release/rodrigocelsius/go-nitro?style=for-the-badge&include_prereleases&sort=semver)](https://github.com/rodrigocelsius/go-nitro/releases)
[![Stars](https://img.shields.io/github/stars/rodrigocelsius/go-nitro?style=for-the-badge)](https://github.com/rodrigocelsius/go-nitro/stargazers)
[![Issues](https://img.shields.io/github/issues/rodrigocelsius/go-nitro?style=for-the-badge)](https://github.com/rodrigocelsius/go-nitro/issues)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen?style=for-the-badge)](CONTRIBUTING.md)

<br />

Ship production APIs in Go **without** the weekend of glue: no hand-rolled CRUD,
no homemade `.env` parser, no SQL-string queries, no “remember to add Recover”.

Nitro is a **high-performance** web framework on
[fasthttp](https://github.com/valyala/fasthttp). One import, secure defaults,
typed Postgres (**BLinq**), JWT/Keycloak, and a CLI that scaffolds Clean
Architecture — so you sell the product, not the framework.

[Documentation](docs/README.md)
([English](docs/guide/README.md) · [Português](docs/pt/guide/README.md)) ·
[Examples](examples/README.md) ·
[Contributing](CONTRIBUTING.md) ·
[Security](SECURITY.md) ·
[AI / Agents](AGENTS.md)

</div>

---

## ⚙️ Installation

Nitro requires **Go 1.25+** (fasthttp 1.73 and pgx 5.10 set the floor).
Install or upgrade Go from the [official downloads](https://go.dev/dl/).

```bash
mkdir myapp && cd myapp
go mod init github.com/you/myapp
```

Add the public facade:

```bash
go get github.com/rodrigocelsius/go-nitro/pkg/v2
```

Install the CLI (scaffold, BLinq codegen, migrations):

```bash
go install github.com/rodrigocelsius/go-nitro/cmd/nitrogen/v2@latest
nitrogen version   # 2.1.x
```

> **Module paths:** every module follows Go major-version rules and ends in `/v2` (e.g. `github.com/rodrigocelsius/go-nitro/pkg/v2`). Tags are per module: `pkg/v2.0.0`, `domain/v2.0.0`, …  
> **Source / issues:** [github.com/rodrigocelsius/go-nitro](https://github.com/rodrigocelsius/go-nitro).

---

## ⚡️ Quickstart

```go
package main

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
	app := nitro.New() // Recover, Logger, MaxBody, RequestID, security headers

	app.Get("/", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"message": "Hello, Nitro!"})
	})

	nitro.Ok(app.Listen(":8080"))
}
```

```bash
go run .
# → http://localhost:8080
```

Scaffold a Clean Architecture app with SQL (SQLite needs no Docker):

```bash
nitrogen new myapi --module github.com/you/myapi
cd myapi
nitrogen install sqlite          # or: postgresql | mysql | mariadb | sqlserver
nitrogen make:scaffold Product name:string price:float64 active:bool --resource
nitrogen up                      # MigrateUp + server (compose when present)
```

Plain CA usecases (omit `--resource`), then `nitrogen up`. Auth: `nitrogen install auth-jwt`. Spec: `nitrogen openapi --bearer`.

---

## 🗺 Pick an example

| Goal | Commands / example |
|------|-------------------|
| HTTP hello | [`examples/v1/basic`](examples/v1/basic/) — `go run .` |
| CRUD in memory | [`examples/v1/crud`](examples/v1/crud/) or `cleanarch` |
| Resource + SQL filters | `nitrogen new …` → `install sqlite\|postgresql` → `make:scaffold … --resource` → `up` · live: [`db-blinq`](examples/v1/db-blinq/) |
| Clean Arch + BLinq | [`cleanarch-blinq`](examples/v1/cleanarch-blinq/) · CLI: `make:scaffold` (no `--resource`) |
| JWT / Keycloak | `nitrogen install auth-jwt\|auth-keycloak` · [`auth`](examples/v1/auth/) · [`auth-keycloak`](examples/v1/auth-keycloak/) |
| AMQP worker | `nitrogen new … --worker` · [`worker-amqp`](examples/v1/worker-amqp/) |

Full matrix: [`examples/README.md`](examples/README.md). CLI reference: [`docs/guide/cli.md`](docs/guide/cli.md).

---

## Why teams pick Nitro

`nitro.New()` is not an empty mux. Recover, access log, max body, Request ID,
security headers, OpenTelemetry, Resource cache, and graceful shutdown are **on**.
You write the domain. Nitro handles the boring (and the dangerous).

### Full REST CRUD — no handlers

UUID v7, timestamps, soft delete, validation, list/get/create/patch/delete.
One type. One line.

```go
type Todo struct {
	nitro.Model
	Title string `json:"title" validate:"required,min=2"`
	Done  bool   `json:"done"`
}

func main() {
	app := nitro.New()
	nitro.Resource(app, "/todos", nitro.NewMemoryRepo[Todo]())
	nitro.Ok(app.Listen(":8080"))
}
```

Postgres? Same idea — Gateway + `StoreOf` + `app.Resource`. Filters and sort
come from `blinq:"index"`. See [blinq-hello](examples/v1/blinq-hello/) and
[db-blinq](examples/v1/db-blinq/).

### Bind that cleans the data for you

`Parse` runs **default → `transform:` → validate** on JSON and form.
No `strings.TrimSpace` scattered across handlers.

```go
type Contact struct {
	Name  string `json:"name" validate:"required,min=3" transform:"trim,titlecase=pt"`
	Phone string `json:"phone" validate:"required,min=10" transform:"digits"`
	UF    string `json:"uf" validate:"required,len=2" transform:"trim,upper"`
}

app.Post("/contacts", func(c *nitro.Ctx) error {
	in, err := nitro.Parse[Contact](c)
	if err != nil {
		return err // 422 with field errors — you didn't write that either
	}
	return c.Created(in)
})
```

Need CPF/CNPJ/CEP/UF? Opt-in Brazilian validators — not stuffed into builtins:

```go
v := nitro.NewValidator()
nitro.RegisterBRValidators(v)
app := nitro.New(nitro.Config{Validator: v})
```

```go
CPF string `json:"cpf" validate:"required,cpf" msg_cpf:"Informe um CPF válido"`
```

### Typed Postgres — not `"WHERE name = $1"`

BLinq is the default data layer. Metamodel from `nitrogen make product.go --data`.
Queries compile. Typos don't ship.

```go
db, err := pgxadapter.OpenEnv(ctx) // DATABASE_URL
nitro.Ok(err)
defer db.Close()
nitro.Ok(db.MigrateUp(ctx))

store := pgxadapter.StoreOf[Product](db, "products")
app.Resource(&Product{}, store) // path from TableName() → /products

q := c.Query("q")
items, err := store.Find(c, store.Query().
	WhereIf(c.Query("active") == "true", modelgen.Products.Active.Eq(true)).
	WhereIf(q != "", modelgen.Products.Name.IContains(q)).
	OrderBy(modelgen.Products.Price.OrderDesc()).
	Limit(50))
```

GORM / sqlx / sqlc still work — **through the same Gateway**. BLinq is the
default; the others are when the requirement asks.

### Auth that looks like a product, not a spike

Register, login, refresh, JWT middleware, roles — minutes, not a new service.

```go
auth := nitro.NewAuth(nitro.AuthConfig{
	JWTSecret:      secret,
	AccessTokenTTL: 15 * time.Minute,
}).WithMemoryStore()

app.AuthRoutes("/auth", auth)

api := app.Group("/api", nitro.JWTMiddleware(auth.Provider()))
api.Get("/me", func(c *nitro.Ctx) error {
	return c.OK(nitro.Map{"user_id": c.UserID()})
})

admin := app.Group("/admin",
	nitro.JWTMiddleware(auth.Provider()),
	nitro.RequireRole("admin"),
)
```

Enterprise SSO: `nitro.NewKeycloak` + `KeycloakAuthRoutes` + the same
`RequireRole`. See [auth](examples/v1/auth/) and
[auth-keycloak](examples/v1/auth-keycloak/).

### Config without a homemade `.env` parser

Nested structs, aliases, defaults, validation. `ConfigOf` loads `.env` /
`.env.local` first.

```go
type Config struct {
	Port int    `env:"PORT" default:"8080"`
	DSN  string `env:"DATABASE_URL,POSTGRES_DSN" validate:"required"`
}

cfg, err := nitro.ConfigOf[Config]()
```

### Exports, health, telemetry — first-class

```go
return c.CSV("items.csv", []string{"id", "name"}, rows)
app.HealthCheck(nitro.NamedHealth("postgres", db.Ping))
app.Prometheus() // opt-in scrape
```

OpenTelemetry HTTP **and** DB spans are **on** (`NITRO_OTEL=0` to opt out).
Listen prints the banner and `Listening on http://…` — don't `log.Printf` the port.

### Zero-allocation honesty

`*Ctx` is pooled. `c.Param` / `c.Query` are valid **only inside the handler**.
Copy if you stash them. That's how you stay fast. Details:
[Request](docs/guide/request.md).

---

## Philosophy (the short version)

1. **One import** — `nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"`
2. **Success on the context** — `c.OK` / `c.Created` / `c.CSV` (not `nitro.OK` for 200)
3. **BLinq by default** — `OpenEnv` → Gateway → `StoreOf` / `modelgen`
4. **CLI first** — `nitrogen new` / `install` / `make:scaffold` — don't invent folders
5. **Secure defaults, opt-in power** — rate limit, CORS, CSRF when you need them

Express-level DX. fasthttp-level speed. Clean Architecture packages so adapters stay replaceable.

---

## 👀 Examples

Copy from `examples/v1/*` — don’t invent legacy APIs.

| Example | What it shows |
|---------|----------------|
| [basic](examples/v1/basic/) | Routing, JSON, groups, middleware |
| [cleanarch](examples/v1/cleanarch/) | Clean Architecture (memory) |
| [cleanarch-blinq](examples/v1/cleanarch-blinq/) | CA + BLinq `products` (CLI layout) |
| [validation](examples/v1/validation/) | Validators (incl. BR) |
| [transform](examples/v1/transform/) | `transform:` tags |
| [auth](examples/v1/auth/) | JWT |
| [auth-keycloak](examples/v1/auth-keycloak/) | OIDC / Keycloak |
| [blinq-hello](examples/v1/blinq-hello/) | Migrate + Resource hello |
| [db-blinq](examples/v1/db-blinq/) | Resource + filters / sort |
| [db-blinq-handlers](examples/v1/db-blinq-handlers/) | Handlers + `WhereIf` DSL |
| [resource-dx](examples/v1/resource-dx/) | Hooks, ExtraRoutes, NamedHealth |
| [DB overview](examples/v1/README-db.md) | Compare BLinq / GORM / sqlx / sqlc |

---

## 🤖 Benchmarks

Relative gains from context pooling and allocation cuts (see [benchmarks](docs/extra/benchmarks.md)):

| Scenario | Before | After | Δ |
|----------|--------|-------|---|
| Static GET | 1,697 ns/op | 1,228 ns/op | **27.6%** |
| Route Params | 2,142 ns/op | 1,583 ns/op | **26.1%** |
| Query String | 2,896 ns/op | 2,268 ns/op | **21.7%** |
| JSON Decode | 5,132 ns/op | 3,827 ns/op | **25.4%** |

---

## 🧬 Built-in capabilities

| Area | Highlights |
|------|------------|
| Middleware | CORS, Compress, RateLimit, CSRF, JWT, RequestID, Recover, Logger, security headers |
| Auth adapters | JWT, Session, OAuth2, OIDC/Keycloak, bcrypt |
| Data | `sqladapter.OpenEnv` / `platform.Open`, Gateway, Store, Resource, soft delete, JSONB helpers (Postgres via pgx under the hood) |
| Codegen | BLinq `modelgen`, goose migrations, DTO, scaffold, `nitrogen openapi` |
| Ops | Scheduler (`app.Schedule`), OpenTelemetry, Prometheus, OpenAPI / Swagger |

---

## 📚 Documentation

| Doc | Audience |
|-----|----------|
| [Guide index](docs/guide/README.md) | Humans — HTTP, auth, DB, CLI |
| [Welcome](docs/guide/welcome.md) | Getting started |
| [CLI](docs/guide/cli.md) | `nitrogen` reference |
| [BLinq](docs/guide/blinq.md) · [Data layer](docs/guide/data-layer.md) | Postgres & DSL |
| [API reference](docs/api/README.md) | Package surface |
| [Security policy](SECURITY.md) · [Audit](docs/security/AUDIT.md) | Reporting & hardening |
| [AGENTS.md](AGENTS.md) · [Playbook](docs/ai/PLAYBOOK.en.md) | AI coding agents |

---

## 👍 Contribute

We welcome issues, docs, examples, and PRs.

1. **Star** the repo if Nitro helps you — it improves discoverability.
2. Open a [bug](https://github.com/rodrigocelsius/go-nitro/issues/new) or [discussion](https://github.com/rodrigocelsius/go-nitro/discussions).
3. Read **[CONTRIBUTING.md](CONTRIBUTING.md)** and the [Code of Conduct](CODE_OF_CONDUCT.md) before submitting a PR.
4. Run **`make test`** (this is a multi-module repo; `go test ./...` at the root will not work).
5. Security issues: **do not** open a public issue — see [SECURITY.md](SECURITY.md).

```text
Fork → branch (feat/… | fix/…) → tests → conventional commit → PR
```

Good first areas: examples, docs, middleware tests, BLinq edge cases, CLI UX.

---

## 💻 Development

```bash
git clone https://github.com/rodrigocelsius/go-nitro.git
cd go-nitro

# Package under test (multi-module repo)
go test ./pkg/...
go test ./cmd/nitrogen/...
go test ./codegen/blinq/...

# Example (local replaces in go.mod)
cd examples/v1/basic && go run .
```

Guidelines:

- `gofmt` / `go vet`; comments and commit messages in **English**
- Conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `perf:`
- Follow contracts in [`AGENTS.md`](AGENTS.md) (Parse, `c.OK`, Gateway, BLinq)

---

## 🏗 Architecture

```text
pkg/                 # Public facade (import this)
domain/              # Core types (no deps)
ports/               # Interfaces
usecases/            # Framework logic
adapters/            # fasthttp, auth, cache, otel, …
data/                # Gateway, repository, sql + pgx adapters
blinq/               # Typed query DSL + providers
codegen/             # blinq, dto, migration, openapi, …
cmd/nitrogen/        # CLI
examples/v1/         # Canonical apps to copy
docs/                # Guides, API, security, AI playbook
```

---

## ⭐️ Stargazers

[![Stargazers over time](https://starchart.cc/rodrigocelsius/go-nitro.svg?variant=adaptive)](https://starchart.cc/rodrigocelsius/go-nitro)

---

## 🧾 License

Copyright (c) 2026–present Rodrigo and Contributors.
Released under the [MIT License](LICENSE).
