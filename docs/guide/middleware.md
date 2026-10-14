# Middleware

**Language:** [English](middleware.md) · [Português](../pt/guide/middleware.md)

Middleware functions run before or after handlers and handle cross-cutting concerns like logging, authentication, and compression.

## Defaults on `nitro.New()`

You do **not** need to register Recover, Logger, MaxBody, RequestID, or security headers manually — they are active by default.

```go
app := nitro.New() // secure + observable defaults

// Opt-out:
app := nitro.New(nitro.Config{DisableDefaults: true})
```

See [Middleware Reference](../middleware/README.md) for the full default table and security checklist.

## How Middleware Works

```go
type Handler func(c *nitro.Ctx) error
```

- Return `nil` to continue to the next middleware/handler
- Return an `error` to stop the chain and send an error response

```go
app.Use(func(c *nitro.Ctx) error {
    fmt.Println("Before handler")
    err := c.Next()
    fmt.Println("After handler")
    return err
})
```

## Global Middleware

```go
app.Use(nitro.RateLimit(100, time.Minute))
app.Use(nitro.CORSMiddleware(nitro.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}}))
```

## Route-Specific Middleware

```go
app.Get("/admin", adminHandler, authMiddleware, adminRoleMiddleware)
```

## Group Middleware

```go
api := app.Group("/api", authMiddleware)
api.Get("/profile", profileHandler)
```

## Built-in Middleware

| Middleware | Description | Default? |
|------------|-------------|----------|
| Security headers | CSP, nosniff, frame deny, HSTS | Yes |
| Request ID | UUID per request | Yes |
| Recover | Panic recovery | Yes |
| Logger | Access log | Yes |
| MaxBody | 1 MiB body limit | Yes |
| CORS | Cross-origin | Dev / config |
| RateLimit | Sliding window | Via `APIStack` |
| Compress | gzip | Via `APIStack` |
| CSRF | Double-submit cookie | BFF / manual |
| Timeout | Per-request timeout | Opt-in |
| JWT / Session | Auth | Opt-in |
| IPWhiteList | IP/CIDR allowlist | Opt-in |

Full API examples: [middleware/README.md](../middleware/README.md).

## Stacks

```go
app.UseStack(nitro.APIStack)    // + RateLimit + Compress
app.UseStack(nitro.PublicStack) // + CORS * (incl. production)
```

See also: [Recipes](recipes.md) · [api changelog](api-changelog.md).

## Access Log

```go
app.Logger()                                       // already on New
app.Logger(nitro.LoggerConfig{Format: "json"})
app.Logger(nitro.LoggerConfig{SkipRoutes: []string{"/health"}})
```

## IP Whitelist

```go
app.Use(nitro.IPWhiteList("127.0.0.1", "10.0.0.0/8"))
```

## Dashboard and Prometheus

```go
app.Dashboard("/admin/nitro")
app.Prometheus() // GET /metrics
app.Metrics()    // collect only
```

## Scheduler

```go
app.Schedule("*/5 * * * *", func(ctx context.Context) error {
    return cleanup(ctx)
})
```

## Disable defaults

```go
app := nitro.New(nitro.Config{
    Env:             nitro.EnvDevelopment,
    DisableDefaults: true, // or Security.DisableHeaders / DisableRequestID
})
```

## Pre vs Post Middleware

```go
app.Use(func(c *nitro.Ctx) error {
    return c.Next()
})

app.UsePost(func(c *nitro.Ctx) error {
    return nil
})
```
