# Bem-vindo ao Nitro

**Idioma:** [Português](welcome.md) · [English](../../guide/welcome.md)

Nitro é um framework web Go de alta performance, construído sobre [fasthttp](https://github.com/valyala/fasthttp), com Clean Architecture, zero alocações e batteries included.

## Instalação

Requer **Go 1.25+**.

```bash
go get github.com/rodrigocelsius/go-nitro/pkg/v2
```

Os módulos Go têm paths como `github.com/rodrigocelsius/go-nitro/pkg/v2` (cada módulo termina em `/v2`). O código-fonte e as issues ficam em [github.com/rodrigocelsius/go-nitro](https://github.com/rodrigocelsius/go-nitro) (`git clone https://github.com/rodrigocelsius/go-nitro.git`).

## Hello, World

```go
package main

import (
    nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
    app := nitro.New()
    app.Get("/", func(c *nitro.Ctx) error {
        return c.OK(nitro.Map{"message": "Hello, Nitro!"})
    })
    nitro.Ok(app.Listen(":8080"))
}
```

```bash
go run main.go
```

## Conceitos centrais

- `nitro.New()` — defaults seguros (Recover, Logger, MaxBody, RequestID, security headers)
- `app.Get("/", …)` — registra handler
- `app.Listen(":8080")` — graceful shutdown (SIGINT/SIGTERM)

**Regra v1:** respostas no context — `c.OK`, `c.Created`, `c.Text`. Ver [filosofia](philosophy.md).

## Monte sua primeira app

### Parâmetros de rota

```go
app.Get("/users/:id", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"id": c.Param("id")})
})
```

### JSON + validate

```go
app.Post("/users", func(c *nitro.Ctx) error {
    user, err := nitro.Parse[User](c)
    if err != nil {
        return err // 400 / 422
    }
    return c.Created(user)
})
```

### Erros

```go
if user == nil {
    return c.NotFound("user not found") // ou nitro.NotFound(...)
}
return c.OK(user)
```

### Groups

```go
api := app.Group("/api")
api.Get("/users", listUsers)
api.Post("/users", createUser)
```

### Resource (CRUD)

```go
repo := nitro.NewMemoryRepo[Todo]()
nitro.Resource(app, "/todos", repo, &nitro.ResourceConfig{NoCache: true})
```

Com Postgres o padrão é **BLinq** (`StoreOf` + `modelgen`): [`examples/v1/blinq-hello`](../../examples/v1/blinq-hello/). GORM/sqlx/sqlc só se o requisito pedir.

## Zero Allocation

Valores de `c.Param` / `c.Query` vivem no pool — use só dentro do handler.

## Próximos passos

- [CLI](cli.md) — `nitrogen new` → `install` → `make:scaffold` → `up`
- [Índice do guia](README.md)
- [Request](request.md) · [Responses](responses.md) · [Recipes](recipes.md)
- [BLinq](blinq.md) · [Camada de dados](data-layer.md)
- [Playbook de IA](../ai/PLAYBOOK.md)
