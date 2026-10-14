# CLAUDE.md — habits in this app

1. Read [`AGENTS.md`](AGENTS.md) — **DEFAULT DATA ACCESS: if no ORM is named, use BLinq** + **`app.Resource` + `routes.go`**.
2. Quick checklist + reminder: [`AI.md`](AI.md).
3. New systems/resources: **CLI first** — `install` / `make:scaffold … --resource` / `make … --data`.
4. With auth: **`SetDefaultResourceMiddleware(JWT…)`**; never blanket `Public: true`. Denorm via `computed` + `BeforeWrite`; actions via `Commands`.

User-facing chat: English. Code, identifiers, and commits: English.

With SQL: **BLinq** + `platform.Open` / `sqladapter.OpenEnv` + Gateway.  
Default CRUD shape: **`app.Resource`**, routes in `internal/handler/routes.go`, **minimal** `main` / handler / repository / usecase.  
Full CA usecases only when the requirement has non-CRUD domain rules.  
Do **not** pick GORM/sqlx/sqlc/raw SQL unless the user explicitly asks to skip BLinq.
