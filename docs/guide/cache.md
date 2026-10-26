# Resource cache

**Language:** [English](cache.md)

`app.Resource` caches reads by default (`DefaultCacheTTL` = 5 min). It works **without Redis**.

| Mode | When | Behaviour |
|------|------|-----------|
| L1 in-process (default) | no `Config.Cache` / `SetCache` | per Resource: max 1024 entries, TTL 5 min, singleflight on concurrent misses, whole-resource purge on every Create/Update/Delete |
| L2 shared | `app.SetCache(nitro.RedisCache(client))` | read-through: `GET /:id` and list pages are read from / stored in Redis (TTL 5 min, keys `<resource>:id:…` / `<resource>:list:…` / `<resource>:lq:…`); writes bust `CacheBustPattern` (derived from the path) so **all replicas** see them. If Redis fails, reads go to the database and Redis is skipped for 30 s (logged at most once a minute) — the cache is never a hard dependency |
| Off | `ResourceConfig{NoCache: true}` or `CacheTTL: -1` | every read hits the repository |

## Rules

- **One replica (or sticky traffic):** the default is correct.
- **Several replicas without Redis:** a write on replica A does not purge replica B, which can serve stale data for up to `CacheTTL`. Use Redis, or `NoCache` / a short `CacheTTL` on resources that must be fresh.
- **Tenants:** list keys include the tenant filter and item reads re-check the tenant, so one tenant never sees another's cached rows (covered by `pkg/cache_tenant_isolation_test.go`).
- Values returned from the cache are shared: treat entities as read-only.
- Write paths (PATCH/PUT/DELETE loads, Commands, anything inside `nitro.InTx`) always read from the database, never from a cache, and caches are busted again after `COMMIT` (no pre-transaction rows left behind).
- No Redis installed? Nothing to configure: the L1 in-process cache is the default. `nitro.MemoryCache(n)` can also be passed to `SetCache` (single replica). `Set` with `ttl <= 0` uses the 5-minute default in both memory and Redis caches.
- Explicit use: `nitro.CacheRepo(repo, ttl)` (L1), `nitro.L2CacheRepo(repo, cache, "prefix:", ttl)` (shared), `nitro.Remember(cache, key, ttl, fn)`, `nitro.Bust(cache, patterns...)`.
