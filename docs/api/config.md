# Configuration

**Language:** [English](config.md) · [Português](../pt/api/config.md)

Options for `nitro.New(nitro.Config{...})`.

## Config

```go
app := nitro.New(nitro.Config{
    Env:                     nitro.ParseEnv(os.Getenv("NITRO_ENV")), // omitted: nitro.New() reads NITRO_ENV (development|dev|local), else production
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

### Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Env` | `Env` | `EnvProduction` | Affects HSTS and dev CORS |
| `RequestTimeout` | `time.Duration` | `0` | Per-request timeout (408) |
| `TLSConfig` | `*tls.Config` | `nil` | Used by Listen / ListenTLS |
| `ErrorHandler` | `func(*Ctx, error) error` | `nil` | Global error override |
| `DisableRequestID` | `bool` | `false` | Skip RequestID middleware |
| `DisableDefaults` | `bool` | `false` | Skip Recover/Logger/MaxBody/dev CORS |
| `DisableGracefulShutdown` | `bool` | `false` | Raw Listen without signals |
| `DisableTelemetry` | `bool` | `false` | Skip OTel HTTP bootstrap |
| `DisableStartupMessage` | `bool` | `false` | Skip ASCII banner on Listen |
| `EnableLogger` | `bool` | `false` | Force access logger even with custom stacks |
| `EnablePrometheus` | `bool` | `false` | Register Prometheus endpoint |
| `PrometheusPath` | `string` | `/metrics` | Path when `EnablePrometheus` |
| `ServiceName` | `string` | (env/OTEL) | OTel service name |
| `UseLinearRouter` | `bool` | `false` | Debug / benchmarks only |
| `Security` | `SecurityConfig` | secure defaults | Headers + optional CORS |

```go
app := nitro.New(nitro.Config{
    EnableLogger:     true,
    EnablePrometheus: true,
    PrometheusPath:   "/metrics",
})
```

## SecurityConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `DisableHeaders` | `bool` | `false` | Disable all security headers |
| `DisableHSTS` | `bool` | `false` | Skip HSTS (also off in development) |
| `CSP` | `string` | `default-src 'self'` | Content-Security-Policy |
| `CORS` | `CORSConfig` | empty | Applied when `AllowedOrigins` is set |

## CORSConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `AllowedOrigins` | `[]string` | none | Explicit origins (or `*` via PublicStack / Defaults in dev) |
| `AllowedMethods` | `[]string` | GET,POST,PUT,DELETE,OPTIONS | … |
| `AllowedHeaders` | `[]string` | Content-Type, Authorization | … |
| `AllowCredentials` | `bool` | `false` | Cannot combine with `*` |
| `MaxAge` | `int` | 600 | Preflight cache seconds |

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

## Env helpers (`ConfigOf`)

```go
type AppCfg struct {
    Port int      `env:"PORT" default:"8080"`
    DSN  string   `env:"DB_DSN,DATABASE_URL" validate:"required"` // aliases
    CORS []string `env:"CORS_ORIGINS" default:"http://localhost:3000"` // CSV
    DB   Nest     `env:"DB_"` // children → DB_HOST, …
}
cfg, err := nitro.ConfigOf[AppCfg]()
```

Nested structs, `[]string` CSV, and `env:"A,B"` aliases are supported. Guide: [`docs/guide/config.md`](../guide/config.md).

Does **not** overwrite existing process env when loading `.env`.

