# Authentication

**Language:** [English](authentication.md) · [Português](../pt/guide/authentication.md)

JWT, sessions, OAuth2, and RBAC. Canonical example: [`examples/v1/auth`](../../examples/v1/auth/).

## JWT (recommended default)

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
    // Typed (when the subject is UUID / int):
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

`SessionMiddleware` **requires** a valid session cookie and returns 401
when there isn't one — never register it globally with `app.Use`, or it
blocks `/login` itself before the handler runs. Apply it only to a `Group`
of already-authenticated routes, and create the session in the login
handler with `nitro.LoginSession`, which **always mints a brand-new ID**
(never one taken from an incoming cookie). Reusing a pre-login session ID
is session fixation (CWE-384): an attacker who plants a known ID in the
victim's browser before they log in would otherwise inherit the
now-authenticated session.

```go
store := nitro.MemorySessionStore()

app.Post("/login", func(c *nitro.Ctx) error {
    user, err := verifyCredentials(c) // your own password check
    if err != nil {
        return domain.ErrInvalidCredentials
    }
    // Always issues a fresh, crypto/rand session ID — never reuses
    // whatever (if anything) the client's "session_id" cookie held.
    if _, err := nitro.LoginSession(c, store, user.ID, "session_id", 24*time.Hour); err != nil {
        return err
    }
    return c.OK(nitro.Map{"status": "logged in"})
})

app.Post("/logout", func(c *nitro.Ctx) error {
    if err := nitro.LogoutSession(c, store, "session_id"); err != nil {
        return err
    }
    return c.NoContent()
})

// Only protected routes require an existing session.
protected := app.Group("", nitro.SessionMiddleware(store, "session_id"))
protected.Get("/profile", func(c *nitro.Ctx) error {
    return c.OK(nitro.Map{"user_id": c.UserID()})
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

`OAuthRedirectHandler`/`OAuthCallbackHandler` automatically use **PKCE** (RFC 7636, `S256`) for any provider built via `GoogleOAuth`/`GitHubOAuth`/`oauth2.New` — a random `code_verifier` is stored in an `HttpOnly` cookie alongside `state` and presented at token exchange, so an intercepted authorization `code` alone is not enough to redeem a token. No extra configuration needed.

## RBAC

```go
api.Get("/reports", reportsHandler, nitro.RequireRole("admin", "manager"))
```

## OIDC (JWKS)

Validates RS256 JWTs from any IdP with JWKS. Roles from `realm_access` / `resource_access` are flattened into `Claims.Roles`.

```go
provider, err := nitro.OIDCProvider(nitro.OIDCConfig{
    Issuer:    "https://auth.example/realms/demo",
    JWKSURL:   "https://auth.example/realms/demo/protocol/openid-connect/certs",
    ClientIDs: []string{"my-api"},
})
api := app.Group("/api", nitro.JWTMiddleware(provider))
```

## Keycloak

Password grant + Admin API + JWKS. Example: [`examples/v1/auth-keycloak`](../../examples/v1/auth-keycloak/).

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

## Redis cache (L2)

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

## See also

- [Recipes](recipes.md) · [Middleware](middleware.md) · [AI Playbook](../ai/PLAYBOOK.md)
