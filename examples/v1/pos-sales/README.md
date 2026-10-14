# pos-sales

Point-of-sale example: a `Sale` header with dependent `SaleItem` line items, wired with
**`sqladapter.Related`** — no `ExtraRoutes`, no hand-written Commands for CRUD, no
`StoreOf` calls in the app.

Demonstrates:

- **Atomic checkout** — `POST /sales` creates the `Sale` and all of its `SaleItem` rows in
  one `db.Tx`; if any item fails to persist, the whole sale rolls back.
- **`RecalcFunc`** — `recalcSale` computes each item's `Subtotal` and sums `Sale.Total`,
  running inside the same transaction on every write that touches items (checkout, add
  item, update item). See [`usecase.go`](usecase.go).
- **Adding an item to an existing sale** — `POST /sales/:id/sale_items`.
- **Updating an item** — `PATCH /sales/:id/sale_items/:childID` re-runs `recalcSale`, so
  `Sale.Total` is always consistent with its items.
- A **Command** (`void`) and a **Query** (`today`) alongside `Related`, using
  `sqladapter.RepositoryFor[Sale]` for the typed repository.

## Layout

```text
models.go     # Sale (+ nested Items []SaleItem) / SaleItem
usecase.go    # recalcSale (RecalcFunc), NotVoided (Rule), Void (Command), Today (Query)
health.go     # GET /health
routes.go     # Routes(app, db) — sqladapter.Related + Rules/Commands/Queries
main.go       # OpenEnv + MigrateUp + Routes
migrations/   # goose SQL (sales, sale_items)
```

## Run

```bash
cd examples/v1/pos-sales
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
go run .
```

## Flow (curl)

```bash
# Checkout — creates the sale header + items, one transaction
ID=$(curl -s -X POST localhost:8080/sales \
  -H 'Content-Type: application/json' \
  -d '{"customer_name":"Ada","items":[{"sku":"COFFEE","quantity":2,"unit_price":8.5},{"sku":"CROISSANT","quantity":1,"unit_price":6}]}' \
  | jq -r .id)

# Sale header — total already computed (23)
curl -s localhost:8080/sales/$ID | jq .

# List a sale's items
curl -s localhost:8080/sales/$ID/sale_items | jq .

# Add another item to the same sale — total recalculates
curl -s -X POST localhost:8080/sales/$ID/sale_items \
  -H 'Content-Type: application/json' -d '{"sku":"TEA","quantity":1,"unit_price":5}'

# Update an item's quantity — total recalculates again
ITEM_ID=$(curl -s localhost:8080/sales/$ID/sale_items | jq -r '.[0].id')
curl -s -X PATCH localhost:8080/sales/$ID/sale_items/$ITEM_ID -d '{"quantity":4}'

# Void the sale — further Commands are rejected (Rule)
curl -s -X POST localhost:8080/sales/$ID/void

# Today's sales
curl -s localhost:8080/sales/queries/today | jq .
```

## Tests

Tests run against an isolated in-memory SQLite DB (`sqladapter.OpenTest` — no Docker),
exercising the real `Related`/`Tx` code path used in production:

```bash
go test ./...
```

See [`docs/guide/business-resource.md`](../../../docs/guide/business-resource.md) for the
full guide on `sqladapter.Related`, `RepositoryFor`, `RecalcFunc`, `SumBy`/`CountBy`.
