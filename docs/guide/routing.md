# Routing

**Language:** [English](routing.md) · [Português](../pt/guide/routing.md)

How the application responds to path + HTTP method.

## Registration

```go
app.Get("/", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"message": "Hello!"})
})
app.Post("/users", createUser)
app.Put("/users/:id", updateUser)
app.Patch("/users/:id", patchUser)
app.Delete("/users/:id", deleteUser)
```

## Path parameters

```go
app.Get("/users/:id", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"id": c.Param("id")})
})

app.Get("/orders/:orderId/items/:itemId", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{
        "order": c.Param("orderId"),
        "item":  c.Param("itemId"),
    })
})
```

Maximum 8 params (`nitro.MaxParams`). Typed / required / defaults: [Request](request.md).

## Query

Use `c.Query` / `c.Int` / `c.Bool` — see [Request](request.md).  
(`QueryParam` is not the v1 API.)

## Groups

```go
api := app.Group("/api")
api.Get("/users", listUsers)

v1 := api.Group("/v1", authMiddleware)
v1.Get("/profile", getProfile)
```

## Mount (sub-app)

```go
v1 := nitro.New()
v1.Get("/users", listUsers)
v1.Use(v1OnlyMiddleware)

app.Mount("/api/v1", v1) // GET /api/v1/users
```

The request path is rewritten (prefix stripped) before forwarding to the sub-app `Handler`; sub-app middlewares are preserved. Sub-app route names are available on the parent with the prefixed path.

## Naming and reverse routing

```go
app.Get("/users/:id", getUser).Name("user.show")

url := app.GetRouteURL("user.show", nitro.Map{"id": 42}) // "/users/42"
url = app.GetRouteURL("user.show", nitro.Map{"id": 42})

// In the handler:
return c.OK(nitro.Map{"href": c.URL("user.show", nitro.Map{"id": c.Param("id")})})
```

```go
for _, route := range app.Routes() {
    fmt.Printf("%s %s -> %s\n", route.Method, route.Path, route.Name)
}
```

## Static / SPA

```go
app.Static("/assets", "./assets")
app.Static("/", "./public", nitro.StaticConfig{SinglePageApp: true})
// equivalent: StaticConfig{NotFound: "index.html"}
```

## Hooks (lifecycle)

```go
app.Hooks().
    OnListen(func(d nitro.ListenData) error {
        log.Printf("listening on %s", d.Addr)
        return nil
    }).
    OnShutdown(func() error {
        // close DB, queues, …
        return nil
    }).
    OnRoute(func(info nitro.RouteInfo) error {
        // introspection when a route is registered
        return nil
    })
```

`OnListen` runs before accepting connections; `OnShutdown` after `Shutdown` / graceful stop.
`OnRoute` fires when the route is registered and **again** when `.Name(...)` is called (with `RouteInfo.Name`).
See also `GracefulShutdownConfig.OnShutdown` in [Deployment](deployment.md).

## Resource (CRUD)

```go
repo := nitro.NewMemoryRepo[User]()
app.Resource("/users", repo, &nitro.ResourceConfig{NoCache: true})

// Postgres — StoreOf implements AsResourceRepository (no AdaptStore on the happy path):
app.Resource("/products", &Product{}, store, opts)

// Convention path (model plural):
app.Resource(&Product{}, store) // → /products

// Group.Resource inherits group middlewares; WriteMiddleware only on POST/PATCH/DELETE:
api := app.Group("/api", jwt)
api.Resource("/drivers", &Driver{}, repo, &nitro.ResourceConfig{
    WriteMiddleware: []nitro.Handler{nitro.RequireRole("admin")},
})
```

Generates: `GET/POST /users`, `GET/PATCH/DELETE /users/:id`.  
Defaults: AutoID (UUID v7), timestamps, 5m cache. Opt-out: `&nitro.ResourceConfig{NoCache: true}`.

Middleware on Resource:

| Field | Applies to |
|-------|-----------|
| `Middleware` | all Resource routes |
| `ReadMiddleware` | GET (list + get) |
| `WriteMiddleware` | POST / PUT / PATCH / DELETE |

## Per-route middleware

```go
app.Get("/admin", adminHandler, authMiddleware, adminRoleMiddleware)
```

## See also

- [Request](request.md) · [Middleware](middleware.md) · [Recipes](recipes.md) · [Data layer](data-layer.md)
