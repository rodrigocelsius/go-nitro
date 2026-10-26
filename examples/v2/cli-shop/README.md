# cli-shop

Nitro v2 app generated **100% by the CLI**, then extended with one business use case:

```bash
nitrogen new cli-shop --module github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2
nitrogen install sqlite
nitrogen make:scaffold Product name:string price:float64 stock:int     # Clean Architecture slice
nitrogen make:scaffold Category name:string --resource                 # plain CRUD → app.Resource
nitrogen make:usecase Product AdjustStock                              # then implemented by hand
```

Layers: `internal/model` → `internal/usecase/product` (ports: `Repository`, `UnitOfWork`) →
`internal/repository` (BLinq) → `internal/handler` (HTTP) → `cmd/server` (wiring).

`AdjustStock` reads, checks and writes the product **in one transaction** through the
`UnitOfWork` port (`*sqladapter.DB` satisfies it), so the use case never imports infrastructure:
`POST /products/:id/stock {"delta": -3}` → `409` when stock would go negative, nothing written.

```bash
cd examples/v2/cli-shop
go test ./...
go run ./cmd/server
```
