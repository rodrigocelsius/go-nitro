# Error Handling

**Language:** [English](errors.md) · [Português](../pt/guide/errors.md)

HTTP errors in Nitro. Success: [Responses](responses.md).

## v1 preference

| Use | API |
|-----|-----|
| Success | `c.OK` / `c.Created` / `c.NoContent` — **not** `nitro.OK` |
| Error in the handler | `c.NotFound(...)` **or** `nitro.NotFound(...)` |
| Body validation | `nitro.Parse[T](c)` → ready-made 400/422 |
| Not found from Store | `pgx.ErrNotFound` / `domain.ErrNotFound` → automatic 404 |

## Error constructors

| API | Status |
|-----|--------|
| `c.BadRequest` / `nitro.BadRequest` | 400 |
| `c.Unauthorized` / `nitro.Unauthorized` | 401 |
| `c.Forbidden` / `nitro.Forbidden` | 403 |
| `c.NotFound` / `nitro.NotFound` | 404 |
| `c.Conflict` / `nitro.Conflict` | 409 |
| `c.UnprocessableEntity` / `nitro.UnprocessableEntity` | 422 |
| `c.InternalError` / `nitro.InternalError` | 500 |
| `c.Error(status, msg)` | custom |
| `nitro.Err(status, code, msg)` | custom + `code` |
| `nitro.ErrCode(code, msg)` | 400 + `code` |

## Examples

```go
app.Get("/users/:id", func(c *nitro.Ctx) error {
    id, err := c.Int64Param("id")
    if err != nil {
        return err
    }
    user, err := store.Get(c, id)
    if err != nil {
        return err // ErrNotFound → 404
    }
    return c.OK(user)
})

app.Post("/users", func(c *nitro.Ctx) error {
    user, err := nitro.Parse[User](c)
    if err != nil {
        return err
    }
    return c.Created(user)
})

return nitro.Err(400, "INVALID_EMAIL", "email field is required")
```

## Propagation

```go
app.Use(func(c *nitro.Ctx) error {
    err := c.Next()
    if err != nil {
        c.Log().Error("request failed", "err", err)
    }
    return err
})
```

## See also

- [Request](request.md) · [Validation](validation.md) · [philosophy](philosophy.md)
