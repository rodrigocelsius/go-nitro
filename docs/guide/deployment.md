# Deployment

**Language:** [English](deployment.md) · [Português](../pt/guide/deployment.md)

This guide covers deploying Nitro applications to production.

## Environment Configuration

Nitro uses `domain.Config` for environment-aware behavior:

```go
app := nitro.New(nitro.Config{
    Env: nitro.EnvProduction,  // Enables security headers, disables debug output
})
```

### Environments

| Environment | Security Headers | HSTS | Recover / Logger / MaxBody | Dev CORS |
|-------------|------------------|------|----------------------------|----------|
| `nitro.EnvProduction` | Enabled | Enabled | On (`New`) | Off |
| `nitro.EnvDevelopment` | Enabled | Disabled | On (`New`) | `*` |

`nitro.New()` picks the environment from `Config.Env`, else `NITRO_ENV` (`development`|`dev`|`local`), else production; `nitro.ParseEnv(name)` converts a config string. Opt-out: `Config.DisableDefaults`. Graceful shutdown is on by default for `Listen`.

## Graceful Shutdown

Handle `SIGINT` / `SIGTERM` — **enabled by default** on `Listen`:

```go
app := nitro.New()
nitro.Ok(app.Listen(":8080"))
```

Disable: `nitro.New(nitro.Config{DisableGracefulShutdown: true})`.

### Custom Shutdown Config

```go
app.Listen(":8080", nitro.GracefulShutdownConfig{
    Timeout: 60 * time.Second,  // Default: 30s
    OnShutdown: func() { /* close resources */ },
})

// Or lifecycle hooks (also fire on Shutdown / graceful):
app.Hooks().
    OnListen(func(d nitro.ListenData) error { return warmup() }).
    OnShutdown(func() error { return db.Close() })
```

The graceful shutdown:
1. Listens for `SIGINT` or `SIGTERM`
2. Stops accepting new connections
3. Waits for in-flight requests to complete (up to timeout)
4. Calls `Shutdown()` on the underlying server (fires `Hooks.OnShutdown`)

## TLS / HTTPS

### Option 1: Certificate Files

```go
app.ListenTLS(":443", "/path/to/cert.pem", "/path/to/key.pem")
```

### Option 2: Programmatic TLS

```go
tlsConfig := &tls.Config{
    MinVersion: tls.VersionTLS13,
}
app.EnableTLS(tlsConfig)
app.Listen(":443")
```

### Option 3: Auto TLS (Let's Encrypt)

```go
app.ListenAutoTLS(":443", "example.com", "www.example.com")
```

## Health Checks

Register health check endpoints for orchestrators (Kubernetes, ECS, etc.):

```go
// Simple function-based health check
app.HealthCheck(func() error {
    // Check database connectivity
    return db.Ping(ctx)
})

// Custom health checker
type DBHealthCheck struct {
    db *sql.DB
}
func (h *DBHealthCheck) Name() string { return "database" }
func (h *DBHealthCheck) Check() error { return h.db.Ping(context.Background()) }

app.HealthCheck(&DBHealthCheck{db: db})
```

This automatically creates three endpoints:

| Endpoint | Purpose |
|----------|---------|
| `GET /health` | Liveness check (is the process alive?) |
| `GET /ready` | Readiness check (is the app ready to serve traffic?) |
| `GET /live` | Liveness check (alias for Kubernetes) |

## Docker

### Dockerfile

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]
```

### Docker Compose

```yaml
version: '3.8'
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://user:pass@db:5432/mydb?sslmode=disable
      - JWT_SECRET=your-secret-key
    depends_on:
      - db

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
      POSTGRES_DB: mydb
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"

volumes:
  pgdata:
```

## Kubernetes

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nitro-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nitro-app
  template:
    metadata:
      labels:
        app: nitro-app
    spec:
      containers:
      - name: app
        image: your-registry/nitro-app:latest
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 3
          periodSeconds: 5
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "500m"
```

## Security Headers (Production)

Nitro automatically applies these headers in production:

| Header | Value |
|--------|-------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Content-Security-Policy` | `default-src 'self'` |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains; preload` |
| `X-XSS-Protection` | `1; mode=block` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `none` |

### Custom CSP

```go
app := nitro.New(nitro.Config{
    Security: nitro.SecurityConfig{
        CSP: "default-src 'self'; script-src 'self' 'unsafe-inline'",
    },
})
```

### Disable Headers

```go
app := nitro.New(nitro.Config{
    DisableHeaders: true,  // Disable all security headers
})
```

## CORS (Production)

Explicit CORS configuration for production:

```go
app.Use(nitro.CORSMiddleware(nitro.CORSConfig{
    AllowedOrigins:   []string{"https://app.example.com"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge:           86400,
}))
```

## Rate Limiting

Protect against abuse:

```go
// 100 requests per minute per IP
app.Use(nitro.RateLimit(100, time.Minute))
```

### Custom Key Function

```go
// Rate limit by API key
app.Use(nitro.RateLimit(1000, time.Minute))

// For trusted proxies, use the proxy-aware key function
// (in adapters/fasthttp/ratelimit.go)
```

## Timeout

Set request timeouts to prevent slow clients:

```go
// Global timeout: 30 seconds per request
app.Use(nitro.TimeoutMiddleware(30 * time.Second))
```

Or via server config:

```go
app := nitro.New(nitro.Config{
    RequestTimeout: 30 * time.Second,
})
```

## Compression

Enable gzip for response compression:

```go
app.UsePost(nitro.CompressMiddleware())  // Must be post-middleware
```

Responses under 1024 bytes are not compressed (overhead not worth it).

## Production Checklist

- [ ] Set `Env: nitro.EnvProduction`
- [ ] Configure CORS explicitly (no `*` in production)
- [ ] Set up TLS/HTTPS
- [ ] Register health checks
- [ ] Enable graceful shutdown
- [ ] Set request timeouts
- [ ] Configure rate limiting
- [ ] Enable compression
- [ ] Set up structured logging
- [ ] Configure OpenTelemetry tracing
- [ ] Use environment variables for secrets (never hardcode)
- [ ] Set `CookieSecure: true` for BFF applications
