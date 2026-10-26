# Business Resource (Commands, Queries, Rules)

Nitro **2.0** extends `ResourceConfig` with business operations while keeping `app.Resource` CRUD unchanged.

## When to use what

| Need | Use |
|------|-----|
| Standard create/update/delete | CRUD (`app.Resource`) |
| Always create a related row on POST | `AfterCreate` hook — see [resource-hooks.md](resource-hooks.md) and `examples/v1/resource-aftercreate` |
| Named business action (approve, place, cancel) | `Commands` |
| Named read model (pending, export view) | `Queries` |
| Block commands by entity state | `Rules` |
| Multi-table write (sales + finance) | `db.Tx` inside the usecase — see `examples/v1/sql-tx` |
| Header + dependent line items (sale + items, order + lines) | `sqladapter.Related` — see below and `examples/v1/pos-sales` |

## ResourceConfig

```go
repo := repository.NewOrderRepository(store) // *sqladapter.Store → order.Repository

app.Resource(&model.Order{}, store, &nitro.ResourceConfig{
    Filterable: []string{"status"},
    Middleware: []nitro.Handler{nitro.JWTMiddleware(auth.Provider())},

    Rules: nitro.Rules(order.NotCancelled),

    Commands: nitro.Commands(
        nitro.Cmd("place", order.Place(db)),              // POST /orders/place (orders+invoices, Tx)
        nitro.Cmd("submit", order.Submit(repo)),          // POST /orders/:id/submit
        nitro.Cmd("approve", order.Approve(db)).
            When(order.HighValue).
            Roles("manager"),                               // POST /orders/:id/approve (orders+invoices, Tx)
        nitro.Cmd("cancel", order.Cancel(repo)),
    ),

    Queries: nitro.Queries(
        nitro.Q("pending", order.Pending(repo)),          // GET /orders/queries/pending
    ),
})
```

Each usecase function takes its dependency (`db` for multi-table Commands, `repo` for single-table ones) and returns the closure the framework calls — no service struct, no extra wiring.

## Ownership (`OwnerCheck`)

`app.Resource`'s default CRUD does **not** scope `Get`/`Update`/`Delete` by
the caller — any authenticated request that knows (or enumerates) an `:id`
can read, change, or remove *any* row, regardless of who created it (IDOR).
`Middleware`/`JWTMiddleware` only proves *who* is calling, not that the
resource is *theirs*.

Set `OwnerCheck` to close that gap. It runs right after the entity is
loaded by `:id` and before it is returned/mutated/removed; returning
`false` responds `404` (not `403`), so a caller probing other IDs can't
tell "not yours" from "does not exist":

```go
app.Resource(&model.Order{}, store, &nitro.ResourceConfig{
    Middleware: []nitro.Handler{nitro.JWTMiddleware(auth.Provider())},
    OwnerCheck: func(c *domain.Ctx, entity any) bool {
        return entity.(*model.Order).UserID == c.UserID()
    },
})
```

`List` is not covered by `OwnerCheck` (it never loads a single entity by
id) — scope it with `Filterable`/`FilterRules` plus a middleware that
injects the caller's id as a fixed query filter, or override the route via
`ResourceConfig.Replace`.

## Command signatures

The framework infers route shape and JSON body from the function:

| Signature | Route | Body |
|-----------|-------|------|
| `func(ctx, in Input) (T, error)` | `POST /{name}` | yes → `201 Created` |
| `func(ctx, id string) (T, error)` | `POST /:id/{name}` | no |
| `func(ctx, id string, in Input) (T, error)` | `POST /:id/{name}` | yes |

Put logic in `internal/usecase/order` (one file per operation, e.g. `place.go`, `submit.go`). Each function is bound to its dependency at registration time, e.g. `order.Submit(repo)`.

## Rules and Roles

- **`Rules`**: `func(*T) error` — run after load by `:id`, before the command handler.
- **`.When(fn).Roles("manager")`**: when `fn(entity)` is true, JWT must include `manager` in `claims.Roles`. Requires `nitro.JWTMiddleware(auth.Provider())` on `ResourceConfig.Middleware` (or a parent `Group`) so `claims.Roles` reaches the request context.

## Queries

```go
func Pending(repo Repository) func(ctx context.Context, page nitro.Page) ([]model.Order, error)
```

`nitro.Page` comes from `?page=` and `?per_page=` (defaults 1 and 20).

## Multi-table writes

With a BLinq repository every Command runs in **one transaction** by
convention (`ResourceConfig.NoTx` opts out): every Store call made with the
Command's `ctx` joins it, and an error rolls all of them back. The response is
written only after `COMMIT`.

```go
func Place(orders *sqladapter.Store[model.Order], invoices *sqladapter.Store[model.Invoice]) func(ctx context.Context, in PlaceInput) (*model.Order, error) {
    return func(ctx context.Context, in PlaceInput) (*model.Order, error) {
        o, err := orders.CreateOne(ctx, &model.Order{CustomerID: in.CustomerID, Total: in.Total})
        if err != nil {
            return nil, err
        }
        if _, err := invoices.CreateOne(ctx, &model.Invoice{OrderID: o.ID, Amount: o.Total}); err != nil {
            return nil, err // order insert rolled back too
        }
        nitro.AfterCommit(ctx, func(ctx context.Context) { notify(o) }) // side effect after COMMIT
        return o, nil
    }
}
```

