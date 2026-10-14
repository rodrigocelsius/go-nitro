# resource-aftercreate

Minimal example: **standard CRUD create** (`POST /orders`) plus an **`AfterCreate`** hook that inserts a related **invoice** row.

Use this when the side effect is always tied to create and does not need a separate business command.

Compare with [`order-business`](../order-business/) when you need named commands (`place`, `approve`, …) or rules per action.

## Run

```bash
go run .
curl -X POST localhost:8080/orders -H 'Content-Type: application/json' \
  -d '{"customer_id":"c1","total":99.90}'
curl localhost:8080/invoices/count
```

## Tests

```bash
go test .
```
