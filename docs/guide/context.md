# Context — cheatsheet

**Language:** [English](context.md) · [Português](../pt/guide/context.md)

Quick reference for `*nitro.Ctx`. API detail: [`docs/api/context.md`](../api/context.md).

`*nitro.Ctx` implements `context.Context` — pass `c` to Store/repos **within the handler**.

> **Lifecycle warning.** `*nitro.Ctx` is pooled and reused across requests. Never keep `c`
> after the handler returns (goroutines, channels, caches, background jobs): it would read
> another request's data. For async work use `ctx := c.Detach()` — a `context.Context` that
> keeps the request values but is not cancelled when the request ends — and copy any
> values you need (`id := c.Param("id")`) before spawning the goroutine.

## Input

| Method | Returns |
|--------|---------|
| `Param` / `ParamInt` / `ParamInt64` / `ParamFloat` | path (`Param` accepts a default) |
| `Int64Param` | `(int64, error)` required path |
| `Query` / `Int` / `IntE` / `Float` / `Bool` / `QuerySlice` / `Sort` | query |
| `Header` / `Cookie` / `FormValue` / `FormFile` | meta / form |
| `BodyParser` / `BodyJSON` / `Bind` / `BindForm` / `Body` | body (CT-aware) |
| `nitro.Parse[T](c)` | typed + validate (JSON or form) |

## Output

| Method | Status |
|--------|--------|
| `OK` / `Created` / `Accepted` / `NoContent` / `Paginated` | 2xx |
| `JSON` / `Text` / `Bytes` / `File` / `Stream` / `Redirect` | various |
| `BadRequest` / `Unauthorized` / `Forbidden` / `NotFound` / `Conflict` / `UnprocessableEntity` / `InternalError` / `Error` | 4xx/5xx |

## Utilities

| Method | Use |
|--------|-----|
| `Method` / `Path` / `RequestID` | request meta |
| `ClientIP` / `IP` | client |
| `Context` / `RequestContext` | `context.Context` |
| `Detach` | request values, no cancellation — safe for goroutines |
| `Protocol` / `Hostname` / `BaseURL` | URL |
| `GetRouteURL` / `URL` | reverse routing (named route) |
| `IsWebSocket` / `IsJSON` / `IsGet`…`IsPatch` | predicates |
| `Set` / `Get` | request-scoped |
| `SetHeader` / `SetStatus` / `SetCookie` / `DeleteCookie` | response |
| `Claims` / `UserID` / `UserUUID` / `UserInt64` | JWT (with middleware) |
| `Log` | contextual logger |

## See also

- [Request](request.md) · [Responses](responses.md) · [philosophy](philosophy.md)
