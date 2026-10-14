# Filosofia Nitro v1 (1.0)

**Idioma:** [Português](philosophy.md) · [English](../../guide/philosophy.md)

> Menos código, menos alocações, mais rápido. Tudo começa e termina no `c`.

## Princípio

| Princípio | legado (pré-1.0) | v1 (atual) |
|-----------|------------------|------------|
| Parâmetros | `c.QueryInt("page", 1)` | `c.Int("page", 1)` |
| Bind + validate | `c.BindJSON(&v)` | `nitro.Parse[T](c)` / `c.BodyParser(&v)` / `c.BodyJSON(&v)` |
| Resposta JSON | `return nitro.OK(body)` | `return c.OK(body)` |
| Texto | `c.SendString("ok")` | `return c.Text("ok")` |
| No content | `return nitro.NoContent()` | `return c.NoContent()` |
| Sort | manual | `field, desc := c.Sort("created_at")` |

**Regra v1:** respostas de sucesso são métodos do context (`c.OK`, `c.Created`). Erros de domínio podem usar `nitro.NotFound(...)` ou `c.NotFound(...)`.

Import canônico:

```go
nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
```

## Comparação rápida

### Hello World

```go
// v1
app.Get("/", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"message": "Hello, World!"})
})
```

### CRUD (trecho)

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
        return err // 400 ou 422
    }
    return c.Created(user)
})
```

## Tabela legado → v1

| Ação | legado | v1 |
|------|--------|----|
| Query int | `c.QueryIntDefault("page", 1)` | `c.Int("page", 1)` |
| Query float | `c.QueryFloat64Default(...)` | `c.Float(...)` |
| Query bool | `c.QueryBool("active")` | `c.Bool("active")` |
| Bind+validate | `c.BindJSON(&v)` | `Parse` / `BodyParser` / `BodyJSON` |
| 200 / 201 / 204 | `nitro.OK` / `Created` / `NoContent` | `c.OK` / `c.Created` / `c.NoContent` |
| Texto | `c.SendString` | `c.Text` |
| CORS / Compress | `*Middleware` | `nitro.CORS` / `nitro.Compress` |

## Dados (Postgres)

**BLinq é o padrão:** `pgxadapter.Open` → `StoreOf` → DSL em `modelgen` (`nitrogen make <model>.go --data`). GORM, sqlx e sqlc só se o requisito pedir, sempre via Gateway.

## Ver também

- [Request](request.md) · [Responses](responses.md) · [Errors](errors.md)
- [BLinq](blinq.md) · [Data layer](data-layer.md)
- [AI Playbook](../ai/PLAYBOOK.md) · [Changelog resumido](api-changelog.md)
