# Resource cache

**Language:** [English](cache.md)

`app.Resource` caches reads by default (`DefaultCacheTTL` = 5 min). It works **without Redis**.

| Mode | When | Behaviour |
|------|------|-----------|
| L1 in-process (default) | no `Config.Cache` / `SetCache` | per Resource: max 1024 entries, TTL 5 min, singleflight on concurrent misses, whole-resource purge on every Create/Update/Delete |
| L2 shared | `app.SetCache(nitro.RedisCache(client))` | entries live in Redis; writes bust `CacheBustPattern` (derived from the path) so **all replicas** see them |
| Off | `ResourceConfig{NoCache: true}` or `CacheTTL: -1` | every read hits the repository |

## Rules

- **One replica (or sticky traffic):** the default is correct.
- **Several replicas without Redis:** a write on replica A does not purge replica B, which can serve stale data for up to `CacheTTL`. Use Redis, or `NoCache` / a short `CacheTTL` on resources that must be fresh.
- **Tenants:** list keys include the tenant filter and item reads re-check the tenant, so one tenant never sees another's cached rows (covered by `pkg/cache_tenant_isolation_test.go`).
- Values returned from the cache are shared: treat entities as read-only.
- Explicit use: `nitro.CacheRepo(repo, ttl)`, `nitro.Remember(cache, key, ttl, fn)`, `nitro.Bust(cache, patterns...)`.
