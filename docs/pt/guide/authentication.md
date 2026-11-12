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

## Limite de tentativas de login

`AuthRoutes` e `KeycloakAuthRoutes` limitam requisições **por IP** (20/min, `auth.WithRateLimit`) e logins com falha **por conta** (10 falhas em 15 min, de qualquer IP — `auth.WithAccountRateLimit(max, window)`). O limite por conta é o que segura uma tentativa de adivinhar senha distribuída por muitos endereços; um login bem-sucedido zera a contagem, e a conta bloqueada responde `429` com `Retry-After`.

## Chaves assimétricas e rotação

HS256 (o padrão) usa um segredo compartilhado: todo serviço que valida tokens também consegue emiti-los. Com chave assimétrica, só o serviço de autenticação tem a chave privada; os demais validam com as chaves públicas publicadas como JWKS.

```go
jwtp, err := nitro.JWTProviderWithKeys(cfg, nitro.JWTKeys{
    Signing: nitro.JWTSigningKey{ID: "2026-11", Key: privateKey}, // RSA ≥ 2048 (RS256), ECDSA P-256 (ES256) ou Ed25519 (EdDSA)
})
auth := nitro.NewAuth(cfg).WithTokenProvider(jwtp).WithStore(users, users)
app.AuthRoutes("/auth", auth)
app.JWKSRoute("/.well-known/jwks.json", jwtp) // somente chaves públicas
```

Rotação sem deslogar ninguém:

```go
jwtp.RotateSigningKey(nitro.JWTSigningKey{ID: "2026-12", Key: newKey}) // tokens novos usam a nova; a antiga continua validando
// depois do RefreshTokenTTL, quando não houver mais token da chave antiga:
jwtp.RetireKey("2026-11")
```

Os tokens levam o id da chave (`kid`); um token com `kid` desconhecido, ou com algoritmo diferente do registrado para aquela chave, é rejeitado. Outros serviços consomem o JWKS com `nitro.OIDCProvider` (abaixo).

Em produção, `AuthRoutes` registra um aviso enquanto as revogações (logout, rotação de refresh) ficam em memória: com mais de uma réplica, use um store compartilhado com `SetRevocationStore(redisrevocation.New(rdb))`.

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
