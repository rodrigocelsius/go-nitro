# BLinq reminder (paste into agent system / first message)

1. If no ORM is named → **BLinq only** (`platform.Open` / `sqladapter.OpenEnv` + `StoreOf` + `modelgen` + `WhereIf`).
2. CRUD → **`app.Resource`** + `handler/routes.go` (`Register`) — never wire CRUD in `main`.
3. Keep `main` / handler / repository / usecase **minimal**; no five hand-rolled CRUD handlers or empty CA layers.
4. Scaffold: `nitrogen make:scaffold … --resource` then `make <model>.go --data`. Copy `db-blinq` / `blinq-hello`, not `db-gorm`.
5. Switch stacks **only** on explicit opt-out (`use GORM`, `skip BLinq`). Generic “ORM” / “repository” still means BLinq + Resource.
6. Auth: **`SetDefaultResourceMiddleware(JWT…)`**; denorm via `computed` + `BeforeWrite`; actions via `Commands`; counts via `CountAll`/`CountBuckets`.
7. Multi-tenant: **`TenantModel`** (`tenant_id` ⇒ auto-scoped; tenant from JWT claim).

Pin rules: `nitrogen ai:init --force` · read `AGENTS.md` and `.cursor/rules/nitro.mdc`.
