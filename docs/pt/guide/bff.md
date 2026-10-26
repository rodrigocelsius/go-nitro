# Backend for Frontend (BFF)

**Idioma:** [Português](bff.md) · [English](../../guide/bff.md)

O Nitro tem suporte de primeira classe ao padrão **Backend for Frontend (BFF)**, com configurações dedicadas para clientes web e mobile.

## O que é BFF?

BFF é um padrão arquitetural em que você cria backends separados, adequados a cada tipo de frontend. Em vez de uma única API servir web e mobile, você cria um **Web BFF** (otimizado para browsers com cookies/CSRF) e um **Mobile BFF** (otimizado para auth stateless baseada em token, com respostas compactas).

## Início rápido: Web BFF

```go
package main

import (
    "time"
    nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
    app := nitro.New()

    // Configure services
    services := []nitro.ServiceConfig{
        {Name: "users", BaseURL: "http://users-service:8081"},
        {Name: "orders", BaseURL: "http://orders-service:8082"},
    }

    // Create Web BFF config
    cfg := nitro.NewWebBFFConfig(services...)
    cfg.AccessTokenTTL = 15 * time.Minute
    cfg.RefreshTokenTTL = 7 * 24 * time.Hour

    // Set up auth
    auth := nitro.NewAuthUseCase(nitro.AuthDeps{
        PasswordHash: nitro.PasswordHasherAdapter(),
        TokenProvider: nitro.JWTProvider(nitro.AuthConfig{
            AccessTTL:  cfg.AccessTokenTTL,
            RefreshTTL: cfg.RefreshTokenTTL,
            Secret:     "your-secret-key",
        }),
        UserLookup:  myUserRepo,
        UserCreator: myUserRepo,
    })

    tokenProvider := nitro.JWTProvider(nitro.AuthConfig{
        AccessTTL: cfg.AccessTokenTTL,
        Secret:    "your-secret-key",
    })

    // Register all Web BFF routes (login, register, refresh, logout, CSRF)
    app.WebBFFRoutes("/web/auth", auth, cfg, tokenProvider)

    // Register protected proxy routes
    api := app.Group("/api")
    api.Use(nitro.JWTMiddleware(tokenProvider))
    api.Get("/users", nitro.BFFProxyHandler(cfg, "users", "/api/users"))
    api.Get("/users/:id", nitro.BFFProxyHandler(cfg, "users", "/api/users/:id"))
    api.Get("/orders", nitro.BFFProxyHandler(cfg, "orders", "/api/orders"))

    // Aggregate endpoint — fans out to multiple services
    api.Get("/dashboard", nitro.BFFAggregateHandler(cfg, []nitro.AggregateRequest{
        {Service: "users", Path: "/api/users/me", Key: "user"},
        {Service: "orders", Path: "/api/orders?limit=5", Key: "recent_orders"},
    }))

    app.Listen(":8080")
}
```

## Configuração do Web BFF

O `WebBFFConfig` oferece recursos específicos para o browser:

```go
cfg := nitro.NewWebBFFConfig(services...)

// Cookie configuration (secure by default)
cfg.TokenCookieName = "_at"       // Access token cookie
cfg.RefreshCookieName = "_rt"     // Refresh token cookie
cfg.CSRFCookieName = "_csrf"      // CSRF token cookie
cfg.CookieSecure = true           // HTTPS only
cfg.CookieSameSite = "Lax"        // CSRF protection
cfg.CookieHTTPOnly = true         // No JavaScript access

// Token TTLs
cfg.AccessTokenTTL = 15 * time.Minute
cfg.RefreshTokenTTL = 7 * 24 * time.Hour

// CSRF protection
cfg.EnableCSRF = true

// Response caching
cfg.CacheEnabled = true
cfg.CacheTTL = 5 * time.Minute
```

### Rotas do Web BFF

`WebBFFRoutes()` registra automaticamente:

| Method | Path | Descrição |
|--------|------|-------------|
| `POST` | `{prefix}/auth/login` | Login com e-mail/senha |
| `POST` | `{prefix}/auth/register` | Cadastro de usuário |
| `POST` | `{prefix}/auth/refresh` | Refresh de token |
| `POST` | `{prefix}/auth/logout` | Limpa cookies |
| `GET` | `{prefix}/csrf` | Obtém token CSRF |

## Início rápido: Mobile BFF

```go
cfg := nitro.NewMobileBFFConfig(services...)

// Token-based auth (no cookies)
cfg.TokenHeader = "Authorization"
cfg.TokenPrefix = "Bearer "
cfg.AccessTokenTTL = 30 * time.Minute
cfg.RefreshTokenTTL = 30 * 24 * time.Hour

// Mobile-specific features
cfg.EnableDeviceTracking = true
cfg.MaxDevicesPerUser = 5

// Register mobile auth routes
app.MobileBFFRoutes("/mobile/auth", auth, cfg, tokenProvider)
```

## Configuração do Mobile BFF

```go
cfg := nitro.NewMobileBFFConfig(services...)

// Token delivery
cfg.TokenHeader = "Authorization"  // HTTP header
cfg.TokenPrefix = "Bearer "        // Prefix before token

// Extended TTLs (mobile apps refresh less frequently)
cfg.AccessTokenTTL = 30 * time.Minute
cfg.RefreshTokenTTL = 30 * 24 * time.Hour

// Device tracking
cfg.EnableDeviceTracking = true
cfg.MaxDevicesPerUser = 5

// Os dispositivos ficam em cfg.Devices (memória por padrão; implemente
// nitro.DeviceStore sobre o seu banco para várias réplicas). Login/registro
// com {"device": {...}}; requisições com X-Device-ID atualizam last_seen_at.
// Tokens FCM/APNs ficam salvos no dispositivo; o envio de push é do app.
cfg.Devices = nitro.NewMemoryDeviceStore(cfg.MaxDevicesPerUser)

// Bandwidth optimization
// EnableBiometric / EnableOffline / DeltaSync / CompactResponse são reservados (ainda sem comportamento embutido)
```

