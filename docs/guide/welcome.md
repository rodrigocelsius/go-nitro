# Welcome to Nitro

**Language:** [English](welcome.md) · [Português](../pt/guide/welcome.md)

Nitro is a high-performance Go web framework built on [fasthttp](https://github.com/valyala/fasthttp) with Clean Architecture, zero allocations, and batteries included.

## Installation

Requires **Go 1.25+**.

```bash
go get github.com/rodrigocelsius/go-nitro/pkg/v2
```

Go module paths look like `github.com/rodrigocelsius/go-nitro/pkg/v2` (each module ends in `/v2`). Source and issues live at [github.com/rodrigocelsius/go-nitro](https://github.com/rodrigocelsius/go-nitro) (`git clone https://github.com/rodrigocelsius/go-nitro.git`).

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

## Core Concepts

- `nitro.New()` — secure defaults (Recover, Logger, MaxBody, RequestID, security headers)
- `app.Get("/", …)` — register a handler
- `app.Listen(":8080")` — graceful shutdown (SIGINT/SIGTERM)

**v1 rule:** responses live on the context — `c.OK`, `c.Created`, `c.Text`. See [philosophy](philosophy.md).

## Build Your First App

### Route parameters

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

### Errors

```go
if user == nil {
    return c.NotFound("user not found") // or nitro.NotFound(...)
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

With Postgres the default is **BLinq** (`StoreOf` + `modelgen`): [`examples/v1/blinq-hello`](../../examples/v1/blinq-hello/). Use GORM/sqlx/sqlc only if the requirement asks for them.

## Zero Allocation

Values from `c.Param` / `c.Query` live in the pool — use them only inside the handler.

## Next Steps

- [CLI](cli.md) — `nitrogen new` → `install` → `make:scaffold` → `up`
- [Guide index](README.md)
- [Request](request.md) · [Responses](responses.md) · [Recipes](recipes.md)
- [BLinq](blinq.md) · [Data layer](data-layer.md)
- [AI Playbook](../ai/PLAYBOOK.en.md)
