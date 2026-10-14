# Documentation for AIs

**Language:** [English](README.en.md) · [Português](README.md)

> Start with [`AGENTS.md`](../../AGENTS.md), then [`CLAUDE.md`](../../CLAUDE.md) / [`AI.md`](../../AI.md) and this index + playbook.

## Unspecified ORM ⇒ BLinq (all systems)

**If the user does not name another ORM/stack, BLinq is mandatory** for every Nitro SQL app:

`sqladapter.OpenEnv` / `platform.Open` + `StoreOf` + `modelgen` + `WhereIf`.

| User says | Agent does |
|-----------|------------|
| “CRUD with Postgres”, “API with DB”, “repository”, silence on ORM | **BLinq** + **`app.Resource`** + `routes.go` (minimal layers) |
| “use GORM”, “sqlx”, “sqlc”, “skip BLinq”, “raw SQL” (explicit) | Other stack (still via Gateway when applicable) |

Do **not** treat “use an ORM”, “data layer”, or “repository pattern” as a license for GORM.  
Do **not** invent five CRUD handlers or fat CA layers for plain CRUD — use `make:scaffold … --resource`.  
Open `examples/v1/db-gorm` / `db-sqlx` / `db-sqlc` **only after** explicit opt-out.

### Pin the contract in consumer apps

```bash
nitrogen ai:init --force   # refresh AGENTS.md, AI.md, nitro.mdc, nitro-blinq.md
```

Paste [`.cursor/prompts/nitro-blinq.md`](../../cmd/nitrogen/templates/ai/nitro-blinq.md.tmpl) (or the 5 bullets in `AI.md`) as the first agent reminder.  
`nitrogen doctor` warns if GORM/sqlx/sqlc appear without an opt-out doc, or if SQL is installed without `modelgen` / `StoreOf`.

## Reading order

1. [`AGENTS.md`](../../AGENTS.md) — contract + **CLI first** + **BLinq default**
2. [`CLAUDE.md`](../../CLAUDE.md) · [`AI.md`](../../AI.md) — habits and checklist
3. [`PLAYBOOK.md`](PLAYBOOK.en.md) — recipes for a full system
4. [`../guide/cli.md`](../guide/cli.md) — nitrogen reference
5. BLinq guide: [`../guide/blinq.md`](../guide/blinq.md)
6. Examples in `examples/v1/*` — copy, do not invent (`cleanarch-blinq` / `db-blinq` first)

## Creating a system (summary)

```bash
nitrogen new app --module github.com/acme/app
nitrogen install postgresql
nitrogen make:scaffold Product name:string price:float64 --resource
# SQL scaffold already emits BLinq; regenerate if needed:
nitrogen make internal/model/product.go --data
nitrogen doctor && go test ./...
```

`internal/{seed,factory,middleware,job}` do **not** come from `nitrogen new` — only via `make:seeder|factory|middleware|job` if the system needs them.

## Quick tree

| Need | Go to |
|---------|---------|
| CA + BLinq (only when non-CRUD usecases needed) | `examples/v1/cleanarch-blinq` |
| CA memory only | `examples/v1/cleanarch` |
| Hello DB + Resource (**preferred**) | `examples/v1/blinq-hello` |
| Resource + filters (**preferred**) | `examples/v1/db-blinq` |
| Resource DX | `examples/v1/resource-dx` |
| JWT auth | `examples/v1/auth` |
| Custom validation | `examples/v1/validation` |
| Business Commands/Queries/Rules + multi-table `Tx` | `examples/v1/order-business` |
| Header + line items (sale/order + items, atomic, one call) | `sqladapter.Related` → `examples/v1/pos-sales` |
| GORM/sqlx/sqlc (opt-out only) | `db-gorm` / `db-sqlx` / `db-sqlc` |

## Anti-patterns (refuse)

- Building `internal/` without `nitrogen new` / `make:scaffold`
- Pre-creating empty `seed`/`factory`/`middleware`/`job` in the scaffold
- `nitro.OK` / `nitro.Created` as success → `c.OK` / `c.Created`
- `g resource` / `--stack` / `make:migration` → `make:scaffold` / `install` / `migrate:make`
- GORM / sqlx / sqlc / raw SQL as first choice → **BLinq**
- `data/runtime` / pool without `sqladapter.OpenEnv` / `platform.Open`
- `log.Printf("listening…")` in main → automatic Listen banner
- Manual 5-route CRUD when Resource or CA scaffold already solves it
- Stale consumer AI rules → `nitrogen ai:init --force`

## SQL scaffold (checklist)

```text
[ ] nitrogen new + install postgresql|sqlite|…
[ ] nitrogen make:scaffold …  (emits BLinq + migrate when SQL is installed)
[ ] repository uses StoreOf + modelgen (not GORM)
[ ] OpenEnv → MigrateUp → StoreOf or BLinq repository
[ ] go build / go test OK
[ ] nitrogen doctor (no alternate ORM without opt-out)
```

## Notes

- `migrate:make` generates **goose SQL** (`.sql`).
- BLinq bulk: `CreateMany` + `WithChunk`; `UpdateWhere` / `OmitReturning` — playbook §6 and [`../guide/data-layer.md`](../guide/data-layer.md).
- Model fields: `json`/`db` tags are optional — omitted ones default to snake_case of the field name (independently for JSON and DB); explicit tags still win. See [`../guide/models.md`](../guide/models.md).
- Cursor: `.cursor/rules/nitro-framework.mdc` (`alwaysApply`).
- CLI version: **2.0.0+** (`nitrogen version`).
