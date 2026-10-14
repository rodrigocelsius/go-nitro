# Context — cheatsheet

**Idioma:** [Português](context.md) · [English](../../guide/context.md)

Referência rápida do `*nitro.Ctx`. Detalhe de API: [`docs/api/context.md`](../api/context.md).

`*nitro.Ctx` implementa `context.Context` — passe `c` a Store/repos **dentro do handler**.

> **Aviso de ciclo de vida.** `*nitro.Ctx` é reutilizado (pool) entre requisições. Nunca guarde `c`
> após o retorno do handler (goroutines, canais, caches, jobs): ele leria dados de outra requisição.
> Para trabalho assíncrono use `ctx := c.Detach()` — um `context.Context` que mantém os valores da
> requisição mas não é cancelado quando ela termina — e copie antes os valores necessários
> (`id := c.Param("id")`).

## Entrada

| Método | Retorna |
|--------|---------|
| `Param` / `ParamInt` / `ParamInt64` / `ParamFloat` | path (`Param` aceita default) |
| `Int64Param` | `(int64, error)` path obrigatório |
| `Query` / `Int` / `IntE` / `Float` / `Bool` / `QuerySlice` / `Sort` | query |
| `Header` / `Cookie` / `FormValue` / `FormFile` | meta / form |
| `BodyParser` / `BodyJSON` / `Bind` / `BindForm` / `Body` | body (CT-aware) |
| `nitro.Parse[T](c)` | tipado + validate (JSON ou form) |

## Saída

| Método | Status |
|--------|--------|
| `OK` / `Created` / `Accepted` / `NoContent` / `Paginated` | 2xx |
| `JSON` / `Text` / `Bytes` / `File` / `Stream` / `Redirect` | variado |
| `BadRequest` / `Unauthorized` / `Forbidden` / `NotFound` / `Conflict` / `UnprocessableEntity` / `InternalError` / `Error` | 4xx/5xx |

## Utilitários

| Método | Uso |
|--------|-----|
| `Method` / `Path` / `RequestID` | request meta |
| `ClientIP` / `IP` | cliente |
| `Context` / `RequestContext` | `context.Context` |
| `Detach` | valores da requisição, sem cancelamento — seguro p/ goroutines |
| `Protocol` / `Hostname` / `BaseURL` | URL |
| `GetRouteURL` / `URL` | reverse routing (rota nomeada) |
| `IsWebSocket` / `IsJSON` / `IsGet`…`IsPatch` | predicates |
| `Set` / `Get` | request-scoped |
| `SetHeader` / `SetStatus` / `SetCookie` / `DeleteCookie` | response |
| `Claims` / `UserID` / `UserUUID` / `UserInt64` | JWT (com middleware) |
| `Log` | logger contextual |

## Ver também

- [Request](request.md) · [Responses](responses.md) · [philosophy](philosophy.md)
