# Referência da API

**Idioma:** [Português](README.md) · [English](../../api/README.md)

Documentação completa da API do framework Nitro.

## Import

```go
import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
```

## App

### `nitro.New(cfg ...Config) *App`

Cria uma nova aplicação.

```go
app := nitro.New()                                    // secure defaults
app := nitro.New()
```

### Métodos de App

| Method | Descrição |
|--------|-------------|
| `app.Get(path, handler, ...mw)` | Registra rota GET |
| `app.GetDomain(path, handler, ...mw)` | GET sem wrapHandler (`AsCtx` no handler) |
| `app.Post(path, handler, ...mw)` | Registra rota POST |
| `app.PostDomain` / `PutDomain` / `PatchDomain` / `DeleteDomain` | Idem sem wrap |
| `app.Put(path, handler, ...mw)` | Registra rota PUT |
| `app.Delete(path, handler, ...mw)` | Registra rota DELETE |
| `app.Patch(path, handler, ...mw)` | Registra rota PATCH |
| `app.Head(path, handler, ...mw)` | Registra rota HEAD |
| `app.Options(path, handler, ...mw)` | Registra rota OPTIONS |
| `app.Use(handler)` | Registra pre-middleware global |
| `app.UsePost(handler)` | Registra post-middleware global |
| `app.Group(prefix, ...mw)` | Cria grupo de rotas |
| `app.Mount(prefix, sub)` | Monta sub-app sob um prefixo |
| `app.Static(prefix, root, cfg...)` | Arquivos estáticos (`StaticConfig{SinglePageApp: true}`) |
| `app.Resource(prefix, controller, ...repo)` | Gera rotas CRUD automaticamente |
| `app.Routes()` | Lista todas as rotas registradas |
| `app.GetRouteURL` / `app.URL` | Reverse routing (rotas nomeadas) |
| `app.Hooks()` | Lifecycle: `OnListen` / `OnShutdown` / `OnRoute` |
| `app.Handler()` | Obtém o handler fasthttp cru |
| `app.Test(method, path, body)` | Teste HTTP in-process |
| `app.Listen(addr, cfg...)` | Sobe o servidor HTTP (graceful shutdown por padrão) |
| `app.ListenTLS(addr, cert, key, cfg...)` | Sobe o servidor HTTPS (graceful por padrão) |
| `app.ListenAutoTLS(addr, domains...)` | Sobe com Let's Encrypt automático (graceful por padrão) |
| `app.EnableTLS(cfg)` | Aplica config TLS |
| `app.Shutdown()` | Graceful shutdown |
| `app.Defaults()` | Reaplica defaults se `DisableDefaults` estava setado |
| `app.UseStack(stacks...)` | Aplica `APIStack` / `PublicStack` |
| `app.HealthCheck(checker)` | Registra health check |
| `app.HealthRoutes()` | Registra rotas de health |

## Context

Guia completo: [guide/context.md](../guide/context.md) · [guide/request.md](../guide/request.md) · [guide/responses.md](../guide/responses.md).

### Request

| Method | Descrição |
|--------|-------------|
| `c.Param` / `ParamInt` / `ParamInt64` / `Int64Param` | Path (`Param` com default) |
| `c.Query` / `Int` / `Float` / `Bool` / `Sort` | Query |
| `c.Header` / `Cookie` | Meta |
| `c.BodyParser` / `BodyJSON` / `Bind` / `BindForm` / `nitro.Parse[T]` | Body (CT-aware) |
| `c.GetRouteURL` / `c.URL` | Reverse routing |

### Response

| Method | Descrição |
|--------|-------------|
| `c.OK` / `Created` / `NoContent` | JSON 2xx |
| `c.Text` / `JSON` / `File` / `Stream` | Outros |
| `c.NotFound` / `BadRequest` / … | Erros no Ctx |

## Errors

Sucesso → métodos do **context** (`c.OK`). Construtores `nitro.NotFound` etc. para erros de domínio.

| Função | Status |
|----------|--------|
| `c.OK` / `c.Created` / `c.NoContent` | 200 / 201 / 204 |
| `nitro.BadRequest` / `c.BadRequest` | 400 |
| `nitro.Unauthorized` / `c.Unauthorized` | 401 |
| `nitro.Forbidden` / `c.Forbidden` | 403 |
| `nitro.NotFound` / `c.NotFound` | 404 |
| `nitro.Conflict` / `c.Conflict` | 409 |
| `nitro.UnprocessableEntity` / `c.UnprocessableEntity` | 422 |
| `nitro.InternalError` / `c.InternalError` | 500 |
| `nitro.Err` / `ErrCode` | erros com código |

## Types

### `nitro.Map`

Atalho para `map[string]any`:

```go
return c.JSON(200, nitro.Map{
    "status": "ok",
    "data":   users,
})
```

### `nitro.Handler`

Assinatura do handler de rota: `func(c *nitro.Ctx) error`

### `nitro.Config`

Configuração da aplicação (veja [Configuration](config.md)).

### `nitro.User`

Interface de usuário autenticável:

```go
type User interface {
    GetID() string
    GetEmail() string
    GetPasswordHash() string
    GetRoles() []string
}
```

### `nitro.Claims`

Claims JWT com `UserID`, `Email`, `Roles`.

### `nitro.TokenPair`

Par access + refresh token:

```go
type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
}
```

## Helpers de texto / identidade

| Função | Descrição |
|----------|-------------|
| `Digits` | Mantém ASCII 0-9 |
| `IsDigits` | String numérica não vazia |
| `SplitName` | Primeiro token + restante |
| `NormalizeEmail` / `IsEmail` | Canonicaliza / mesmas regras de `validate:"email"` |
| `RandomDigits` / `RandomAlphaNum` / `RandomHex` | Códigos com `crypto/rand` |
| `IfBlank` | Fallback se em branco |
| `Ptr` / `Deref` / `Contains` | Helpers de ponteiro e slice |

### `nitro.ResourceListResult`

Resultado paginado:

```go
type ResourceListResult struct {
    Items      []any `json:"items"`
    Total      int   `json:"total"`
    Page       int   `json:"page"`
    PerPage    int   `json:"per_page"`
    TotalPages int   `json:"total_pages"`
}
```
