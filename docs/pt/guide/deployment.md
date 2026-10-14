# Deploy

**Idioma:** [Português](deployment.md) · [English](../../guide/deployment.md)

Este guia cobre o deploy de aplicações Nitro em produção.

## Configuração de ambiente

O Nitro usa `domain.Config` para comportamento consciente do ambiente:

```go
app := nitro.New(nitro.Config{
    Env: nitro.EnvProduction,  // Enables security headers, disables debug output
})
```

### Ambientes

| Ambiente | Security Headers | HSTS | Recover / Logger / MaxBody | CORS de dev |
|-------------|------------------|------|----------------------------|----------|
| `nitro.EnvProduction` | Ativado | Ativado | On (`New`) | Off |
| `nitro.EnvDevelopment` | Ativado | Desativado | On (`New`) | `*` |

Opt-out: `Config.DisableDefaults`. Graceful shutdown vem ligado por padrão no `Listen`.

## Graceful Shutdown

Trata `SIGINT` / `SIGTERM` — **ativado por padrão** no `Listen`:

```go
app := nitro.New()
nitro.Ok(app.Listen(":8080"))
```

Desligar: `nitro.New(nitro.Config{DisableGracefulShutdown: true})`.

### Configuração customizada de shutdown

```go
app.Listen(":8080", nitro.GracefulShutdownConfig{
    Timeout: 60 * time.Second,  // Default: 30s
    OnShutdown: func() { /* close resources */ },
})

// Ou lifecycle hooks (também disparam em Shutdown / graceful):
app.Hooks().
    OnListen(func(d nitro.ListenData) error { return warmup() }).
    OnShutdown(func() error { return db.Close() })
```

O graceful shutdown:
1. Escuta `SIGINT` ou `SIGTERM`
2. Para de aceitar novas conexões
3. Espera as requisições em voo terminarem (até o timeout)
4. Chama `Shutdown()` no servidor subjacente (dispara `Hooks.OnShutdown`)

## TLS / HTTPS

### Opção 1: arquivos de certificado

```go
app.ListenTLS(":443", "/path/to/cert.pem", "/path/to/key.pem")
```

### Opção 2: TLS programático

```go
tlsConfig := &tls.Config{
    MinVersion: tls.VersionTLS13,
}
app.EnableTLS(tlsConfig)
app.Listen(":443")
```

### Opção 3: Auto TLS (Let's Encrypt)

```go
app.ListenAutoTLS(":443", "example.com", "www.example.com")
```

## Health Checks

Registre endpoints de health check para orquestradores (Kubernetes, ECS, etc.):

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

Isso cria automaticamente três endpoints:

| Endpoint | Propósito |
|----------|---------|
| `GET /health` | Liveness check (o processo está vivo?) |
| `GET /ready` | Readiness check (a app está pronta para receber tráfego?) |
| `GET /live` | Liveness check (alias para Kubernetes) |

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

## Security Headers (produção)

O Nitro aplica automaticamente estes headers em produção:

| Header | Value |
|--------|-------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Content-Security-Policy` | `default-src 'self'` |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains; preload` |
| `X-XSS-Protection` | `1; mode=block` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `none` |

### CSP customizado

```go
app := nitro.New(nitro.Config{
    Security: nitro.SecurityConfig{
        CSP: "default-src 'self'; script-src 'self' 'unsafe-inline'",
    },
})
```

### Desabilitar headers

```go
app := nitro.New(nitro.Config{
    DisableHeaders: true,  // Disable all security headers
})
```

## CORS (produção)

Configuração explícita de CORS para produção:

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

Proteja contra abuso:

```go
// 100 requests per minute per IP
app.Use(nitro.RateLimit(100, time.Minute))
```

### Função de chave customizada

```go
// Rate limit by API key
app.Use(nitro.RateLimit(1000, time.Minute))

// For trusted proxies, use the proxy-aware key function
// (in adapters/fasthttp/ratelimit.go)
```

## Timeout

Defina timeouts de requisição para evitar clientes lentos:

```go
// Global timeout: 30 seconds per request
app.Use(nitro.TimeoutMiddleware(30 * time.Second))
```

Ou via config do servidor:

```go
app := nitro.New(nitro.Config{
    RequestTimeout: 30 * time.Second,
})
```

## Compressão

Ative gzip para comprimir respostas:

```go
app.UsePost(nitro.CompressMiddleware())  // Must be post-middleware
```

Respostas com menos de 1024 bytes não são comprimidas (o overhead não vale a pena).

## Checklist de produção

- [ ] Defina `Env: nitro.EnvProduction`
- [ ] Configure CORS de forma explícita (sem `*` em produção)
- [ ] Configure TLS/HTTPS
- [ ] Registre health checks
- [ ] Ative graceful shutdown
- [ ] Defina timeouts de requisição
- [ ] Configure rate limiting
- [ ] Ative compressão
- [ ] Configure logging estruturado
- [ ] Configure tracing OpenTelemetry
- [ ] Use variáveis de ambiente para segredos (nunca hardcode)
- [ ] Defina `CookieSecure: true` em aplicações BFF
