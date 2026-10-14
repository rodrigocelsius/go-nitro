# Clean Architecture — generated with Nitrogen 1.4

```bash
nitrogen new cleanarch --module github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2
cd cleanarch
nitrogen make:scaffold Product name:string price:float64
```

Layout:

```text
cmd/server/main.go
internal/
  config/
  model/product.go
  usecase/product/{create,get,list,update,delete,discount}.go
  handler/{routes,product,health}.go
  repository/product_memory.go
```

## Run / test

```bash
go run ./cmd/server
go test ./...
```

The Nitro banner prints the port and `Listening on http://…` automatically.
On shutdown: `Nitro: shutting down…` / `Nitro: server stopped`.
