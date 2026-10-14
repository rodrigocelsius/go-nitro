# Changelog de API (legado → v1)

**Idioma:** [Português](api-changelog.md) · [English](../../guide/api-changelog.md)

Resumo das mudanças de API em relação ao Nitro pré-1.0. Filosofia: [philosophy.md](philosophy.md).

| legado | v1 |
|--------|----|
| `c.QueryIntDefault` / `QueryFloat64Default` / `QueryBool` | `c.Int` / `Float` / `Bool` |
| `c.BindJSON` | `Parse` / `BodyParser` / `BodyJSON` (`BindJSON` = alias deprecated) |
| `c.SendString` | `c.Text` |
| `nitro.OK` / `Created` / `NoContent` (sucesso) | `c.OK` / `c.Created` / `c.NoContent` |
| `nitro.CORSMiddleware` / `CompressMiddleware` / `TimeoutMiddleware` | `CORS` / `Compress` / `Timeout` |
| — | `c.Accepted`, `File`, `Stream`, `Bytes`, `Sort`, `Paginated` |
| — | `c.ParamInt` / `ParamInt64` / `ParamFloat` / `Int64Param` / `IntE` / `QuerySlice` |
| — | `c.Param` / `Query` / `Header` com default variádico |
| — | `nitro.Parse[T]`, `BodyParser`, `Err` / `ErrCode`, `WebSocketHandler`, `SSECreate`, `NewWSHub` |
| — | `app.Mount`, `app.Static` + `SinglePageApp`, `app.Hooks`, `GetRouteURL` / `URL` |
| — | `app.Logger`, `Dashboard`, `Prometheus`, `Metrics`, `Schedule`, `HealthCheck` |
| — | `*Ctx` implementa `context.Context` |
| `data/runtime` (LINQ) | **removido** → [BLinq](blinq.md) |

## Helpers úteis (v1+)

```go
id := nitro.NextIDString()
id := nitro.NewUUIDString()
req, err := nitro.Parse[LoginRequest](c) // JSON ou form
app.Get("/users/:id", h).Name("user.show")
href := app.GetRouteURL("user.show", nitro.Map{"id": 42})
app.Mount("/api/v1", v1App)
app.Static("/", "./public", nitro.StaticConfig{SinglePageApp: true})
app.Hooks().OnShutdown(func() error { return db.Close() })
cfg := nitro.MustConfig[struct {
    Port string `env:"PORT" default:"8080"`
}]()
app.Schedule("*/5 * * * *", func(ctx context.Context) error { return cleanup(ctx) })
tc := nitro.NewTestClient(app)
```

## Ver também

- [AI Playbook](../ai/PLAYBOOK.md) · [Benchmarks](../extra/benchmarks.md)
