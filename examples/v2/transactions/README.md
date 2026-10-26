# Transactions

The two scenarios from the transaction requirements, with BLinq's ambient transactions (`ctx` carries the transaction):

1. `POST /sales` inserts the sale, its items and decrements stock **in one transaction** (`AfterCreate` hook). Insufficient stock → `409`, everything rolled back, the client retries.
2. `POST /payables` with `paid: true` clears the customer status in the same transaction; `DELETE /customers/:id` is vetoed (`BeforeDelete`) while payables are open.

Also: `db.InTxRetry` for a use case outside Resource and `AfterCommit` side effects (never fired on rollback).

Guide: [`docs/guide/transactions.md`](../../../docs/guide/transactions.md).

```bash
cd examples/v2/transactions
go test .
go run .   # :8080, SQLite in ./data unless DATABASE_URL is set
```
