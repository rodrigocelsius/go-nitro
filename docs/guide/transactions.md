# Transactions & write hooks

**Language:** [English](transactions.md) · [Português](../pt/guide/transactions.md)

Nitro transactions are **ambient**: the transaction travels in `context.Context`.
Any BLinq `Store`, `RepositoryFor` repository or `app.Resource` called with that
`ctx` joins it. No tx handle to pass around, no second set of repositories.

| Need | API |
|------|-----|
| Several writes, all-or-nothing | `db.InTx(ctx, func(ctx) error)` · `store.InTx` · `nitro.InTx(ctx, db, fn)` |
| Retry on deadlock / serialization failure | `db.InTxRetry(ctx, 3, fn)` · `sqladapter.WithRetry` |
| Side effect only if the data was saved (queue, e-mail, webhook) | `nitro.AfterCommit(ctx, fn)` · `ResourceConfig.AfterCommit` |
| Compensation / alert on rollback | `nitro.AfterRollback(ctx, fn)` |
| Validate / prepare **before** a Resource write | `BeforeWrite`, `BeforeCreate`, `BeforeUpdate`, `BeforeDelete` |
| Write related rows **after** a Resource write | `AfterCreate`, `AfterUpdate`, `AfterDelete`, `OnMutate` |
| Business action with several steps | `ResourceConfig.Commands` (`nitro.Cmd`) |
| Header + items (sale + sale items) with nested body | `sqladapter.Related` |
| Is this code inside a transaction? | `nitro.InTransaction(ctx)` |

## Convention: Resource writes are atomic

When the repository supports transactions (every BLinq `Store` / `StoreFor` /
`RepositoryFor`) **and** the Resource declares a write hook (`BeforeWrite`,
`Before*`, `After*`, `OnMutate`) or `Commands`, each write runs as **one
transaction**:

```
BEGIN
  load (PATCH/DELETE)  → BeforeWrite → BeforeCreate|BeforeUpdate|BeforeDelete
  INSERT|UPDATE|DELETE
  AfterCreate|AfterUpdate|AfterDelete → OnMutate
COMMIT  → AfterCommit (outside the tx) → HTTP response
```

Any hook returning an error rolls everything back and the client receives that
error (e.g. `nitro.Conflict(...)` → 409). Transient database conflicts
(deadlock / serialization failure) answer **409 `{"code":"tx_conflict","retryable":true}`**
so the client can retry. Resources without hooks keep the single-statement path
(no extra `BEGIN`/`COMMIT`). Opt out with `ResourceConfig{NoTx: true}`.

### Example 1 — sale + sale details

Inserting a sale must insert its detail rows with the sale id; any error rolls
both back.

```go
details := sqladapter.StoreFor[model.SaleDetail](db)

app.Resource(&model.Sale{}, sqladapter.StoreFor[model.Sale](db), &nitro.ResourceConfig{
    AfterCreate: func(ctx context.Context, e any) error {
        sale := e.(*model.Sale)
        for i := range sale.Items {            // Items []SaleDetail `db:"-"`
            sale.Items[i].SaleID = sale.ID
        }
        _, err := details.CreateMany(ctx, sale.Items) // same tx (ctx)
        return err
    },
    AfterCommit: func(ctx context.Context, action string, e any) {
        _ = nitro.Publish(ctx, queue, "sale.created", e) // only after COMMIT
    },
})
```

For the common header + items shape, `sqladapter.Related` does this (plus
`GET/POST/PATCH /sales/:id/sale_details` and a recalculation hook) with no code:
see [business-resource.md](business-resource.md).

### Example 2 — paid bill clears the customer

```go
customers := sqladapter.StoreFor[model.Customer](db)

app.Resource(&model.Payable{}, sqladapter.StoreFor[model.Payable](db), &nitro.ResourceConfig{
    AfterCreate: func(ctx context.Context, e any) error {
        p := e.(*model.Payable)
        if !p.Paid {                       // conditional action
            return nil
        }
        _, err := customers.Update(ctx, p.CustomerID, modelgen.Customers.Status.Set("clear"))
        return err                         // error → payable insert rolled back
    },
})
```

### Before-checks and conditional branches

Hooks are plain Go, so conditions are `if`/`switch`:

```go
BeforeDelete: func(ctx context.Context, id any) error {
    open, err := payables.CountWhere(ctx, modelgen.Payables.CustomerID.Eq(id), modelgen.Payables.Paid.Eq(false))
    if err != nil { return err }
    if open > 0 { return nitro.Conflict("customer has open payables") }
    return nil
},
BeforeWrite: func(ctx context.Context, e any) error {
    o := e.(*model.Order)
    switch {
    case o.Total > 10_000:
        o.Status = "review"            // one action…
    default:
        o.Status = "approved"          // …or another
    }
    return nil
},
```

## Use cases and services (outside Resource)

```go
err := db.InTx(ctx, func(ctx context.Context) error {
    sale, err := sales.CreateOne(ctx, &model.Sale{CustomerID: cid})
    if err != nil {
        return err
    }
    if _, err := stock.UpdateWhere(ctx, stock.Query().Where(modelgen.Stock.SKU.Eq(sku)),
        modelgen.Stock.Qty.Inc(-1)); err != nil {
        return err
    }
    nitro.AfterCommit(ctx, func(ctx context.Context) { notify(sale) })
    return nil
})
```

- **Nested** `InTx` (or a Resource hook calling a use case that opens `InTx`) on the same database **joins** the outer transaction; the outermost one commits.
- Stores of **another** database are not captured by the transaction (each `DB` has its own).
- `db.Tx(ctx, func(tx *sqladapter.DB) error)` still works: it hands a tx-bound `*DB` so `StoreOf(tx, …)` works with any `ctx`.
- Retry the whole unit of work with `db.InTxRetry(ctx, attempts, fn)`; keep side effects in `AfterCommit` so a retry does not repeat them.
- Row locks inside the tx: `query.ForUpdate()` / `SkipLocked()` (see [blinq.md](blinq.md)).
- The SELECT cache is bypassed inside a transaction and invalidated again after `COMMIT`.

## Outside a transaction

`AfterCommit` called without a transaction runs immediately, and
`AfterRollback` is a no-op — the same hook code works with or without `NoTx`
and with in-memory repositories (which do not support transactions).
