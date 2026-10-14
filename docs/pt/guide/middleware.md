# Middleware

**Idioma:** [Português](middleware.md) · [English](../../guide/middleware.md)

Funções de middleware rodam antes ou depois dos handlers e tratam preocupações transversais como logging, autenticação e compressão.

## Defaults no `nitro.New()`

Você **não** precisa registrar Recover, Logger, MaxBody, RequestID ou security headers manualmente — eles já vêm ativos por padrão.

```go
app := nitro.New() // secure + observable defaults

// Opt-out:
app := nitro.New(nitro.Config{DisableDefaults: true})
```

Veja a [referência de middleware](../middleware/README.md) para a tabela completa de defaults e o checklist de segurança.

## Como o middleware funciona

```go
type Handler func(c *nitro.Ctx) error
```

- Retorne `nil` para seguir ao próximo middleware/handler
- Retorne um `error` para interromper a cadeia e enviar uma resposta de erro

```go
app.Use(func(c *nitro.Ctx) error {
    fmt.Println("Before handler")
    err := c.Next()
    fmt.Println("After handler")
    return err
})
```

## Middleware global

```go
app.Use(nitro.RateLimit(100, time.Minute))
app.Use(nitro.CORSMiddleware(nitro.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}}))
```

## Middleware por rota

```go
app.Get("/admin", adminHandler, authMiddleware, adminRoleMiddleware)
```

## Middleware de grupo

```go
api := app.Group("/api", authMiddleware)
api.Get("/profile", profileHandler)
```

## Middleware embutido

| Middleware | Descrição | Default? |
|------------|-------------|----------|
| Security headers | CSP, nosniff, frame deny, HSTS | Sim |
| Request ID | UUID por requisição | Sim |
| Recover | Recuperação de panic | Sim |
| Logger | Access log | Sim |
| MaxBody | Limite de body de 1 MiB | Sim |
| CORS | Cross-origin | Dev / config |
| RateLimit | Sliding window | Via `APIStack` |
| Compress | gzip | Via `APIStack` |
| CSRF | Cookie double-submit | BFF / manual |
| Timeout | Timeout por requisição | Opt-in |
| JWT / Session | Auth | Opt-in |
| IPWhiteList | Allowlist de IP/CIDR | Opt-in |

Exemplos completos da API: [middleware/README.md](../middleware/README.md).

## Stacks

```go
app.UseStack(nitro.APIStack)    // + RateLimit + Compress
app.UseStack(nitro.PublicStack) // + CORS * (incl. production)
```

Ver também: [Recipes](recipes.md) · [changelog da API](api-changelog.md).

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

## Dashboard e Prometheus

```go
app.Dashboard("/admin/nitro")
app.Prometheus() // GET /metrics
app.Metrics()    // só coleta
```

## Scheduler

```go
app.Schedule("*/5 * * * *", func(ctx context.Context) error {
    return cleanup(ctx)
})
```

## Desabilitar defaults

```go
app := nitro.New(nitro.Config{
    Env:             nitro.EnvDevelopment,
    DisableDefaults: true, // ou Security.DisableHeaders / DisableRequestID
})
```

## Middleware pre vs post

```go
app.Use(func(c *nitro.Ctx) error {
    return c.Next()
})

app.UsePost(func(c *nitro.Ctx) error {
    return nil
})
```
