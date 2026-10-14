# API Reference

**Language:** [English](README.md) · [Português](../pt/api/README.md)

Complete API documentation for the Nitro framework.

## Import

```go
import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
```

## App

### `nitro.New(cfg ...Config) *App`

Create a new application.

```go
app := nitro.New()                                    // secure defaults
app := nitro.New()
```

### App Methods

| Method | Description |
|--------|-------------|
| `app.Get(path, handler, ...mw)` | Register GET route |
| `app.GetDomain(path, handler, ...mw)` | GET without wrapHandler (`AsCtx` in the handler) |
| `app.Post(path, handler, ...mw)` | Register POST route |
| `app.PostDomain` / `PutDomain` / `PatchDomain` / `DeleteDomain` | Same without wrap |
| `app.Put(path, handler, ...mw)` | Register PUT route |
| `app.Delete(path, handler, ...mw)` | Register DELETE route |
| `app.Patch(path, handler, ...mw)` | Register PATCH route |
| `app.Head(path, handler, ...mw)` | Register HEAD route |
| `app.Options(path, handler, ...mw)` | Register OPTIONS route |
| `app.Use(handler)` | Register global pre-middleware |
| `app.UsePost(handler)` | Register global post-middleware |
| `app.Group(prefix, ...mw)` | Create route group |
| `app.Mount(prefix, sub)` | Mount sub-app under prefix |
| `app.Static(prefix, root, cfg...)` | Static files (`StaticConfig{SinglePageApp: true}`) |
| `app.Resource(prefix, controller, ...repo)` | Auto-generate CRUD routes |
| `app.Routes()` | List all registered routes |
| `app.GetRouteURL` / `app.URL` | Reverse routing (named routes) |
| `app.Hooks()` | Lifecycle: `OnListen` / `OnShutdown` / `OnRoute` |
| `app.Handler()` | Get raw fasthttp handler |
| `app.Test(method, path, body)` | In-process HTTP test |
| `app.Listen(addr, cfg...)` | Start HTTP server (graceful shutdown by default) |
| `app.ListenTLS(addr, cert, key, cfg...)` | Start HTTPS server (graceful by default) |
| `app.ListenAutoTLS(addr, domains...)` | Start with auto Let's Encrypt (graceful by default) |
| `app.EnableTLS(cfg)` | Apply TLS config |
| `app.Shutdown()` | Graceful shutdown |
| `app.Defaults()` | Re-apply defaults if `DisableDefaults` was set |
| `app.UseStack(stacks...)` | Apply `APIStack` / `PublicStack` |
| `app.HealthCheck(checker)` | Register health check |
| `app.HealthRoutes()` | Register health routes |

## Context

Full guide: [guide/context.md](../guide/context.md) · [guide/request.md](../guide/request.md) · [guide/responses.md](../guide/responses.md).

### Request

| Method | Description |
|--------|-------------|
| `c.Param` / `ParamInt` / `ParamInt64` / `Int64Param` | Path (`Param` with default) |
| `c.Query` / `Int` / `Float` / `Bool` / `Sort` | Query |
| `c.Header` / `Cookie` | Meta |
| `c.BodyParser` / `BodyJSON` / `Bind` / `BindForm` / `nitro.Parse[T]` | Body (CT-aware) |
| `c.GetRouteURL` / `c.URL` | Reverse routing |

### Response

| Method | Description |
|--------|-------------|
| `c.OK` / `Created` / `NoContent` | JSON 2xx |
| `c.Text` / `JSON` / `File` / `Stream` | Other |
| `c.NotFound` / `BadRequest` / … | Errors on Ctx |

## Errors

Success → **context** methods (`c.OK`). Constructors `nitro.NotFound` etc. for domain errors.

| Function | Status |
|----------|--------|
| `c.OK` / `c.Created` / `c.NoContent` | 200 / 201 / 204 |
| `nitro.BadRequest` / `c.BadRequest` | 400 |
| `nitro.Unauthorized` / `c.Unauthorized` | 401 |
| `nitro.Forbidden` / `c.Forbidden` | 403 |
| `nitro.NotFound` / `c.NotFound` | 404 |
| `nitro.Conflict` / `c.Conflict` | 409 |
| `nitro.UnprocessableEntity` / `c.UnprocessableEntity` | 422 |
| `nitro.InternalError` / `c.InternalError` | 500 |
| `nitro.Err` / `ErrCode` | coded errors |

## Types

### `nitro.Map`

Shortcut for `map[string]any`:

```go
return c.JSON(200, nitro.Map{
    "status": "ok",
    "data":   users,
})
```

### `nitro.Handler`

Route handler signature: `func(c *nitro.Ctx) error`

### `nitro.Config`

Application configuration (see [Configuration](config.md)).

### `nitro.User`

Authenticable user interface:

```go
type User interface {
    GetID() string
    GetEmail() string
    GetPasswordHash() string
    GetRoles() []string
}
```

### `nitro.Claims`

JWT claims with `UserID`, `Email`, `Roles`.

### `nitro.TokenPair`

Access + refresh token pair:

```go
type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
}
```

## Text / identity helpers

| Function | Description |
|----------|-------------|
| `Digits` | Keep ASCII 0-9 |
| `IsDigits` | Non-empty numeric string |
| `SplitName` | First token + remainder |
| `NormalizeEmail` / `IsEmail` | Canonicalize / same rules as `validate:"email"` |
| `RandomDigits` / `RandomAlphaNum` / `RandomHex` | `crypto/rand` codes |
| `IfBlank` | Fallback if blank |
| `Ptr` / `Deref` / `Contains` | Pointer and slice helpers |

### `nitro.ResourceListResult`

Paginated result:

```go
type ResourceListResult struct {
    Items      []any `json:"items"`
    Total      int   `json:"total"`
    Page       int   `json:"page"`
    PerPage    int   `json:"per_page"`
    TotalPages int   `json:"total_pages"`
}
```
