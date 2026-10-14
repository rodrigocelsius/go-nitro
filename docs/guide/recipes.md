# Recipes — practical patterns

**Language:** [English](recipes.md) · [Português](../pt/guide/recipes.md)

Short recipes. Full systems: [AI Playbook](../ai/PLAYBOOK.md).

## Resource (CRUD)

```go
app.Resource(&Product{}, store, &nitro.ResourceConfig{
    NoCache: true,
    FilterRules: []nitro.FilterRule{
        {Param: "min_price", Field: "price", Op: "gte"},
        {Param: "q", Field: "name", Op: "icontains"},
    },
})

nitro.Resource(app, "/todos", repo, &nitro.ResourceConfig{NoCache: true})
```

CoC path: `app.Resource(&Product{}, store)` → `/products`. Filterable/Sortable: `blinq:"index"` if omitted.

See: [Data layer](data-layer.md) · `examples/v1/blinq-hello` · `db-blinq`.

## Filters with BLinq

```go
items, err := store.Find(c, store.Query().
    WhereIf(c.Query("name") != "", Products.Name.IContains(c.Query("name"))).
    WhereIf(c.Float("min_price", 0) > 0, Products.Price.Gte(c.Float("min_price", 0))).
    WhereIf(c.Bool("active"), Products.Active.Eq(true)).
    Limit(50))
if err != nil {
    return err
}
return c.OK(items)
```

See: [BLinq](blinq.md) · `examples/v1/db-blinq-handlers`.

## JWT auth

```go
auth := nitro.NewAuth(nitro.AuthConfig{JWTSecret: "at-least-32-characters-long-random-secret"}).WithMemoryStore()
app.AuthRoutes("/auth", auth)

api := app.Group("", nitro.JWTMiddleware(auth.Provider()))
api.Get("/profile", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"user_id": c.UserID()})
})
```

See: [Authentication](authentication.md) · `examples/v1/auth`.

## Upload

```go
app.Post("/avatar", func(c *nitro.Ctx) error {
    file, err := c.FormFile("avatar")
    if err != nil {
        return c.BadRequest("no file uploaded")
    }
    data, err := io.ReadAll(file)
    if err != nil {
        return c.InternalError(err)
    }
    return c.Created(nitro.Map{"size": len(data), "name": file.Filename})
})
```

## SSE

```go
app.Get("/events", func(c *nitro.Ctx) error {
    c.SetHeader("Content-Type", "text/event-stream")
    c.SetHeader("Cache-Control", "no-cache")
    return c.Stream(reader) // or nitro.SSECreate(...)
})
```

## WebSocket

```go
hub := nitro.NewWSHub()
app.Get("/ws", nitro.WebSocketHandler(func(ws nitro.WebSocket) {
    hub.Join("global", ws)
    defer hub.Leave("global", ws)
    for {
        msg, err := ws.ReadMessage()
        if err != nil {
            break
        }
        hub.BroadcastTo("global", msg)
    }
}))
```

## Health

```go
app := nitro.New()
// GET /health, /ready, /live are automatic
app.HealthCheck(func() error { return db.Ping(ctx) })
```

## Mount + reverse URL

```go
v1 := nitro.New()
v1.Get("/items/:id", getItem).Name("item.show")
app.Mount("/api/v1", v1)

app.Get("/links", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"item": c.URL("item.show", nitro.Map{"id": 1})})
})
```

## Static SPA

```go
app.Static("/", "./dist", nitro.StaticConfig{SinglePageApp: true})
```

## Text, email, and codes

```go
phone := nitro.Digits(raw)                
if !nitro.IsDigits(code) || len(code) != 6 {
    return nitro.BadRequest("invalid code")
}
email := nitro.NormalizeEmail(rawEmail)
if !nitro.IsEmail(email) {
    return nitro.UnprocessableEntity("invalid email")
}
first, last := nitro.SplitName(fullName)
otp, err := nitro.RandomDigits(6)
```

See: [Validation](validation.md).

## Full app (skeleton)

```go
func main() {
    app := nitro.New()
    app.Use(nitro.CORSMiddleware(nitro.CORSConfig{AllowedOrigins: []string{"*"}}))
    app.Use(nitro.RateLimit(1000, time.Minute))

    repo := nitro.NewMemoryRepo[User]()
    nitro.Resource(app, "/users", repo, &nitro.ResourceConfig{NoCache: true})

    app.Get("/users/by-email/:email", func(c *nitro.Ctx) error {
        // ...
        return c.OK(user)
    })

    nitro.Ok(app.Listen(":8080"))
}
```

Postgres: `examples/v1/blinq-hello`. Clean arch: `examples/v1/cleanarch`.

## See also

- [Request](request.md) · [Responses](responses.md) · [Middleware](middleware.md)
