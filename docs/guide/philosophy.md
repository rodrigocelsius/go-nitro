# Nitro v1 philosophy (1.0)

**Language:** [English](philosophy.md) · [Português](../pt/guide/philosophy.md)

> Less code, fewer allocations, faster. Everything starts and ends on `c`.

## Principle

| Principle | legacy (pre-1.0) | v1 (current) |
|-----------|------------------|------------|
| Parameters | `c.QueryInt("page", 1)` | `c.Int("page", 1)` |
| Bind + validate | `c.BindJSON(&v)` | `nitro.Parse[T](c)` / `c.BodyParser(&v)` / `c.BodyJSON(&v)` |
| JSON response | `return nitro.OK(body)` | `return c.OK(body)` |
| Text | `c.SendString("ok")` | `return c.Text("ok")` |
| No content | `return nitro.NoContent()` | `return c.NoContent()` |
| Sort | manual | `field, desc := c.Sort("created_at")` |

**v1 rule:** success responses are context methods (`c.OK`, `c.Created`). Domain errors may use `nitro.NotFound(...)` or `c.NotFound(...)`.

Canonical import:

```go
nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
```

## Quick comparison

### Hello World

```go
// v1
app.Get("/", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"message": "Hello, World!"})
})
```

### CRUD (excerpt)

```go
app.Get("/users/:id", func(c *nitro.Ctx) error {
    user, err := userRepo.FindByID(c, c.Param("id"))
    if err != nil {
        return c.NotFound("user not found")
    }
    return c.OK(user)
})

app.Post("/users", func(c *nitro.Ctx) error {
    user, err := nitro.Parse[User](c)
    if err != nil {
        return err // 400 or 422
    }
    return c.Created(user)
})
```

## Legacy → v1 table

| Action | legacy | v1 |
|------|--------|----|
| Query int | `c.QueryIntDefault("page", 1)` | `c.Int("page", 1)` |
| Query float | `c.QueryFloat64Default(...)` | `c.Float(...)` |
| Query bool | `c.QueryBool("active")` | `c.Bool("active")` |
| Bind+validate | `c.BindJSON(&v)` | `Parse` / `BodyParser` / `BodyJSON` |
| 200 / 201 / 204 | `nitro.OK` / `Created` / `NoContent` | `c.OK` / `c.Created` / `c.NoContent` |
| Text | `c.SendString` | `c.Text` |
| CORS / Compress | `*Middleware` | `nitro.CORS` / `nitro.Compress` |

## Data (Postgres)

**BLinq is the default:** `pgxadapter.Open` → `StoreOf` → DSL in `modelgen` (`nitrogen make <model>.go --data`). GORM, sqlx, and sqlc only if the requirement asks for them, always via the Gateway.

## See also

- [Request](request.md) · [Responses](responses.md) · [Errors](errors.md)
- [BLinq](blinq.md) · [Data layer](data-layer.md)
- [AI Playbook](../ai/PLAYBOOK.md) · [Short changelog](api-changelog.md)
