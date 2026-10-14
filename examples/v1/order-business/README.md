# order-business

`app.Resource` CRUD plus **Commands** (`place`, `submit`, `approve`, `cancel`) and a **Query** (`pending`) on `/orders`, backed by Postgres + BLinq.

Demonstrates:

- Collection Command `POST /orders/place` — inserts `orders` **and** `invoices` atomically via `sqladapter.DB.Tx` (enterprise-style sales + financial write).
- Item Command `POST /orders/:id/approve` — updates `orders` **and** `invoices` in the same transaction.
- Item Commands `submit` / `cancel` — single-table updates through the `order.Repository` port (`FindByID` → mutate → `Update`).
- Global **Rule** (`NotCancelled`) run before every item Command, and conditional **Roles** (`approve` requires `manager` only when `order.Total > 10000`, via `.When(order.HighValue).Roles("manager")`).
- Business logic split one file per operation in `internal/usecase/order/` (`place.go`, `approve.go`, `submit.go`, `cancel.go`, `rules.go`, `queries.go`).

## Layout

```text
cmd/server/main.go                        # DI: OpenEnv + MigrateUp + auth + Register
internal/model/{order,invoice}.go         # nitro.Model + BLinq tags
internal/usecase/order/
  repository.go   # Repository = nitro.TypedRepository[model.Order]
  place.go        # Place(db)    — collection Command, orders+invoices insert (Tx)
  approve.go      # Approve(db)  — item Command, orders+invoices update (Tx)
  submit.go       # Submit(repo) — item Command, single table
  cancel.go       # Cancel(repo) — item Command, single table
  rules.go        # NotCancelled, HighValue
  queries.go      # Pending(repo)
internal/repository/order_repository.go  # adapts *sqladapter.Store to order.Repository
internal/handler/routes.go                # app.Resource + Rules/Commands/Queries
migrations/                                # goose SQL (orders, invoices)
```

## Run

```bash
cd examples/v1/order-business
docker compose up -d
cp .env.example .env
go run ./cmd/server
```

## Flow (curl)

```bash
# Register (default role: user)
TOKEN=$(curl -s -X POST localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@co.com","password":"12345678"}' | jq -r .access_token)

# Place order → creates a draft order + an open invoice (one transaction)
ID=$(curl -s -X POST localhost:8080/orders/place \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"customer_id":"c1","total":15000}' | jq -r .id)

# Submit → pending
curl -s -X POST localhost:8080/orders/$ID/submit -H "Authorization: Bearer $TOKEN"

# Approve: total > 10000 requires the "manager" role → 403 for a plain user
curl -s -X POST localhost:8080/orders/$ID/approve \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"reason":"VIP customer"}'

# Pending list
curl -s localhost:8080/orders/queries/pending -H "Authorization: Bearer $TOKEN"
```

## Tests

Tests run against an isolated in-memory SQLite DB (`sqladapter.OpenTest` — no Docker), exercising the real `Store`/`Tx` code path used in production:

```bash
go test ./...
```

See [`docs/guide/business-resource.md`](../../../docs/guide/business-resource.md) for the full guide on Commands/Rules/Queries and multi-table writes.
