# Production checklist

**Language:** [English](production.md)

What the framework does for you, what you must set, and what is **not yet verified**.

## Built in
- Security headers, `X-Request-ID` (UUID v7), recover, access log, 1 MiB body cap, graceful shutdown.
- Probes: `/health`, `/ready`, `/live` (register checkers with `app.HealthCheck(nitro.NamedHealth("db", fn))`).
- Config mistakes never panic mid-chain: `app.Err()` / `Listen` return them.
- `c.ClientIP()` is proxy-aware by default (private/loopback peers trusted).

## You must set
| Item | How |
|------|-----|
| `NITRO_ENV=production` (or unset) | dev mode enables permissive CORS and verbose errors |
| Auth on every Resource | `app.SetDefaultResourceMiddleware(nitro.JWTMiddleware(...))`; never blanket `Public: true` |
| Tenant isolation | `tenant_id` column + JWT tenant claim; `X-Tenant-ID` stays off |
| CORS origins | `app.CORS(nitro.CORSConfig{AllowedOrigins: ...})` — `*` is rejected in production |
| Public proxies | `Config.TrustedProxies` only if your proxy is not on a private network |
| DB | `nitrogen install <driver>`, `MigrateUp` on boot, pool limits in the DSN |
| Cache across replicas | Redis (`SetCache`) — see [cache.md](cache.md) |
| Rate limit | `app.RateLimit(n, window)` |

## Verified vs not yet verified
- Verified in this repository's tests: HTTP core, middleware, Resource (memory repo), cache, tenant scoping, auth adapters (JWT/OIDC), CLI scaffolds, SQLite store.
- **CI runs real Postgres/MySQL/MariaDB/SQL Server** (`sql-integration` job) and a short fuzz smoke; run them green before releasing.
- Not benchmarked: `Resource` + real database under load. Measure with your schema before sizing.
- External security review: recommended before exposing publicly.

## Release
Tag every nested module `<dir>/v2.0.0` (see `RELEASING.md`, `scripts/release.sh`); without tags `go get` cannot resolve the `/v2` modules.
