# sql-tx

Parent + child insert in one `sqladapter.DB.Tx` — **no Docker** (in-memory SQLite via `OpenTest`).

```bash
cd examples/v1/sql-tx
go test .
go run .
```

Same pattern as the data-layer guide: `StoreOf` inside the callback shares the transaction across engines.
