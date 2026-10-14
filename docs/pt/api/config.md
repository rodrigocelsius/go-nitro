# Configuração

**Idioma:** [Português](config.md) · [English](../../api/config.md)

Opções para `nitro.New(nitro.Config{...})`.

## Config

```go
app := nitro.New(nitro.Config{
    Env:                     nitro.EnvDevelopment, // or EnvProduction (default)
    RequestTimeout:          30 * time.Second,     // 0 = no per-request timeout
    DisableRequestID:        false,
    DisableDefaults:         false, // Recover + Logger + MaxBody (+ CORS in dev)
    DisableGracefulShutdown: false, // Listen handles SIGINT/SIGTERM
    Security: nitro.SecurityConfig{
        DisableHeaders: false,
        DisableHSTS:    false,
        CSP:            "default-src 'self'",
        CORS: nitro.CORSConfig{
            AllowedOrigins: []string{"https://app.example.com"},
        },
    },
})
```

### Campos

| Field | Type | Default | Descrição |
|-------|------|---------|-------------|
| `Env` | `Env` | `EnvProduction` | Afeta HSTS e CORS de dev |
| `RequestTimeout` | `time.Duration` | `0` | Timeout por requisição (408) |
| `TLSConfig` | `*tls.Config` | `nil` | Usado por Listen / ListenTLS |
| `ErrorHandler` | `func(*Ctx, error) error` | `nil` | Override global de erro |
| `DisableRequestID` | `bool` | `false` | Pula o middleware RequestID |
| `DisableDefaults` | `bool` | `false` | Pula Recover/Logger/MaxBody/CORS de dev |
| `DisableGracefulShutdown` | `bool` | `false` | Listen cru, sem signals |
| `DisableTelemetry` | `bool` | `false` | Pula bootstrap OTel HTTP |
| `DisableStartupMessage` | `bool` | `false` | Pula o banner ASCII no Listen |
| `EnableLogger` | `bool` | `false` | Força access logger mesmo com stacks customizados |
| `EnablePrometheus` | `bool` | `false` | Registra endpoint Prometheus |
| `PrometheusPath` | `string` | `/metrics` | Path quando `EnablePrometheus` |
| `ServiceName` | `string` | (env/OTEL) | Nome do serviço OTel |
| `UseLinearRouter` | `bool` | `false` | Só debug / benchmarks |
| `Security` | `SecurityConfig` | defaults seguros | Headers + CORS opcional |

```go
app := nitro.New(nitro.Config{
    EnableLogger:     true,
    EnablePrometheus: true,
    PrometheusPath:   "/metrics",
})
```

## SecurityConfig

| Field | Type | Default | Descrição |
|-------|------|---------|-------------|
| `DisableHeaders` | `bool` | `false` | Desabilita todos os security headers |
| `DisableHSTS` | `bool` | `false` | Pula HSTS (também off em development) |
| `CSP` | `string` | `default-src 'self'` | Content-Security-Policy |
| `CORS` | `CORSConfig` | empty | Aplicado quando `AllowedOrigins` está setado |

## CORSConfig

| Field | Type | Default | Descrição |
|-------|------|---------|-------------|
| `AllowedOrigins` | `[]string` | none | Origins explícitas (ou `*` via PublicStack / Defaults em dev) |
| `AllowedMethods` | `[]string` | GET,POST,PUT,DELETE,OPTIONS | … |
| `AllowedHeaders` | `[]string` | Content-Type, Authorization | … |
| `AllowCredentials` | `bool` | `false` | Não pode combinar com `*` |
| `MaxAge` | `int` | 600 | Segundos de cache do preflight |

## Graceful shutdown

```go
// Default: graceful
nitro.Ok(app.Listen(":8080"))

// Custom timeout / OnShutdown
nitro.Ok(app.Listen(":8080", nitro.GracefulShutdownConfig{
    Timeout: 60 * time.Second,
    OnShutdown: func() { /* close DB */ },
}))

// Lifecycle hooks (OnListen / OnShutdown / OnRoute)
app.Hooks().OnShutdown(func() error { return db.Close() })

// Disable
app := nitro.New(nitro.Config{DisableGracefulShutdown: true})
```

## Helpers de env (`ConfigOf`)

```go
type AppCfg struct {
    Port int      `env:"PORT" default:"8080"`
    DSN  string   `env:"DB_DSN,DATABASE_URL" validate:"required"` // aliases
    CORS []string `env:"CORS_ORIGINS" default:"http://localhost:3000"` // CSV
    DB   Nest     `env:"DB_"` // children → DB_HOST, …
}
cfg, err := nitro.ConfigOf[AppCfg]()
```

Structs aninhadas, `[]string` CSV e aliases `env:"A,B"` são suportados. Guia: [`docs/guide/config.md`](../guide/config.md).

**Não** sobrescreve o env do processo já existente ao carregar `.env`.
