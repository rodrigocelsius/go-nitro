# db-blinq-handlers

BLinq with **custom routes** (`list` / `get` / `create` / `update` / `remove`) and DSL (`WhereIf`).

Same domain via **Resource** (no handlers): [`../db-blinq`](../db-blinq/).

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
go run .   # MigrateUp on ./migrations

curl 'http://localhost:8080/products?active=true&min_price=100&q=key'
curl -X POST http://localhost:8080/products -d '{"name":"Keyboard","price":199.9,"active":true}'
curl -X POST http://localhost:8080/products/bulk -d '[
  {"name":"A","price":10,"active":true},
  {"name":"B","price":20,"active":true}
]'
```
