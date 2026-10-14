# Authentication

**Idioma:** [Português](authentication.md) · [English](../../guide/authentication.md)

JWT, sessions, OAuth2 e RBAC. Exemplo canônico: [`examples/v1/auth`](../../examples/v1/auth/).

## JWT (padrão recomendado)

```go
auth := nitro.NewAuth(nitro.AuthConfig{
    JWTSecret:       os.Getenv("JWT_SECRET"), // generate with: openssl rand -hex 32
    AccessTokenTTL:  15 * time.Minute,
    RefreshTokenTTL: 7 * 24 * time.Hour,
}).WithMemoryStore()

app := nitro.New()
app.AuthRoutes("/auth", auth) // register, login, refresh, …

api := app.Group("", nitro.JWTMiddleware(auth.Provider()))
api.Get("/profile", func(c *nitro.Ctx) error {
    claims, err := c.Claims()
    if err != nil {
        return err
    }
    // Tipados (quando o subject é UUID / int):
    // uid, err := c.UserUUID()
    // id, err := c.UserInt64()
    return c.OK(nitro.Map{"user_id": c.UserID(), "email": claims.Email})
})

admin := app.Group("/admin",
    nitro.JWTMiddleware(auth.Provider()),
    nitro.RequireRole("admin"),
)
admin.Get("/dashboard", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"user_id": c.UserID()})
})
```

## Session

```go
store := nitro.MemorySessionStore()
app.Use(nitro.SessionMiddleware(store, "session_id"))

app.Post("/login", func(c *nitro.Ctx) error {
    session := c.Get("session").(*nitro.Session)
    session.Set("user_id", "123")
    return c.OK(nitro.Map{"status": "logged in"})
})
```

## OAuth2

```go
google := nitro.GoogleOAuth(nitro.OAuthConfig{
    ClientID:     "…",
    ClientSecret: "…",
    RedirectURL:  "http://localhost:8080/auth/google/callback",
    Scopes:       []string{"openid", "email", "profile"},
})
app.Get("/auth/google", nitro.OAuthRedirectHandler(google))
app.Get("/auth/google/callback", nitro.OAuthCallbackHandler(google, auth))

github := nitro.GitHubOAuth(nitro.OAuthConfig{ /* … */ })
app.Get("/auth/github", nitro.OAuthRedirectHandler(github))
app.Get("/auth/github/callback", nitro.OAuthCallbackHandler(github, auth))
```

## RBAC

```go
api.Get("/reports", reportsHandler, nitro.RequireRole("admin", "manager"))
```

## OIDC (JWKS)

Valida JWT RS256 de qualquer IdP com JWKS. Roles de `realm_access` / `resource_access` são achatadas em `Claims.Roles`.

```go
provider, err := nitro.OIDCProvider(nitro.OIDCConfig{
    Issuer:    "https://auth.example/realms/demo",
    JWKSURL:   "https://auth.example/realms/demo/protocol/openid-connect/certs",
    ClientIDs: []string{"my-api"},
})
api := app.Group("/api", nitro.JWTMiddleware(provider))
```

## Keycloak

Password grant + Admin API + JWKS. Exemplo: [`examples/v1/auth-keycloak`](../../examples/v1/auth-keycloak/).

```go
kc, err := nitro.NewKeycloak(nitro.KeycloakConfig{
    BaseURL:      "http://localhost:8080",
    Realm:        "scheduler",
    ClientID:     "scheduler-api",
    ClientSecret: "…",
})
api := app.Group("/api", nitro.JWTMiddleware(kc.Provider()))

app.Post("/auth/login", func(c *nitro.Ctx) error {
    body, err := nitro.Parse[struct {
        Username string `json:"username" validate:"required"`
        Password string `json:"password" validate:"required"`
    }](c)
    if err != nil {
        return err
    }
    tokens, err := kc.Tokens().Login(c, body.Username, body.Password)
    if err != nil {
        return err
    }
    return c.OK(tokens)
})
```

## Cache Redis (L2)

```go
rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
cache := nitro.RedisCache(rdb, nitro.WithRedisDefaultTTL(10*time.Minute))
_ = cache.Set("drivers:all", drivers, 0)
_ = cache.Invalidate("drivers:*")
```

## CSRF

```go
app.Use(nitro.StandaloneCSRFMiddleware())
app.Get("/csrf-token", nitro.StandaloneCSRFTokenHandler())
```

## Ver também

- [Recipes](recipes.md) · [Middleware](middleware.md) · [AI Playbook](../ai/PLAYBOOK.md)
