# db-linq

BLinq Resource example with a **legacy folder name**. Same idea as [`../db-blinq`](../db-blinq/): `StoreOf` + Gateway + `app.Resource`.

Shared model: [`../productmodel`](../productmodel/).

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
go run .
```

Prefer [`../db-blinq`](../db-blinq/) for new copies. This folder also exposes `/products/search`.
