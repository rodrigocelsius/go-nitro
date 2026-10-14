# crud

In-memory CRUD with `nitro.Resource(app, "/todos", nitro.NewMemoryRepo[Todo]())` (no Postgres).

```bash
go run .
go test .
```
