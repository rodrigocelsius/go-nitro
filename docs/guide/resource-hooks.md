# Resource lifecycle hooks

`ResourceConfig` hooks run around generated CRUD handlers.

| Hook | When |
|------|------|
| `BeforeWrite` | Create **and** update — after `readonly`/`computed` reset + defaults, before verb-specific hooks |
| `BeforeCreate` | After JSON bind / defaults / BeforeWrite, before `repo.Create` |
| `AfterCreate` | After successful `repo.Create` |
| `BeforeUpdate` | After bind/merge / defaults / BeforeWrite, before `repo.Update` |
| `AfterUpdate` | After successful `repo.Update` |
| `BeforeDelete` | After the access check, before `repo.Delete` (veto / cascade) |
| `AfterDelete` | After successful `repo.Delete` |
| `OnMutate` | After create/update/delete (`action` = `"create"` \| `"update"` \| `"delete"`) |
| `AfterCommit` | After the transaction committed, outside it (queue, e-mail); cannot fail the request |

**Atomic by convention:** with a BLinq repository, a Resource that declares any
write hook runs load + `Before*` + SQL + `After*` + `OnMutate` in **one
transaction** — a hook error rolls the write back, and every Store call made
with the hook's `ctx` joins it. `NoTx: true` opts out. Details:
[transactions.md](transactions.md).

Prefer **`BeforeWrite`** when the same normalize/denorm logic applies to both create and update.

## Denorm / computed fields

Tag derived columns so clients cannot mass-assign them:

```go
type Album struct {
    nitro.Model
    ArtistID   *nitro.UUID `json:"artist_id" db:"artist_id"`
    ArtistName string      `json:"artist_name" db:"artist_name" computed:"true"`
}
```

`computed:"true"` is treated like `readonly:"true"` (cleared on create, restored on PATCH merge). Fill in `BeforeWrite`:

```go
BeforeWrite: func(ctx context.Context, entity any) error {
    a := entity.(*model.Album)
    return nitro.FillStringFrom(ctx, stores.Artists, a.ArtistID, &a.ArtistName, func(ar *model.Artist) string {
        return ar.Name
    })
},
```

## AfterCreate — related rows on create

Use when every `POST /resource` must also insert into another table (audit, finance, inventory) and a separate **Command** is not needed.

```go
app.Resource(&model.Order{}, store, &nitro.ResourceConfig{
    AfterCreate: func(ctx context.Context, entity any) error {
        o := entity.(*model.Order)
        return invoices.Create(ctx, &model.Invoice{
            OrderID: o.ID,
            Amount:  o.Total,
            Status:  "open",
        })
    },
})
```

**Notes:**

- `entity` is the same pointer passed to `repo.Create` (with ID/timestamps already set).
- Return an error to fail the HTTP request: with a BLinq repository the order **and** the invoice are rolled back (same transaction, through `ctx`).
- Use the hook's `ctx` for every Store call — that is what joins the transaction.
- Side effects that must only happen after the data is saved go in `AfterCommit`.

## Example

Runnable demo: [`examples/v1/resource-aftercreate`](../../examples/v1/resource-aftercreate/).

For approve/place/cancel/enrich flows use [business-resource.md](business-resource.md) (`Commands`).
Security defaults: [resource-security.md](resource-security.md).
