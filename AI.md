# AI.md — quick guide for coding agents

The contract is [`AGENTS.md`](AGENTS.md); recipes are in [`docs/ai/PLAYBOOK.en.md`](docs/ai/PLAYBOOK.en.md). This page is only the checklist.

## Defaults

- Generic "ORM / repository / data layer" → **BLinq** (`platform.Open` + `StoreOf` + `modelgen`). Another stack (GORM, sqlx, sqlc) is fine when the user names it.
- CRUD → `app.Resource` + `routes.go`; keep `main`/handlers/repos thin.
- Scaffold with the CLI: `nitrogen new` → `install` → `make:scaffold … --resource`.
- Auth: `SetDefaultResourceMiddleware(JWT…)`; never blanket `Public: true`.
- Multi-tenant: a `tenant_id` column auto-scopes the Resource (`make:scaffold … --tenant`); tenant comes from the JWT claim. `NoTenantScope` opts out.
- Conventions (do not configure): env via `NITRO_ENV`, plural paths, snake_case, trusted proxies, `app.CORS().ETag().Compress()` — see [`docs/guide/conventions.md`](docs/guide/conventions.md).
- Never keep `*nitro.Ctx` after the handler returns; use `c.Detach()` for goroutines.

## Before finishing

- [ ] `nitrogen doctor` and `go test ./...` pass
- [ ] Success responses use `c.OK` / `Parse`; errors use sentinels
- [ ] Migrations via `migrate:make`
- [ ] No hand-made folder trees the CLI already generates

Examples: `examples/v1/cli-shop` (CLI output), `examples/v1/conventions`, `examples/v1/cleanarch-blinq`, `blinq-hello`, `auth`, `resource-tenant-scoped`. CLI: [`docs/guide/cli.md`](docs/guide/cli.md).
