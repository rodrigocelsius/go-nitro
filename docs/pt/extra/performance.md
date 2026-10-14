# Ajuste de performance

**Idioma:** [Português](performance.md) · [English](../../extra/performance.md)

O Nitro foi feito para alta performance. Este guia cobre técnicas de otimização e configuração.

## Benchmarks

Rode a suíte completa de benchmarks:

```bash
go test -bench=. -benchmem ./bench/...
```

### Resultados comparativos

| Framework | Req/sec | Allocs/op | ns/op |
|-----------|---------|-----------|-------|
| **Nitro** | 142,000 | 6 | 8,450 |
| Fiber | 138,000 | 8 | 8,720 |
| Gin | 98,000 | 12 | 12,100 |
| stdlib | 52,000 | 18 | 23,400 |

*Benchmark: 100 clientes concorrentes, duração de 10s, resposta JSON simples*

## Design de zero allocation

O Nitro chega perto de zero alocações no hot path por meio de:

1. **Arrays de parâmetros de tamanho fixo** — parâmetros de rota usam arrays `[8]string` em vez de maps
2. **Sync.Pool para Ctx** — contexts são reutilizados entre requisições
3. **Middleware pré-compilado** — pipelines de middleware são compilados uma vez no registro, não por requisição
4. **Sem reflection no hot path** — reflection só no startup, para registro de resource

## Otimização de middleware

### Pipeline pré-compilado

O Nitro compila a cadeia de middleware uma vez, no momento do registro da rota:

```go
// This compiles the middleware chain immediately:
app.Get("/users", handler, authMiddleware, loggingMiddleware)
```

A cadeia compilada é um único closure — sem dispatch dinâmico, sem chamadas a `next()`, sem recursão.

### Post-middleware

Use `UsePost` para middleware no nível da resposta (compressão, logging):

```go
app.UsePost(nitro.CompressMiddleware())  // Runs AFTER handler
```

Isso evita overhead em requisições que encerram cedo (ex.: respostas 401).

## Connection pooling

No PostgreSQL, configure o pool de conexões de forma adequada:

```go
poolConfig, _ := pgxpool.ParseConfig(dsn)
poolConfig.MaxConns = 20              // Match your CPU cores * 2
poolConfig.MinConns = 5               // Keep warm connections
poolConfig.MaxConnLifetime = time.Hour // Recycle old connections
poolConfig.MaxConnIdleTime = 30 * time.Minute

pool, _ := pgxpool.NewWithConfig(ctx, poolConfig)
```

## Caching

### Cache no nível da query

Hints de cache na DSL BLinq: `query.Cache(ttl)` / `CacheTag(tag)`. Honrados por `provider.Wrap` + `provider.Cache` (ex.: `provider/cache.Memory`). O Gateway continua com `gateway.WithCache` no caminho SQL legado.

### Cache no nível do Gateway

O Gateway cacheia automaticamente resultados de `Query` e invalida em `Exec`:

```go
gw := gateway.New(executor,
    gateway.WithCache(redisCache),  // Cache implementation
)
```

Formato da chave de cache: `db:select:<sql>` ou `db:exec:<sql>`

## Compressão de resposta

Ative compressão gzip para respostas grandes:

```go
app.UsePost(nitro.CompressMiddleware(nitro.CompressConfig{
    Level: gzip.BestSpeed,  // Fastest compression (slightly larger output)
}))
```

Níveis de compressão:
- `gzip.DefaultCompression` (6) — Equilibrado
- `gzip.BestSpeed` (1) — Mais rápido, saída maior
- `gzip.BestCompression` (9) — Mais lento, saída menor

Respostas com menos de 1024 bytes são ignoradas automaticamente (o overhead não vale a pena).

## Rate Limiting

Proteja contra abuso sem prejudicar tráfego legítimo:

```go
// Sliding window (default) — more accurate than fixed window
app.Use(nitro.RateLimit(100, time.Minute))

// Token bucket — allows burst traffic
// Use via adapters/fasthttp/ratelimit.go
```

## Configuração de timeout

Evite que clientes lentos prendam recursos:

```go
// Per-request timeout (cooperative)
app.Use(nitro.TimeoutMiddleware(10 * time.Second))

// Server-level timeout (enforced)
app := nitro.New(nitro.Config{
    RequestTimeout: 30 * time.Second,
})
```

## Gestão de memória

### Reuso de context

O Nitro reutiliza objetos `Ctx` via `sync.Pool`. Nunca guarde referências a `*Ctx` além da vida do handler:

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

### Limites de body

No parse do body da requisição, considere definir limites:

```go
// Read body with limit
body := c.Body()
if len(body) > 1<<20 { // 1MB limit
    return nitro.BadRequest("request too large")
}
```

## Compiled Router

Para o máximo de performance de roteamento, use o compiled router (gerado por código):

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

O compiled router substitui a busca em árvore em runtime por um dispatch direto de função, eliminando o overhead do matching dinâmico de rotas.

## Configuração de produção

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

## Monitoramento

### OpenTelemetry

```go
import "github.com/rodrigocelsius/go-nitro/adapters/otel/v2"

app.Use(otel.TracingMiddleware(tracer))
app.Use(otel.MetricsMiddleware(meter))
```

### Logging estruturado

```go
import "github.com/rodrigocelsius/go-nitro/adapters/log/v2"

app.Use(log.LoggingMiddleware(logger))
app.Use(log.RequestLogger())
app.UsePost(log.RequestLoggerPost())
```

## Profiling

Use o profiler nativo do Go:

```go
import _ "net/http/pprof"

go func() {
    http.ListenAndServe(":6060", nil)
}()
```

Em seguida:

```bash
go tool pprof http://localhost:6060/debug/pprof/heap
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```
