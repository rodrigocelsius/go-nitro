# API changelog (legacy → v1)

**Language:** [English](api-changelog.md) · [Português](../pt/guide/api-changelog.md)

Summary of API changes relative to pre-1.0 Nitro. Philosophy: [philosophy.md](philosophy.md).

| legacy | v1 |
|--------|----|
| `c.QueryIntDefault` / `QueryFloat64Default` / `QueryBool` | `c.Int` / `Float` / `Bool` |
| `c.BindJSON` | `Parse` / `BodyParser` / `BodyJSON` (`BindJSON` = deprecated alias) |
| `c.SendString` | `c.Text` |
| `nitro.OK` / `Created` / `NoContent` (success) | `c.OK` / `c.Created` / `c.NoContent` |
| `nitro.CORSMiddleware` / `CompressMiddleware` / `TimeoutMiddleware` | `CORS` / `Compress` / `Timeout` |
| — | `c.Accepted`, `File`, `Stream`, `Bytes`, `Sort`, `Paginated` |
| — | `c.ParamInt` / `ParamInt64` / `ParamFloat` / `Int64Param` / `IntE` / `QuerySlice` |
| — | `c.Param` / `Query` / `Header` with variadic default |
| — | `nitro.Parse[T]`, `BodyParser`, `Err` / `ErrCode`, `WebSocketHandler`, `SSECreate`, `NewWSHub` |
| — | `app.Mount`, `app.Static` + `SinglePageApp`, `app.Hooks`, `GetRouteURL` / `URL` |
| — | `app.Logger`, `Dashboard`, `Prometheus`, `Metrics`, `Schedule`, `HealthCheck` |
| — | `*Ctx` implements `context.Context` |
| `data/runtime` (LINQ) | **removed** → [BLinq](blinq.md) |

## Useful helpers (v1+)

```go
id := nitro.NextIDString()
id := nitro.NewUUIDString()
req, err := nitro.Parse[LoginRequest](c) // JSON or form
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

## See also

- [AI Playbook](../ai/PLAYBOOK.md) · [Benchmarks](../extra/benchmarks.md)
