# db-gorm

Same `products` HTTP contract as the BLinq examples, using **GORM** on `db.SQL()` through the Nitro Gateway.

See [`../README-db.md`](../README-db.md). Prefer BLinq unless the requirement asks for GORM.

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
go run .
```
