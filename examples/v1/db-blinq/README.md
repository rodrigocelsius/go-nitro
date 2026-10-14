# db-blinq

CRUD via **Resource** + `StoreOf` (no hand-written handlers).

Shared model/metamodel: [`../productmodel`](../productmodel/).

```bash
# regenerate the BLinq metamodel
cd ../productmodel && go generate ./...

export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
go run .   # applies ./migrations via MigrateUp

curl 'http://localhost:8080/products?active=true&min_price=100&q=key&sort=-price'
curl -X POST http://localhost:8080/products -d '{"name":"Keyboard","price":199.9,"active":true}'
```

Demo without Postgres: `go run -tags memory .`

Handlers + DSL: [`../db-blinq-handlers`](../db-blinq-handlers/). Minimal hello: [`../blinq-hello`](../blinq-hello/).
