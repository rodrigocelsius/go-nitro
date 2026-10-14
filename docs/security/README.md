# Security notes

See also [`AUDIT.md`](AUDIT.md).

## Diagnostics endpoints (pprof, dashboard)

`SetupPprof(app, authMiddleware...)` and `app.Dashboard(prefix, authMiddleware...)` expose runtime internals. In production (`NITRO_ENV=production`) they **panic at startup unless at least one auth middleware is passed**. Example:

```go
admin := jwtMiddleware // any nitro.Handler that restricts access to operators
nitroadapter.SetupPprof(fastApp, admin) // fastApp = the adapter *App behind nitro.App
app.Dashboard("/_nitro", admin)
```

Profiles are capped at 30 s and one profile may run at a time.
