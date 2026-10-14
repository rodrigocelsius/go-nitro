# Error Handling

**Idioma:** [Português](errors.md) · [English](../../guide/errors.md)

Erros HTTP no Nitro. Sucesso: [Responses](responses.md).

## Preferência v1

| Uso | API |
|-----|-----|
| Sucesso | `c.OK` / `c.Created` / `c.NoContent` — **não** `nitro.OK` |
| Erro no handler | `c.NotFound(...)` **ou** `nitro.NotFound(...)` |
| Validação body | `nitro.Parse[T](c)` → retorna 400/422 pronto |
| Not found no Store | `pgx.ErrNotFound` / `domain.ErrNotFound` → 404 automático |

## Construtores de erro

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

## Exemplos

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

return nitro.Err(400, "INVALID_EMAIL", "campo email é obrigatório")
```

## Propagação

```go
app.Use(func(c *nitro.Ctx) error {
    err := c.Next()
    if err != nil {
        c.Log().Error("request failed", "err", err)
    }
    return err
})
```

## Ver também

- [Request](request.md) · [Validation](validation.md) · [philosophy](philosophy.md)
