# FAQ

**Language:** [English](faq.md) · [Português](../pt/extra/faq.md)

## General

### What is Nitro?

Nitro is a high-performance Go web framework built on [fasthttp](https://github.com/valyala/fasthttp). It follows Clean Architecture principles with strict layer separation and includes built-in middleware, authentication, BFF patterns, and a BLinq data layer.

### How does Nitro compare to Fiber?

Both frameworks are built on fasthttp. Nitro differentiates itself with:
- **Clean Architecture** layer separation
- **Built-in auth** (JWT, sessions, OAuth2)
- **BFF patterns** (Web and Mobile)
- **BLinq data layer** with type-safe queries
- **Code generation** for CRUD routes, JSON, SQL

### What Go version do I need?

- **All modules:** Go **1.25+**

### How do I get CRUD + SQL with the fewest commands?

```bash
nitrogen new shop --module github.com/acme/shop
cd shop
nitrogen install sqlite   # or postgresql
nitrogen make:scaffold Product name:string price:float64 active:bool --resource
nitrogen up
```

See the [CLI guide](../guide/cli.md) and [examples matrix](../../examples/README.md). Ergonomics work that shipped: [`ERGONOMICS_ROADMAP.md`](../../ERGONOMICS_ROADMAP.md) (Phases 0–5).

### How do I open the database?

Prefer **`sqladapter.OpenEnv`** / **`platform.Open`** (after `nitrogen install …`). Postgres still uses pgx under the hood — do not call raw pools in app code. Guide: [data layer](../guide/data-layer.md).

## Installation

### How do I install Nitro?

```bash
go get github.com/rodrigocelsius/go-nitro/pkg/v2
```

Go modules live under `github.com/rodrigocelsius/go-nitro/<module>/v2` (e.g. `pkg/v2`, `adapters/fasthttp/v2`). Source: [github.com/rodrigocelsius/go-nitro](https://github.com/rodrigocelsius/go-nitro).

### Can I use Nitro with `go mod`?

Yes. Add the import and run `go mod tidy`:

```go
import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
```

## Routing

### How do I add route parameters?

Use `:param` syntax:

```go
app.Get("/users/:id", func(c *nitro.Ctx) error {
    id := c.Param("id")
    return c.OK(nitro.Map{"id": id})
})
```

### How many parameters can I have?

Maximum 8 parameters per route (`nitro.MaxParams`).

### How do I create route groups?

```go
api := app.Group("/api")
api.Get("/users", listHandler)
api.Post("/users", createHandler)
```

### How do I auto-generate CRUD routes?

Use `app.Resource()`:

```go
app.Resource("/users", &User{}, userRepo)
// Generates: GET, GET/:id, POST, PATCH/:id, DELETE/:id (cache 5m by default)
```

## Middleware

### How do I add middleware?

```go
// Global middleware
app.Use(myMiddleware)

// Route-specific middleware
app.Get("/admin", adminHandler, authMiddleware)

// Group middleware
api := app.Group("/api", authMiddleware)
```

### What middleware is built-in?

- CORS
## Built-in Middleware

Active by default on `nitro.New()`: security headers, RequestID, Recover, Logger, MaxBody (CORS in development).

Opt-in stacks / middleware:

- Compress (gzip) — `APIStack` or `UsePost(Compress())`
- RateLimit — `APIStack` or `RateLimit(n, window)`
- Timeout — `Config.RequestTimeout` / `TimeoutMiddleware`
- JWT / Session / CSRF / IPWhiteList — route or BFF level

See [middleware guide](../guide/middleware.md).

## Authentication

### How do I use JWT?

```go
tokenProvider := nitro.JWTProvider(nitro.AuthConfig{
    JWTSecret:      "at-least-32-characters-long-random-secret",
    AccessTokenTTL: 15 * time.Minute,
})

app.Use(nitro.JWTMiddleware(tokenProvider))
```

### How do I get the current user?

```go
claims, err := c.Claims()
userID := c.UserID()
```

### How do I use OAuth2?

```go
google := nitro.GoogleOAuth(nitro.OAuthConfig{
    ClientID:     "your-client-id",
    ClientSecret: "your-client-secret",
    RedirectURL:  "http://localhost:8080/auth/google/callback",
})

app.Get("/auth/google", nitro.OAuthRedirectHandler(google))
app.Get("/auth/google/callback", nitro.OAuthCallbackHandler(google, auth))
```

## Error Handling

### How do I return errors?

```go
return nitro.NotFound("user not found")
return nitro.BadRequest("invalid json")
return nitro.InternalError(err)
```

### What error constructors are available?

Success: `c.OK` / `c.Created` / `c.NoContent`. Errors:

| Function | Status |
|----------|--------|
| `c.BadRequest` / `nitro.BadRequest` | 400 |
| `c.Unauthorized` / `nitro.Unauthorized` | 401 |
| `c.Forbidden` / `nitro.Forbidden` | 403 |
| `c.NotFound` / `nitro.NotFound` | 404 |
| `c.Conflict` / `nitro.Conflict` | 409 |
| `c.UnprocessableEntity` / `nitro.UnprocessableEntity` | 422 |
| `c.InternalError` / `nitro.InternalError` | 500 |

## Testing

### How do I test my app?

Use `app.Test()` for in-process testing:

```go
func TestGetUser(t *testing.T) {
    app := nitro.New()
    app.Get("/users/:id", getUserHandler)
    
    req := httptest.NewRequest("GET", "/users/42", nil)
    resp, err := app.Test(req)
    
    if err != nil {
        t.Fatal(err)
    }
    
    if resp.StatusCode != 200 {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }
}
```

## Performance

### How do I enable compression?

```go
app.Use(nitro.CompressMiddleware(nitro.CompressConfig{
    Level: 5,
}))
```

### How do I optimize for production?

1. Set appropriate timeouts:
```go
app := nitro.New(nitro.Config{
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
    IdleTimeout:  120 * time.Second,
})
```

2. Enable security headers:
```go
app.Use(nitro.SecurityMiddleware())
```

3. Use the compiled router for extreme performance:
```go
//go:generate go run gen_router.go
```

## Deployment

### How do I deploy to production?

1. Use graceful shutdown:
```go
app.Listen(":8080", nitro.GracefulShutdownConfig{
    ShutdownTimeout: 10 * time.Second,
})
```

2. Set up health checks:
```go
app.HealthCheck(func() error {
    return db.Ping()
})
```

3. Use environment variables for secrets:
```go
app := nitro.New(nitro.Config{
    // ...
})
```

### How do I use Docker?

For SQL apps use a **Go 1.25+** builder image:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
CMD ["./server"]
```

## Deprecated / prefer these instead

| Avoid | Prefer |
|-------|--------|
| `nitrogen g resource` / `make:migration` | `make:scaffold` / `migrate:make` |
| `nitro.OK` / `nitro.Created` for success | `c.OK` / `c.Created` |
| Opening pgx / `database/sql` pools in app code | `sqladapter.OpenEnv` / `platform.Open` |
| Folder `examples/v1/db-linq` as the default | `db-blinq` / `blinq-hello` |
| Homemade `.env` parser | `nitro.ConfigOf` |
