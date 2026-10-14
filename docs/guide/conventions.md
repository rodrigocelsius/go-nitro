# Conventions (Convention over Configuration)

**Language:** [English](conventions.md)

Nitro works with zero configuration. Every default below is covered by the executable contract in `pkg/coc_contract_test.go`; changing one is a breaking change.

| Area | Convention | Override |
|------|-----------|----------|
| Environment | `Config.Env`, else `NITRO_ENV` (`development`\|`dev`\|`local`), else production; CLI apps use `nitro.ParseEnv(cfg.Env)` | `Config.Env` |
| Resource path | plural, snake_case of the model / `TableName()` (`Person` → `/people`) | `ResourceConfig.Path`, `nitro.RegisterPlural` |
| Wire names | `json` / `db` default to snake_case of the Go field | explicit tags |
| Multi-tenant | a model with a `tenant_id` column is auto-scoped (list filter, get/update/delete check, create stamp) | `NoTenantScope: true` |
| Tenant source | JWT claim `tenant_id` (also `tenantId`, `tid`, `org_id`, `organization_id`); `X-Tenant-ID` header is **ignored** | `TenantConfig.AllowHeader` |
| Client IP | `c.ClientIP()` trusts forwarding headers only from private/loopback peers (works behind nginx/ALB/Docker out of the box); uses the rightmost untrusted `X-Forwarded-For` hop | `Config.TrustedProxies` (CIDRs/IPs) |
| Rate limit | keyed by `c.ClientIP()` | `nitro.RateLimitWith` |
| Security headers | on by default | `Config.DisableDefaults` |
| Request ID | UUIDv7 (time-ordered) in `X-Request-ID` | send your own header |
| Errors | `{"error": "..."}`; opt in to RFC 9457 with `nitro.ProblemDetails()` | `Config.ErrorHandler` |
| Config mistakes | never panic mid-chain: collected and returned by `app.Err()` / `Listen` | — |
| Typed handlers / `BindQuery` | path and query params bind to input fields by JSON (snake_case) name; body by JSON | struct tags |
| Logs | one format for HTTP access + DB lines: `NITRO_LOG_FORMAT=text` (default, `[LEVEL] - date - …`) or `json` (slog shape: `time`/`level`/`msg`); `NITRO_LOG_LEVEL` (default `info`); access level follows status (4xx WARN, 5xx ERROR); DB queries are DEBUG, slow ones (`NITRO_DB_SLOW_MS`, default 200) WARN | `LoggerConfig.Format`, `gateway.WithLogger` / `WithSlowQuery` |
| Wrong method | a path registered for other methods answers **405** with an `Allow` header (unknown paths stay 404) | — |

## List filters (Resource)

For every `Filterable` field: `?name=Pen` (equality), `?unit_price_gte=10&unit_price_lt=50`, `?name_contains=pe`, `?name_icontains=PE` (suffixes `gt gte lt lte contains icontains`). `FilterRules` add named params (`?q=`). `?sort=-unit_price` (numeric-aware). The in-memory repo applies exactly the same operators as the SQL stores.

## One-line middleware

```go
app.CORS().Compress().Timeout(10 * time.Second).RateLimit(100, time.Minute)
app.ETag().ResponseTime()          // call ETag() before Compress()
app.Use(nitro.RedirectHTTPS())     // 308, loopback excluded, proxy-aware
app.Get("/admin", h, nitro.BasicAuth(map[string]string{"admin": pw}))
app.Get("/v1/x", h, nitro.APIKeyAuth(key))   // X-API-Key or Bearer
app.Use(nitro.Skip(func(c *nitro.Ctx) bool { return c.Path() == "/health" }, auth))
```

## Migration from the removed names

| Removed | Use instead |
|---------|-------------|
| `nitro.CORS/Compress/Timeout(...)` | `app.CORS()`, `app.Compress()`, `app.Timeout(d)` (or `CORSMiddleware` etc. for per-route use) |
| `nitro.OK/Created/NoContent` helpers | `c.OK`, `c.Created`, `c.NoContent` |
| `MapAny`, `SerialID`, `WithCache`, `ResourceOpts` option structs | `*ResourceConfig` (cache defaults applied automatically) |
