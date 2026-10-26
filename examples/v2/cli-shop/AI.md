# AI.md — anti-patterns (this app)

## DEFAULT DATA ACCESS (STOP)

**Unspecified ORM ⇒ BLinq for every system.**  
Use `sqladapter.OpenEnv` / `platform.Open` + `StoreOf` + `modelgen` + `WhereIf`.  
Refuse GORM / sqlx / sqlc / raw SQL unless the user explicitly says `use GORM` / `skip BLinq` / etc.

**Shape:** `app.Resource` + `handler/routes.go` · minimal `main` / handler / repository / usecase (no CA CRUD for plain CRUD).

### Paste this as a system reminder

1. If no ORM is named → **BLinq only**.
2. Open DB with `platform.Open` / `sqladapter.OpenEnv` (Gateway).
3. CRUD → **`app.Resource`** + `StoreOf` / `modelgen`; routes only in `routes.go`.
4. Keep `main`, handlers, repos, and usecases **thin** — no five hand-rolled CRUD handlers.
5. Generate DSL: `nitrogen make <model>.go --data`; scaffold with `--resource` unless real domain rules need CA.
6. After auth: **`SetDefaultResourceMiddleware(JWT…)`** — never blanket `Public: true`.
7. Denorm: **`computed:"true"`** + **`BeforeWrite`** / **`FillStringFrom`**.
8. Actions: Resource **`Commands`**; aggregations: **`CountAll` / `CountBuckets`**.
9. Multi-tenant: a **`tenant_id`** column (`TenantModel`, or `make:scaffold … --tenant`) auto-scopes the Resource; tenant comes from the JWT claim. No `TenantScope`.
10. Never hand-configure conventions: env via `NITRO_ENV` (`nitro.ParseEnv(cfg.Env)`), plural paths, snake_case, trusted proxies; use `app.CORS().ETag().Compress()`.

| Wrong | Right |
|-------|--------|
| Hand-made folders without the CLI | `nitrogen make:scaffold … --resource` |
| Fat `main` with CRUD routes | `handler/routes.go` + `Register` + `app.Resource` |
| Five CRUD handlers + empty usecases | `app.Resource` + FilterRules / hooks |
| Pre-creating empty `seed`/`factory`/`middleware`/`job` | Only via `make:*` when needed |
| `return nitro.OK(x)` for success | `return c.OK(x)` |
| `gorm.Open` / sqlx / sqlc | `platform.Open` + **BLinq** `StoreOf` |
| `db.Query("SELECT …")` in repository | `modelgen` / Resource filters |
| `Public: true` on all Resources | `SetDefaultResourceMiddleware(JWT…)` |
| `app.Post("/x/:id/enrich")` ad-hoc | `Commands: nitro.Commands(nitro.Cmd("enrich", …))` |
| FilterRules alone for tenant isolation | `tenant_id` column (auto-scope) + JWT tenant claim |
| `Config{Env: nitro.EnvDevelopment}` hard-coded | `nitro.ParseEnv(cfg.Env)` / `NITRO_ENV` |
| `nitro.CORS()` / `nitro.Compress()` (removed) | `app.CORS()` / `app.Compress()` |
| `log.Printf("listening…")` in main | Automatic `Listen` banner |

Checklist: CLI + `--resource` · **BLinq** · `routes.go` · JWT default middleware · thin layers · `go test` · `nitrogen doctor`.
