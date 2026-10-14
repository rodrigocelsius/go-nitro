# Performance Tuning

**Language:** [English](performance.md) · [Português](../pt/extra/performance.md)

Nitro is built for high performance. This guide covers optimization techniques and configuration.

## Benchmarks

Run the full benchmark suite:

```bash
go test -bench=. -benchmem ./bench/...
```

### Comparison Results

| Framework | Req/sec | Allocs/op | ns/op |
|-----------|---------|-----------|-------|
| **Nitro** | 142,000 | 6 | 8,450 |
| Fiber | 138,000 | 8 | 8,720 |
| Gin | 98,000 | 12 | 12,100 |
| stdlib | 52,000 | 18 | 23,400 |

*Benchmark: 100 concurrent clients, 10s duration, simple JSON response*

## Zero Allocation Design

Nitro achieves near-zero allocations on the hot path through:

1. **Fixed-size parameter arrays** — Route parameters use `[8]string` arrays instead of maps
2. **Sync.Pool for Ctx** — Contexts are reused between requests
3. **Pre-compiled middleware** — Middleware pipelines are compiled once at registration, not per request
4. **No reflection on hot path** — Reflection is only used at startup for resource registration

## Middleware Optimization

### Pre-compiled Pipeline

Nitro compiles the middleware chain once at route registration time:

```go
// This compiles the middleware chain immediately:
app.Get("/users", handler, authMiddleware, loggingMiddleware)
```

The compiled chain is a single function closure — no dynamic dispatch, no `next()` calls, no recursion.

### Post-Middleware

Use `UsePost` for response-level middleware (compression, logging):

```go
app.UsePost(nitro.CompressMiddleware())  // Runs AFTER handler
```

This avoids overhead on requests that short-circuit (e.g., 401 responses).

## Connection Pooling

For PostgreSQL, configure the connection pool appropriately:

```go
poolConfig, _ := pgxpool.ParseConfig(dsn)
poolConfig.MaxConns = 20              // Match your CPU cores * 2
poolConfig.MinConns = 5               // Keep warm connections
poolConfig.MaxConnLifetime = time.Hour // Recycle old connections
poolConfig.MaxConnIdleTime = 30 * time.Minute

pool, _ := pgxpool.NewWithConfig(ctx, poolConfig)
```

## Caching

### Query-Level Caching

Cache hints on the BLinq DSL: `query.Cache(ttl)` / `CacheTag(tag)`. Honored by `provider.Wrap` + `provider.Cache` (e.g. `provider/cache.Memory`). The Gateway still has `gateway.WithCache` on the legacy SQL path.

### Gateway-Level Caching

The Gateway automatically caches `Query` results and invalidates on `Exec`:

```go
gw := gateway.New(executor,
    gateway.WithCache(redisCache),  // Cache implementation
)
```

Cache key format: `db:select:<sql>` or `db:exec:<sql>`

## Response Compression

Enable gzip compression for large responses:

```go
app.UsePost(nitro.CompressMiddleware(nitro.CompressConfig{
    Level: gzip.BestSpeed,  // Fastest compression (slightly larger output)
}))
```

Compression levels:
- `gzip.DefaultCompression` (6) — Balanced
- `gzip.BestSpeed` (1) — Fastest, larger output
- `gzip.BestCompression` (9) — Slowest, smallest output

Responses under 1024 bytes are automatically skipped (overhead not worth it).

## Rate Limiting

Protect against abuse without impacting legitimate traffic:

```go
// Sliding window (default) — more accurate than fixed window
app.Use(nitro.RateLimit(100, time.Minute))

// Token bucket — allows burst traffic
// Use via adapters/fasthttp/ratelimit.go
```

## Timeout Configuration

Prevent slow clients from tying up resources:

```go
// Per-request timeout (cooperative)
app.Use(nitro.TimeoutMiddleware(10 * time.Second))

// Server-level timeout (enforced)
app := nitro.New(nitro.Config{
    RequestTimeout: 30 * time.Second,
})
```

## Memory Management

### Context Reuse

Nitro reuses `Ctx` objects via `sync.Pool`. Never store references to `*Ctx` beyond the handler lifetime:

```go
// BAD — stores reference beyond handler lifetime
var storedCtx *nitro.Ctx
app.Get("/bad", func(c *nitro.Ctx) error {
    storedCtx = c  // WRONG! Context will be reused
    return nil
})

// GOOD — copy the data you need
var userID string
app.Get("/good", func(c *nitro.Ctx) error {
    userID = c.Param("id")  // String is copied
    return nil
})
```

### Body Limits

For request body parsing, consider setting limits:

```go
// Read body with limit
body := c.Body()
if len(body) > 1<<20 { // 1MB limit
    return nitro.BadRequest("request too large")
}
```

## Compiled Router

For maximum routing performance, use the compiled router (code-generated):

```go
//go:build ignore

package main

import (
    "github.com/rodrigocelsius/go-nitro/codegen/router/v2"
)

func main() {
    router.Generate("routes.json", "router_gen.go")
}
```

The compiled router replaces the runtime tree lookup with a direct function dispatch, eliminating the overhead of dynamic route matching.

## Production Configuration

```go
app := nitro.New(nitro.Config{
    Env:            nitro.EnvProduction,
    RequestTimeout: 30 * time.Second,
    Security: nitro.SecurityConfig{
        CSP: "default-src 'self'",
    },
})

app.Use(nitro.RateLimit(1000, time.Minute))
app.Use(nitro.TimeoutMiddleware(10 * time.Second))
app.UsePost(nitro.CompressMiddleware())

app.Listen(":8080")
```

## Monitoring

### OpenTelemetry

```go
import "github.com/rodrigocelsius/go-nitro/adapters/otel/v2"

app.Use(otel.TracingMiddleware(tracer))
app.Use(otel.MetricsMiddleware(meter))
```

### Structured Logging

```go
import "github.com/rodrigocelsius/go-nitro/adapters/log/v2"

app.Use(log.LoggingMiddleware(logger))
app.Use(log.RequestLogger())
app.UsePost(log.RequestLoggerPost())
```

## Profiling

Use Go's built-in profiler:

```go
import _ "net/http/pprof"

go func() {
    http.ListenAndServe(":6060", nil)
}()
```

Then:

```bash
go tool pprof http://localhost:6060/debug/pprof/heap
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```
