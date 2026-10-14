# Referência de middleware

**Idioma:** [Português](README.md) · [English](../../middleware/README.md)

O Nitro inclui middleware embutido. Muitos já vêm **ligados por padrão** via `nitro.New()`.

## Defaults (ativos no `New()`)

| Camada | O quê | Opt-out |
|-------|------|---------|
| Adapter | Security headers (estilo Helmet) | `Config.Security.DisableHeaders` |
| Adapter | Request ID (UUID) | `Config.DisableRequestID` |
| Defaults | Recover (panic → 500) | `Config.DisableDefaults` |
| Defaults | Logger (access log) | `Config.DisableDefaults` |
| Defaults | MaxBody 1 MiB | `Config.DisableDefaults` |
| Defaults | CORS `*` (só em dev) | `EnvProduction` ou `DisableDefaults` |
| Config | CORS a partir de `Security.CORS` | defina `AllowedOrigins` em produção |

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

## Segurança: implementado vs quando ativar

| Preocupação | Status | Default? | Notas |
|---------|--------|----------|-------|
| Security headers | ✅ | Sim | CSP, nosniff, frame deny, HSTS (prod) |
| Request ID | ✅ | Sim | Correlação / tracing |
| Panic recover | ✅ | Sim | Evita crash do processo |
| Limite de tamanho do body | ✅ | Sim (1 MiB) | Aumente com `MaxBodySize` |
| CORS | ✅ | Só em dev | Produção: defina `Security.CORS` ou PublicStack |
| Rate limit | ✅ | Não | Use `APIStack` ou `RateLimit` |
| CSRF | ✅ | Não (API) | Use Web BFF ou `StandaloneCSRFMiddleware` para forms com cookie |
| JWT / Session | ✅ | Não | Auth no nível da rota |
| IP whitelist | ✅ | Não | Admin / interno |
| TLS / HSTS | ✅ | HSTS em prod | Encerre TLS no proxy ou `ListenTLS` |
| Timeout | ✅ | Não | `Config.RequestTimeout` ou middleware de timeout |

Nada crítico para uma API JSON falta como **código** — CSRF e RateLimit permanecem opt-in de propósito.

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

Ou via config:

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

Ativado automaticamente. Leia com `c.RequestID()` quando disponível.

## Security headers

```go
app.Use(nitro.SecurityHeaders())
// or SecurityHeaders(nitro.SecurityConfig{CSP: "default-src 'self'"})
```

Já aplicados pelo `New()`, salvo `DisableHeaders`.

## RateLimit

```go
app.Use(nitro.RateLimit(100, time.Minute)) // 100 req / window (por IP)
app.Use(nitro.RateLimitWith(1000, time.Minute, nitro.CompositeKeyFunc())) // IP + X-API-Key
```

Helpers de chave: `IPKeyFunc`, `APIKeyKeyFunc`, `CompositeKeyFunc`, `HeaderKeyFunc`.

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

O Web BFF ativa CSRF por padrão nas rotas de auth com cookie.

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
