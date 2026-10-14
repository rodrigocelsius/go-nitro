# FAQ

**Idioma:** [Português](faq.md) · [English](../../extra/faq.md)

## Geral

### O que é o Nitro?

O Nitro é um framework web Go de alta performance, construído sobre [fasthttp](https://github.com/valyala/fasthttp). Segue os princípios de Clean Architecture, com separação estrita de camadas, e inclui middleware, autenticação, padrões BFF e a camada de dados BLinq.

### Como o Nitro se compara ao Fiber?

Ambos os frameworks são construídos sobre fasthttp. O Nitro se diferencia por:
- Separação de camadas **Clean Architecture**
- **Auth embutida** (JWT, sessions, OAuth2)
- **Padrões BFF** (Web e Mobile)
- **Camada de dados BLinq** com queries type-safe
- **Code generation** para rotas CRUD, JSON, SQL

### Qual versão do Go eu preciso?

- **Todos os módulos:** Go **1.25+**

### Como faço CRUD + SQL com o mínimo de comandos?

```bash
nitrogen new shop --module github.com/acme/shop
cd shop
nitrogen install sqlite   # ou postgresql
nitrogen make:scaffold Product name:string price:float64 active:bool --resource
nitrogen up
```

Ver [guia da CLI](../guide/cli.md) e [matriz de exemplos](../../examples/README.md). Itens de ergonomia entregues: [`ERGONOMICS_ROADMAP.md`](../../ERGONOMICS_ROADMAP.md) (fases 0–5).

### Como abro o banco?

Prefira **`sqladapter.OpenEnv`** / **`platform.Open`** (depois de `nitrogen install …`). Postgres ainda usa pgx por baixo — não abra pools crus no app. Guia: [data layer](../guide/data-layer.md).

## Instalação

### Como instalo o Nitro?

```bash
go get github.com/rodrigocelsius/go-nitro/pkg/v2
```

Os módulos Go ficam em `github.com/rodrigocelsius/go-nitro/<módulo>/v2` (ex.: `pkg/v2`, `adapters/fasthttp/v2`). Código: [github.com/rodrigocelsius/go-nitro](https://github.com/rodrigocelsius/go-nitro).

### Posso usar o Nitro com `go mod`?

Sim. Adicione o import e rode `go mod tidy`:

```go
import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
```

## Roteamento

### Como adiciono parâmetros de rota?

Use a sintaxe `:param`:

```go
app.Get("/users/:id", func(c *nitro.Ctx) error {
    id := c.Param("id")
    return c.OK(nitro.Map{"id": id})
})
```

### Quantos parâmetros posso ter?

No máximo 8 parâmetros por rota (`nitro.MaxParams`).

### Como crio grupos de rotas?

```go
api := app.Group("/api")
api.Get("/users", listHandler)
api.Post("/users", createHandler)
```

### Como gero rotas CRUD automaticamente?

Use `app.Resource()`:

```go
app.Resource("/users", &User{}, userRepo)
// Generates: GET, GET/:id, POST, PATCH/:id, DELETE/:id (cache 5m by default)
```

## Middleware

### Como adiciono middleware?

```go
// Global middleware
app.Use(myMiddleware)

// Route-specific middleware
app.Get("/admin", adminHandler, authMiddleware)

// Group middleware
api := app.Group("/api", authMiddleware)
```

### Qual middleware vem embutido?

- CORS
## Middleware embutido

Ativo por padrão no `nitro.New()`: security headers, RequestID, Recover, Logger, MaxBody (CORS em development).

Stacks / middleware opt-in:

- Compress (gzip) — `APIStack` ou `UsePost(Compress())`
- RateLimit — `APIStack` ou `RateLimit(n, window)`
- Timeout — `Config.RequestTimeout` / `TimeoutMiddleware`
- JWT / Session / CSRF / IPWhiteList — no nível da rota ou do BFF

Veja o [guia de middleware](../guide/middleware.md).

## Autenticação

### Como uso JWT?

```go
tokenProvider := nitro.JWTProvider(nitro.AuthConfig{
    JWTSecret:      "at-least-32-characters-long-random-secret",
    AccessTokenTTL: 15 * time.Minute,
})

app.Use(nitro.JWTMiddleware(tokenProvider))
```

### Como obtenho o usuário atual?

```go
claims, err := c.Claims()
userID := c.UserID()
```

### Como uso OAuth2?

```go
google := nitro.GoogleOAuth(nitro.OAuthConfig{
    ClientID:     "your-client-id",
    ClientSecret: "your-client-secret",
    RedirectURL:  "http://localhost:8080/auth/google/callback",
})

app.Get("/auth/google", nitro.OAuthRedirectHandler(google))
app.Get("/auth/google/callback", nitro.OAuthCallbackHandler(google, auth))
```

## Tratamento de erros

### Como devolvo erros?

```go
return nitro.NotFound("user not found")
return nitro.BadRequest("invalid json")
return nitro.InternalError(err)
```

### Quais construtores de erro existem?

Sucesso: `c.OK` / `c.Created` / `c.NoContent`. Erros:

| Função | Status |
|----------|--------|
| `c.BadRequest` / `nitro.BadRequest` | 400 |
| `c.Unauthorized` / `nitro.Unauthorized` | 401 |
| `c.Forbidden` / `nitro.Forbidden` | 403 |
| `c.NotFound` / `nitro.NotFound` | 404 |
| `c.Conflict` / `nitro.Conflict` | 409 |
| `c.UnprocessableEntity` / `nitro.UnprocessableEntity` | 422 |
| `c.InternalError` / `nitro.InternalError` | 500 |

## Testes

### Como testo minha app?

Use `app.Test()` para testes in-process:

```go
func TestGetUser(t *testing.T) {
    app := nitro.New()
    app.Get("/users/:id", getUserHandler)
    
    req := httptest.NewRequest("GET", "/users/42", nil)
    resp, err := app.Test(req)
    
    if err != nil {
        t.Fatal(err)
    }
    
    if resp.StatusCode != 200 {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }
}
```

## Desempenho

### Como ativo compressão?

```go
app.Use(nitro.CompressMiddleware(nitro.CompressConfig{
    Level: 5,
}))
```

### Como otimizo para produção?

1. Defina timeouts adequados:
```go
app := nitro.New(nitro.Config{
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
    IdleTimeout:  120 * time.Second,
})
```

2. Ative security headers:
```go
app.Use(nitro.SecurityMiddleware())
```

3. Use o compiled router para performance extrema:
```go
//go:generate go run gen_router.go
```

## Deploy

### Como faço deploy em produção?

1. Use graceful shutdown:
```go
app.Listen(":8080", nitro.GracefulShutdownConfig{
    ShutdownTimeout: 10 * time.Second,
})
```

2. Configure health checks:
```go
app.HealthCheck(func() error {
    return db.Ping()
})
```

3. Use variáveis de ambiente para segredos:
```go
app := nitro.New(nitro.Config{
    // ...
})
```

### Como uso Docker?

Para apps SQL use imagem de build **Go 1.25+**:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
CMD ["./server"]
```

## Depreciado / prefira isto

| Evitar | Preferir |
|--------|----------|
| `nitrogen g resource` / `make:migration` | `make:scaffold` / `migrate:make` |
| `nitro.OK` / `nitro.Created` para sucesso | `c.OK` / `c.Created` |
| Abrir pools pgx / `database/sql` no app | `sqladapter.OpenEnv` / `platform.Open` |
| Pasta `examples/v1/db-linq` como padrão | `db-blinq` / `blinq-hello` |
| Parser caseiro de `.env` | `nitro.ConfigOf` |
