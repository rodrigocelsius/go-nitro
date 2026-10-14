# Backend for Frontend (BFF)

**Language:** [English](bff.md) · [Português](../pt/guide/bff.md)

Nitro has first-class support for the **Backend for Frontend (BFF)** pattern, with dedicated configurations for web and mobile clients.

## What is BFF?

BFF is an architectural pattern where you create separate backend services tailored to each frontend type. Instead of one API serving both web and mobile, you create a **Web BFF** (optimized for browsers with cookies/CSRF) and a **Mobile BFF** (optimized for stateless token-based auth with compact responses).

## Quick Start: Web BFF

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

## Web BFF Configuration

The `WebBFFConfig` provides browser-specific features:

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

### Web BFF Routes

`WebBFFRoutes()` automatically registers:

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `{prefix}/login` | Email/password login |
| `POST` | `{prefix}/register` | User registration |
| `POST` | `{prefix}/refresh` | Token refresh |
| `POST` | `{prefix}/logout` | Clear cookies |
| `GET` | `{prefix}/csrf` | Get CSRF token |

## Quick Start: Mobile BFF

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
cfg.CompactResponse = true    // Minimized JSON for bandwidth savings

// Register mobile auth routes
app.MobileBFFRoutes("/mobile/auth", auth, cfg, tokenProvider)
```

## Mobile BFF Configuration

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

// Push notifications
cfg.EnablePushNotifications = true

// Bandwidth optimization
cfg.CompactResponse = true  // Removes whitespace from JSON
```

### Mobile BFF Routes

`MobileBFFRoutes()` automatically registers:

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `{prefix}/login` | Email/password login |
| `POST` | `{prefix}/register` | User registration |
| `POST` | `{prefix}/refresh` | Token refresh |
| `POST` | `{prefix}/logout` | Invalidate tokens |

## Proxy Handler

Proxy requests to upstream microservices:

```go
// Single service proxy
app.Get("/api/users", nitro.BFFProxyHandler(cfg, "users", "/api/users"))
app.Get("/api/users/:id", nitro.BFFProxyHandler(cfg, "users", "/api/users/:id"))
app.Post("/api/orders", nitro.BFFProxyHandler(cfg, "orders", "/api/orders"))
```

The proxy handler:
1. Forwards the request to the configured upstream service
2. Passes through headers, body, and query parameters
3. Returns the upstream response as-is

## Aggregate Handler

Combine multiple upstream calls into a single response:

```go
app.Get("/api/dashboard", nitro.BFFAggregateHandler(cfg, []nitro.AggregateRequest{
    {Service: "users", Path: "/api/users/me", Key: "user"},
    {Service: "orders", Path: "/api/orders?limit=5", Key: "recent_orders"},
    {Service: "products", Path: "/api/products?featured=true", Key: "featured_products"},
}))
```

Response:

```json
{
    "user": { "id": 1, "name": "Joao" },
    "recent_orders": [ ... ],
    "featured_products": [ ... ]
}
```

## Service Configuration

Define upstream services:

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

## CSRF Protection (Web)

Web BFF includes built-in CSRF protection. The flow:

1. Client fetches a CSRF token: `GET /web/auth/csrf`
2. Token is included in subsequent state-changing requests
3. Server validates the token automatically

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

## Device Tracking (Mobile)

Mobile BFF tracks registered devices per user:

```go
cfg.EnableDeviceTracking = true
cfg.MaxDevicesPerUser = 5

// When a user logs in from a 6th device, the oldest device is automatically removed
```

## Custom Handlers

You can add custom handlers alongside the BFF routes:

```go
app.WebBFFRoutes("/web/auth", auth, cfg, tokenProvider)

// Custom endpoint with BFF middleware
api := app.Group("/api")
api.Use(nitro.JWTMiddleware(tokenProvider))
api.Get("/custom", customHandler)
```

## Example: Mobile BFF with Home Screen Aggregate

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
    cfg.CompactResponse = true

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

## Web vs Mobile Comparison

| Feature | Web BFF | Mobile BFF |
|---------|---------|------------|
| Token delivery | Secure cookies | HTTP header (`Bearer`) |
| CSRF protection | Yes | No (stateless) |
| Token TTL | 15 min / 7 days | 30 min / 30 days |
| Device tracking | No | Yes (max 5) |
| Compact response | No | Yes |
| Push notifications | No | Yes |

## Full Example

See [`examples/bff-web/`](../../examples/bff-web/main.go) and [`examples/bff-mobile/`](../../examples/bff-mobile/main.go) for complete working examples.
