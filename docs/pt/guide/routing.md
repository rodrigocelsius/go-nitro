# Routing

**Idioma:** [Português](routing.md) · [English](../../guide/routing.md)

Como a aplicação responde a path + método HTTP.

## Registro

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

Máximo 8 params (`nitro.MaxParams`). Tipados / obrigatórios / defaults: [Request](request.md).

## Query

Use `c.Query` / `c.Int` / `c.Bool` — ver [Request](request.md).  
(`QueryParam` não é a API v1.)

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

O path do request é reescrito (prefixo removido) antes de encaminhar ao `Handler` do sub-app; middlewares do sub-app são preservados. Nomes de rota do sub-app ficam disponíveis no parent com path prefixado.

## Nomeação e reverse routing

```go
app.Get("/users/:id", getUser).Name("user.show")

url := app.GetRouteURL("user.show", nitro.Map{"id": 42}) // "/users/42"
url = app.GetRouteURL("user.show", nitro.Map{"id": 42})

// No handler:
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
// equivalente: StaticConfig{NotFound: "index.html"}
```

## Hooks (ciclo de vida)

```go
app.Hooks().
    OnListen(func(d nitro.ListenData) error {
        log.Printf("listening on %s", d.Addr)
        return nil
    }).
    OnShutdown(func() error {
        // fechar DB, filas, …
        return nil
    }).
    OnRoute(func(info nitro.RouteInfo) error {
        // introspecção ao registrar rota
        return nil
    })
```

`OnListen` roda antes de aceitar conexões; `OnShutdown` após `Shutdown` / graceful stop.
`OnRoute` dispara ao registrar a rota e **de novo** quando `.Name(...)` é chamado (com `RouteInfo.Name`).
Ver também `GracefulShutdownConfig.OnShutdown` em [Deployment](deployment.md).

## Resource (CRUD)

```go
repo := nitro.NewMemoryRepo[User]()
app.Resource("/users", repo, &nitro.ResourceConfig{NoCache: true})

// Postgres — StoreOf implementa AsResourceRepository (sem AdaptStore no happy path):
app.Resource("/products", &Product{}, store, opts)

// Path por convenção (plural do model):
app.Resource(&Product{}, store) // → /products

// Group.Resource herda middlewares do group; WriteMiddleware só em POST/PATCH/DELETE:
api := app.Group("/api", jwt)
api.Resource("/drivers", &Driver{}, repo, &nitro.ResourceConfig{
    WriteMiddleware: []nitro.Handler{nitro.RequireRole("admin")},
})
```

Gera: `GET/POST /users`, `GET/PATCH/DELETE /users/:id`.  
Defaults: AutoID (UUID v7), timestamps, cache 5m. Opt-out: `&nitro.ResourceConfig{NoCache: true}`.

Middleware no Resource:

| Campo | Aplica em |
|-------|-----------|
| `Middleware` | todas as rotas do Resource |
| `ReadMiddleware` | GET (list + get) |
| `WriteMiddleware` | POST / PUT / PATCH / DELETE |

## Middleware por rota

```go
app.Get("/admin", adminHandler, authMiddleware, adminRoleMiddleware)
```

## Ver também

- [Request](request.md) · [Middleware](middleware.md) · [Recipes](recipes.md) · [Data layer](data-layer.md)
