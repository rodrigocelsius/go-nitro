# Middleware Reference

**Language:** [English](README.md) · [Português](../pt/middleware/README.md)

Nitro includes built-in middleware. Many are **on by default** via `nitro.New()`.

## Defaults (active on `New()`)

| Layer | What | Opt-out |
|-------|------|---------|
| Adapter | Security headers (Helmet-like) | `Config.Security.DisableHeaders` |
| Adapter | Request ID (UUID) | `Config.DisableRequestID` |
| Defaults | Recover (panic → 500) | `Config.DisableDefaults` |
| Defaults | Logger (access log) | `Config.DisableDefaults` |
| Defaults | MaxBody 1 MiB | `Config.DisableDefaults` |
| Defaults | CORS `*` (dev only) | `EnvProduction` or `DisableDefaults` |
| Config | CORS from `Security.CORS` | set `AllowedOrigins` in production |

```go
app := nitro.New() // all of the above

// Minimal (benchmarks / custom stacks):
app := nitro.New(nitro.Config{DisableDefaults: true})
```

## Stacks (opt-in)

```go
app.UseStack(nitro.APIStack)    // Defaults + RateLimit(100/min) + Compress
app.UseStack(nitro.PublicStack) // Defaults + CORS * (also in production)
```

## Security: implemented vs when to enable

| Concern | Status | Default? | Notes |
|---------|--------|----------|-------|
| Security headers | ✅ | Yes | CSP, nosniff, frame deny, HSTS (prod) |
| Request ID | ✅ | Yes | Correlation / tracing |
| Panic recover | ✅ | Yes | Avoid process crash |
| Body size limit | ✅ | Yes (1 MiB) | Raise with `MaxBodySize` |
| CORS | ✅ | Dev only | Production: set `Security.CORS` or PublicStack |
| Rate limit | ✅ | No | Use `APIStack` or `RateLimit` |
| CSRF | ✅ | No (API) | Use Web BFF or `StandaloneCSRFMiddleware` for cookie forms |
| JWT / Session | ✅ | No | Route-level auth |
| IP whitelist | ✅ | No | Admin / internal |
| TLS / HSTS | ✅ | HSTS in prod | Terminate TLS at proxy or `ListenTLS` |
| Timeout | ✅ | No | `Config.RequestTimeout` or timeout middleware |

Nothing critical for a JSON API is missing as **code** — CSRF and RateLimit stay opt-in by design.

## CORS

```go
app.Use(nitro.CORSMiddleware(nitro.CORSConfig{
    AllowedOrigins:   []string{"https://example.com"},
    AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge:           86400,
}))
```

Or via config:

```go
nitro.New(nitro.Config{
    Security: nitro.SecurityConfig{
        CORS: nitro.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}},
    },
})
```

## Compress

```go
app.UsePost(nitro.CompressMiddleware())
// or APIStack
```

## RequestID

Enabled automatically. Read with `c.RequestID()` when available.

## Security headers

```go
app.Use(nitro.SecurityHeaders())
// or SecurityHeaders(nitro.SecurityConfig{CSP: "default-src 'self'"})
```

Already applied by `New()` unless `DisableHeaders`.

## RateLimit

```go
app.Use(nitro.RateLimit(100, time.Minute)) // 100 req / window (by IP)
app.Use(nitro.RateLimitWith(1000, time.Minute, nitro.CompositeKeyFunc())) // IP + X-API-Key
```

Key helpers: `IPKeyFunc`, `APIKeyKeyFunc`, `CompositeKeyFunc`, `HeaderKeyFunc`.

## Timeout

```go
app := nitro.New(nitro.Config{RequestTimeout: 30 * time.Second})
// or middleware:
app.Use(nitro.TimeoutMiddleware(5 * time.Second))
```

## Recover

```go
app.Use(nitro.RecoverMiddleware()) // already in New()
```

## CSRF (browser / forms)

```go
app.Get("/csrf", nitro.StandaloneCSRFToken())
app.Use(nitro.StandaloneCSRFMiddleware())
```

Web BFF enables CSRF by default on cookie auth routes.

## JWT

```go
app.Get("/me", meHandler, nitro.JWTMiddleware(provider))
```

Claims: `c.Claims()`, `c.UserID()`.

## Logger

```go
app.Logger() // already in New()
app.Logger(nitro.LoggerConfig{Format: "json", SkipRoutes: []string{"/health"}})
```

## IP Whitelist

```go
app.Use(nitro.IPWhiteList("127.0.0.1", "10.0.0.0/8"))
```
