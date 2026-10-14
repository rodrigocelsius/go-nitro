# Context (Ctx) API Reference

**Idioma:** [Português](context.md) · [English](../../api/context.md)

`Ctx` é o contexto por request. Guia temático: [`../guide/context.md`](../guide/context.md) · [`../guide/request.md`](../guide/request.md) · [`../guide/responses.md`](../guide/responses.md).

```go
type Ctx = nitroadapter.FastCtx
```

`*nitro.Ctx` implementa `context.Context` e é **reutilizado (pool)**: não o retenha após o handler. Use `c.Detach()` (mantém valores, sem cancelamento) em goroutines/background.

## Request

| Método | Descrição |
|--------|-----------|
| `Param` / `ParamInt` / `ParamInt64` / `ParamFloat` | Path (`Param(name, default?)`) |
| `Int64Param` | Path int64 obrigatório |
| `Query` / `Int` / `IntE` / `Float` / `Bool` / `QuerySlice` / `Sort` | Query |
| `Header` / `Cookie` | Meta (`Header` aceita default) |
| `Body` / `BodyParser` / `BodyJSON` / `Bind` / `BindForm` / `FormValue` / `FormFile` | Body / form (CT-aware) |
| `nitro.Parse[T](c)` | Bind + validate tipado (JSON ou form) |
| `Method` / `Path` / `RequestID` / `ClientIP` / `IP` | Meta |
| `Protocol` / `Hostname` / `BaseURL` | URL |
| `GetRouteURL` / `URL` | Reverse routing |
| `IsJSON` / `IsGet`…`IsPatch` / `IsWebSocket` | Predicates |
| `Claims` / `UserID` / `UserUUID` / `UserInt64` | JWT |

Aliases legados (se existirem no adapter): preferir `Query` em vez de `QueryParam`, `Header` em vez de `HeaderParam`.

## Response

| Método | Status |
|--------|--------|
| `OK` / `Created` / `Accepted` / `NoContent` / `Paginated` | 2xx |
| `JSON` / `Text` / `Bytes` / `File` / `Stream` / `Redirect` | variado |
| `BadRequest` … `InternalError` / `Error` | 4xx/5xx |
| `SetHeader` / `SetStatus` / `SetCookie` / `DeleteCookie` | meta |

Preferir `c.Text` a `SendString`; `c.OK` a `nitro.OK` para sucesso.

## Context store

| Método | Descrição |
|--------|-----------|
| `Context` / `RequestContext` | `context.Context` |
| `Get` / `Set` | valores request-scoped |
| `Log` | logger contextual |
| `Next` | chain de middleware |

## Ver também

- [Guide index](../guide/README.md) · [pkg facade](../api/README.md) · [Config](config.md)
