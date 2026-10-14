# Clean Architecture layout + BLinq Resource — `products`

CA folders from `nitrogen new`, SQL via `sqladapter.OpenEnv`, CRUD via **`app.Resource`**
(not five hand-written handlers). Prefer this shape for plain CRUD; add usecases only when
you have non-CRUD domain rules.

```bash
nitrogen new cleanarch-blinq --module github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2 --nitro-path /path/to/nitro
cd cleanarch-blinq
nitrogen install postgresql
nitrogen make:scaffold Product name:string price:float64 active:bool --resource
nitrogen make:seeder Product   # optional
```

```text
cmd/server/main.go                 # OpenEnv + MigrateUp + StoreOf + Register + Listen
cmd/seed/main.go                   # optional seeder
internal/
  config/
  model/product.go                 # //nitro:table=products
  modelgen/product_gen.go          # BLinq DSL
  handler/routes.go                # health + app.Resource
  handler/health.go
  repository/.gitkeep              # unused for plain Resource CRUD
  usecase/.gitkeep                 # unused for plain Resource CRUD
  seed/product_seeder.go           # optional
migrations/*_create_products.sql
```

## Run (Postgres)

```bash
docker compose up -d
export NITRO_DB_DRIVER=postgresql
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/nitro?sslmode=disable'
go run ./cmd/server
# optional: go run ./cmd/seed
```

Requires **Go 1.25+** (pgx v5.10 toolchain).

```bash
curl -s -X POST localhost:8080/products -H 'Content-Type: application/json' \
  -d '{"name":"Keyboard","price":199.9,"active":true}'
curl -s 'localhost:8080/products?active=true&q=key&sort=-price'
curl -s localhost:8080/health
```

List filters use Resource `Filterable` / `FilterRules` (`?q=` → `name` icontains) — no SQL strings.

Tests (SQLite via `OpenTestMigrate`, no Docker):

```bash
go test ./...
```

The Nitro banner prints the port and `Listening on http://…` automatically.