Outside a Resource (use cases, jobs) use `db.InTx(ctx, fn)` / `nitro.InTx(ctx, db, fn)`.
Details: [transactions.md](transactions.md).

## Header + line items (`sqladapter.Related`)

A header/items relationship (`Sale`/`SaleItem`, `Order`/`OrderLine`, …) is common enough
to not need `ExtraRoutes` or a hand-written Command. `sqladapter.Related` registers the
parent as a normal `app.Resource` **and** wires the child table by convention:

```go
type Sale struct {
    nitro.Model
    CustomerName string     `json:"customer_name" db:"customer_name"`
    Total        float64    `json:"total" db:"total"`
    Items        []SaleItem `json:"items" db:"-"` // nested input for POST /sales
}

type SaleItem struct {
    nitro.Model
    SaleID    nitro.UUID `json:"sale_id" db:"sale_id"` // FK: snake_case(Sale)+"_id"
    SKU       string     `json:"sku" db:"sku"`
    Quantity  int        `json:"quantity" db:"quantity"`
    UnitPrice float64    `json:"unit_price" db:"unit_price"`
    Subtotal  float64    `json:"subtotal" db:"subtotal"`
}

func recalcSale(sale *Sale, items []SaleItem) {
    sale.Total = nitro.SumBy(items, func(item *SaleItem) float64 {
        item.Subtotal = float64(item.Quantity) * item.UnitPrice
        return item.Subtotal
    })
}

func Routes(app *nitro.App, db *sqladapter.DB) {
    repo := sqladapter.RepositoryFor[Sale](db) // typed CRUD repo, table by convention

    sqladapter.Related(app, db, recalcSale, &nitro.ResourceConfig{
        NoCache:  true,
        Rules:    nitro.Rules(NotVoided),
        Commands: nitro.Commands(nitro.Cmd("void", Void(repo))),
    })
}
```

No table names, no `StoreOf`, no `ExtraRoutes` are written by the app — `Related` derives
everything from the two struct types:

- **Table**: same convention as `app.Resource` (`TableName()`, else plural snake of the type).
- **FK column** on the child: `snake_case(ParentTypeName) + "_id"` (`Sale` → `sale_id`) — the
  child struct must declare a field with that `db` tag.
- **Nested field**: the parent struct must declare exactly one field of type `[]Child`
  (any name, `db:"-"` so it is never persisted directly) — that's the body `Related` binds
  on `POST /{parent}`.
- **Child route segment**: same table-name convention as the child, e.g. `sale_items`.

`Related` registers:

| Route | What |
|-------|------|
| `POST /sales` | Creates `Sale` + `[]SaleItem`, one `db.Tx`, then `recalc` |
| `GET /sales/:id` , `GET /sales`, `PATCH /sales/:id`, `DELETE /sales/:id` | Normal `app.Resource` CRUD on `Sale` (unchanged) |
| `GET /sales/:id/sale_items` | Lists that sale's items |
| `POST /sales/:id/sale_items` | Adds one item to an existing sale, one `db.Tx`, then `recalc` |
| `PATCH /sales/:id/sale_items/:childID` | Updates one item (PATCH-merge), then `recalc`, one `db.Tx` |

### `RecalcFunc[T, C]`

```go
type RecalcFunc[T any, C any] func(parent *T, children []C)
```

A plain function, not a Command — it never gets its own HTTP route. `Related` calls it
with the **full** parent and the **full** current set of children, inside the same
transaction as the write, every time children change (checkout, add item, update item).
Because it receives `*T` and `[]C` (not a single field), it can update as many parent
fields as needed in one place (`Total`, `ItemCount`, `UpdatedBy`, …), and it may also
mutate each child (e.g. compute `Subtotal`) before the child rows are persisted.

### `sqladapter.RepositoryFor[T]`

```go
func RepositoryFor[T any](db *sqladapter.DB) nitro.TypedRepository[T]
```

The typed-repository equivalent of a `StoreOf` + `pgxadapter.NewStoreRepo` pair, with the
table derived by the same convention as `Related`/`app.Resource`. Use it to get a
`nitro.TypedRepository[T]` for Commands/Queries without repeating the table name:

```go
repo := sqladapter.RepositoryFor[Sale](db) // == StoreOf[Sale](db, "sales") + NewStoreRepo
```

### `nitro.SumBy` / `nitro.CountBy`

Small generic helpers meant to replace a manual `for` loop inside a `RecalcFunc`:

```go
sale.Total = nitro.SumBy(items, func(item *SaleItem) float64 {
    item.Subtotal = item.Quantity * item.UnitPrice // may also mutate the item
    return item.Subtotal
})
sale.ActiveItems = nitro.CountBy(items, func(item SaleItem) bool { return !item.Voided })
```

Full example: [`examples/v1/pos-sales`](../../examples/v1/pos-sales/).

## Example

Full app: [`examples/v1/order-business`](../../examples/v1/order-business/).

AfterCreate only: [`examples/v1/resource-aftercreate`](../../examples/v1/resource-aftercreate/).

Header + line items: [`examples/v1/pos-sales`](../../examples/v1/pos-sales/).