### Rotas do Mobile BFF

`MobileBFFRoutes()` registra automaticamente:

| Method | Path | Descrição |
|--------|------|-------------|
| `POST` | `{prefix}/auth/login` | Login com e-mail/senha |
| `POST` | `{prefix}/auth/register` | Cadastro de usuário |
| `POST` | `{prefix}/auth/refresh` | Refresh de token |
| `POST` | `{prefix}/auth/logout` | Invalida tokens |
| `GET` | `{prefix}/devices` | Dispositivos do usuário autenticado (`EnableDeviceTracking`) |
| `DELETE` | `{prefix}/devices/:id` | Remove um dispositivo do usuário |

## Proxy Handler

Encaminhe requisições para microserviços upstream:

```go
// Single service proxy
app.Get("/api/users", nitro.BFFProxyHandler(cfg, "users", "/api/users"))
app.Get("/api/users/:id", nitro.BFFProxyHandler(cfg, "users", "/api/users/:id"))
app.Post("/api/orders", nitro.BFFProxyHandler(cfg, "orders", "/api/orders"))
```

O proxy handler:
1. Encaminha a requisição para o serviço upstream configurado
2. Repassa headers, body e query parameters
3. Devolve a resposta do upstream como está

## Aggregate Handler

Combine várias chamadas upstream em uma única resposta:

```go
app.Get("/api/dashboard", nitro.BFFAggregateHandler(cfg, []nitro.AggregateRequest{
    {Service: "users", Path: "/api/users/me", Key: "user"},
    {Service: "orders", Path: "/api/orders?limit=5", Key: "recent_orders"},
    {Service: "products", Path: "/api/products?featured=true", Key: "featured_products"},
}))
```

Resposta:

```json
{
    "user": { "id": 1, "name": "Joao" },
    "recent_orders": [ ... ],
    "featured_products": [ ... ]
}
```

## Configuração de serviços

Defina os serviços upstream:

```go
services := []nitro.ServiceConfig{
    {
        Name:    "users",
        BaseURL: "http://users-service:8081",
    },
    {
        Name:    "orders",
        BaseURL: "http://orders-service:8082",
    },
    {
        Name:    "products",
        BaseURL: "http://products-service:8083",
    },
}
```

## Proteção CSRF (Web)

O Web BFF inclui proteção CSRF nativa. O fluxo:

1. O cliente busca um token CSRF: `GET /web/auth/csrf`
2. O token entra nas requisições seguintes que alteram estado
3. O servidor valida o token automaticamente

```go
// The CSRF token endpoint is auto-registered by WebBFFRoutes
// Client-side JavaScript:
const { token } = await fetch('/web/auth/csrf').then(r => r.json())
await fetch('/api/orders', {
    method: 'POST',
    headers: { 'X-CSRF-Token': token },
    body: JSON.stringify({ ... })
})
```

## Rastreamento de dispositivos (Mobile)

O Mobile BFF rastreia dispositivos registrados por usuário:

```go
cfg.EnableDeviceTracking = true
cfg.MaxDevicesPerUser = 5

// When a user logs in from a 6th device, the oldest device is automatically removed
```

## Handlers customizados

Você pode adicionar handlers próprios junto das rotas BFF:

```go
app.WebBFFRoutes("/web/auth", auth, cfg, tokenProvider)

// Custom endpoint with BFF middleware
api := app.Group("/api")
api.Use(nitro.JWTMiddleware(tokenProvider))
api.Get("/custom", customHandler)
```

## Exemplo: Mobile BFF com aggregate da home

```go
func main() {
    app := nitro.New()

    cfg := nitro.NewMobileBFFConfig(
        nitro.ServiceConfig{Name: "users", BaseURL: "http://users:8081"},
        nitro.ServiceConfig{Name: "orders", BaseURL: "http://orders:8082"},
        nitro.ServiceConfig{Name: "notifications", BaseURL: "http://notif:8083"},
    )
    cfg.EnableDeviceTracking = true
    cfg.MaxDevicesPerUser = 5

    auth := setupAuth()
    tokenProvider := nitro.JWTProvider(nitro.AuthConfig{Secret: "secret"})

    app.MobileBFFRoutes("/mobile/auth", auth, cfg, tokenProvider)

    api := app.Group("/api")
    api.Use(nitro.JWTMiddleware(tokenProvider))

    // Single call for the entire home screen
    api.Get("/home", nitro.BFFAggregateHandler(cfg, []nitro.AggregateRequest{
        {Service: "users", Path: "/api/users/me", Key: "profile"},
        {Service: "orders", Path: "/api/orders?limit=3&sort=recent", Key: "recent_orders"},
        {Service: "notifications", Path: "/api/notifications?unread=true", Key: "unread_count"},
    }))

    app.Listen(":8080")
}
```

## Comparação Web vs Mobile

| Recurso | Web BFF | Mobile BFF |
|---------|---------|------------|
| Entrega do token | Cookies seguros | Header HTTP (`Bearer`) |
| Proteção CSRF | Sim | Não (stateless) |
| TTL do token | 15 min / 7 dias | 30 min / 30 dias |
| Rastreamento de dispositivo | Não | Sim (máx. 5) |
| Tokens de push (FCM/APNs) salvos | Não | Sim (o envio é do app) |

## Exemplo completo

Os trechos acima são completos; `pkg/bff_devices_test.go` exercita o fluxo mobile de ponta a ponta (registro → login em vários dispositivos → listagem → remoção).
