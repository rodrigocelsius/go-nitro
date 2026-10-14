# Model field tags (`json` / `db`)

**Language:** [English](models.md) · Português: see inline notes in [PLAYBOOK (PT)](../ai/PLAYBOOK.md).

Nitro model fields no longer require explicit `json`/`db` tags. Declare only
what you need to customize; anything you omit falls back to a single,
predictable convention: **snake_case of the Go field name**.

## The rule

For each struct field, the JSON key and the DB column are resolved
**independently**:

| You write | JSON key | DB column |
|-----------|----------|-----------|
| `Message string` | `message` (default) | `message` (default) |
| `Message string `db:"messages"`` | `message` (default) | `messages` (explicit) |
| `Message string `json:"msg"`` | `msg` (explicit) | `message` (default) |
| `Message string `json:"msg" db:"messages"`` | `msg` (explicit) | `messages` (explicit) |
| `Message string `json:"-"`` | excluded from JSON | `message` (default) |
| `Message string `db:"-"`` | `message` (default) | excluded from persistence |

An explicit tag always wins; an omitted tag always falls back to
snake_case of the field name — the `db` default does **not** reuse a custom
`json` name (or vice versa), each namespace resolves on its own.

This is fully backward compatible: every model that already declares both
tags (the style used everywhere in this repo prior to this convention)
behaves exactly as before, byte for byte.

## Compound / acronym names

The snake_case conversion treats a run of consecutive uppercase letters as
a single acronym, not one underscore per letter:

| Go field | Default name |
|----------|--------------|
| `CustomerName` | `customer_name` |
| `UnitPriceUSD` | `unit_price_usd` |
| `SKU` | `sku` |
| `OrderID` | `order_id` |
| `URLPath` | `url_path` |
| `HTTPStatus` | `http_status` |

The same algorithm is used everywhere in the framework that needs to derive
a default name from a Go identifier: DB columns (`data/adapter/pgx`,
`blinq/scan`), JSON keys (`usecases/jsontag`), and the BLinq metamodel
generator (`nitrogen make <model>.go --data`). See
`usecases/transform.SnakeCase` (exported as `nitro.SnakeCase`) for the
runtime implementation.

## Example

```go
type Sale struct {
    nitro.Model
    CustomerName string     `validate:"required"`             // json: "customer_name", db: "customer_name"
    Status       string     `db:"status" default:"open"`      // json: "status" (default), db: "status" (explicit, same value here)
    Total        float64                                       // json: "total", db: "total"
    Items        []SaleItem `json:"items" db:"-"`              // relation: only in JSON, never persisted directly
}
```

## Where this applies

- **Persistence** (`sqladapter.StoreOf` / `pgxadapter.Store`): `Create`/`Update`
  use the resolved DB column for every field, including ones with no `db`
  tag at all — see [Data layer](data-layer.md).
- **HTTP JSON** (`c.OK`, `c.Created`, `nitro.Parse[T]`, `c.BodyJSON`): the
  resolved JSON key is used for both encoding responses and decoding request
  bodies (including PATCH merges and default-value application) — see
  [Responses](responses.md) and [Validation](validation.md).
- **BLinq metamodel** (`nitrogen make <model>.go --data`): the generated
  `modelgen` DSL already derives column names the same way when the `db` tag
  is missing.
- **Resource filters/sort** (`app.Resource` inferred `Filterable`/`Sortable`
  from `blinq:"index"`): uses the same resolved name.

## Protecting fields from mass assignment (`readonly`)

`app.Resource` binds the request body directly onto the persistence model
for `Create`/`Update` (see [Resources](resources.md)). This is convenient,
but it means **any exported field the client can guess the JSON name of is
writable from the request body** unless you protect it — a classic mass
assignment / overposting bug (CWE-915): a client could send
`{"name":"x","is_admin":true}` and have `IsAdmin` persisted.

Tag any field that must never come from the client with `readonly:"true"`
or `computed:"true"` (same mass-assignment protection; prefer `computed` for
denormalized / derived values filled by hooks).
The framework resets it after binding — to its zero value on
create/replace, or restored to the pre-update value on a PATCH merge —
**before** any hook or repository call sees the entity:

```go
type User struct {
    nitro.Model
    Email    string `validate:"required,email"`
    Name     string `validate:"required"`
    IsAdmin  bool   `readonly:"true"` // never settable from the request body
    TenantID string `readonly:"true"` // set only by BeforeCreate/BeforeUpdate hooks
}

type Album struct {
    nitro.Model
    ArtistID   string `json:"artist_id"`
    ArtistName string `json:"artist_name" computed:"true"` // filled in BeforeWrite
}
```

Tags on embedded structs are honored too (e.g. `nitro.TenantModel.TenantID`).

**System fields need no tag.** By convention `ID`, `CreatedAt`, `UpdatedAt`
and `DeletedAt` are owned by the server: on create the body's values are
discarded (UUID v7 and timestamps are generated), and on update `CreatedAt` /
`DeletedAt` are restored from the stored row. A client therefore cannot pick a
primary key (and overwrite another row or tenant), back-date timestamps or
create an already soft-deleted row. A string `ID` with `AutoID: false`
(natural keys) is still taken from the body.

To assign a `readonly` / `computed` field's real value, use `BeforeWrite`
(runs on create **and** update) or `BeforeCreate` / `BeforeUpdate`, which run
*after* the reset and *before* persistence. Order: defaults → BeforeWrite →
BeforeCreate|BeforeUpdate. For FK lookups, use `nitro.FillStringFrom`:

```go
opts.BeforeWrite = func(ctx context.Context, entity any) error {
    a := entity.(*Album)
    return nitro.FillStringFrom(ctx, artists, a.ArtistID, &a.ArtistName,
        func(ar *Artist) string { return ar.Name })
}
```

Prefer this over trusting the request body for any field that represents a
privilege, ownership, or financial value.

## Performance note

Models that keep declaring both tags explicitly (the style used throughout
this repo's examples) pay **zero** extra cost: the JSON encode/decode path
detects there is nothing to rename and takes the exact same route as before
(`encoding/json` directly). The rename step only runs for types that
actually have at least one field relying on the default name.
