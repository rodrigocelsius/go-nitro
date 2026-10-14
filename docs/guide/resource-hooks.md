# Resource lifecycle hooks

`ResourceConfig` hooks run around generated CRUD handlers.

| Hook | When |
|------|------|
| `BeforeWrite` | Create **and** update — after `readonly`/`computed` reset + defaults, before verb-specific hooks |
| `BeforeCreate` | After JSON bind / defaults / BeforeWrite, before `repo.Create` |
| `AfterCreate` | After successful `repo.Create` |
| `BeforeUpdate` | After bind/merge / defaults / BeforeWrite, before `repo.Update` |
| `AfterUpdate` | After successful `repo.Update` |
| `AfterDelete` | After successful `repo.Delete` |
| `OnMutate` | After create/update/delete (`action` = `"create"` \| `"update"` \| `"delete"`) |

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
- Return an error to fail the HTTP request (create is not rolled back automatically — use `db.Tx` in `BeforeCreate`/`AfterCreate` or a Command for atomic multi-table writes).
- For Postgres, prefer `sqladapter.DB.Tx` when order + invoice must commit together.

## Example

Runnable demo: [`examples/v1/resource-aftercreate`](../../examples/v1/resource-aftercreate/).

For approve/place/cancel/enrich flows use [business-resource.md](business-resource.md) (`Commands`).
Security defaults: [resource-security.md](resource-security.md).
