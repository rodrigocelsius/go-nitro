# Context (Ctx) API Reference

**Language:** [English](context.md) · [Português](../pt/api/context.md)

`Ctx` is the per-request context. Thematic guide: [`../guide/context.md`](../guide/context.md) · [`../guide/request.md`](../guide/request.md) · [`../guide/responses.md`](../guide/responses.md).

```go
type Ctx = nitroadapter.FastCtx
```

`*nitro.Ctx` implements `context.Context`. It is **pooled**: do not retain it after the handler returns. Use `c.Detach()` (values kept, not cancelled) for goroutines/background work.

## Request

| Method | Description |
|--------|-----------|
| `Param` / `ParamInt` / `ParamInt64` / `ParamFloat` | Path (`Param(name, default?)`) |
| `Int64Param` | Required path int64 |
| `Query` / `Int` / `IntE` / `Float` / `Bool` / `QuerySlice` / `Sort` | Query |
| `Header` / `Cookie` | Meta (`Header` accepts a default) |
| `Body` / `BodyParser` / `BodyJSON` / `Bind` / `BindForm` / `FormValue` / `FormFile` | Body / form (CT-aware) |
| `nitro.Parse[T](c)` | Typed bind + validate (JSON or form) |
| `Method` / `Path` / `RequestID` / `ClientIP` / `IP` | Meta |
| `Protocol` / `Hostname` / `BaseURL` | URL |
| `GetRouteURL` / `URL` | Reverse routing |
| `IsJSON` / `IsGet`…`IsPatch` / `IsWebSocket` | Predicates |
| `Claims` / `UserID` / `UserUUID` / `UserInt64` | JWT |

Legacy aliases (if present on the adapter): prefer `Query` over `QueryParam`, `Header` over `HeaderParam`.

## Response

| Method | Status |
|--------|--------|
| `OK` / `Created` / `Accepted` / `NoContent` / `Paginated` | 2xx |
| `JSON` / `Text` / `Bytes` / `File` / `Stream` / `Redirect` | various |
| `BadRequest` … `InternalError` / `Error` | 4xx/5xx |
| `SetHeader` / `SetStatus` / `SetCookie` / `DeleteCookie` | meta |

Prefer `c.Text` over `SendString`; `c.OK` over `nitro.OK` for success.

## Context store

| Method | Description |
|--------|-----------|
| `Context` / `RequestContext` | `context.Context` |
| `Get` / `Set` | request-scoped values |
| `Log` | contextual logger |
| `Next` | middleware chain |

## See also

- [Guide index](../guide/README.md) · [pkg facade](README.md) · [Config](config.md)
