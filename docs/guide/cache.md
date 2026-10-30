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

## Valkey

[Valkey](https://valkey.io) is the BSD-3 fork of Redis 7.2 (Redis 7.4+ is
RSAL/SSPL, Redis 8 adds AGPL). It speaks the same protocol, so every Redis
adapter takes a Valkey client unchanged:

```go
rdb, err := nitro.ValkeyClientFromEnv() // VALKEY_ADDR, else REDIS_ADDR; nil when unset
if err != nil {
    log.Fatal(err)
}
if rdb != nil {
    app.SetCache(nitro.ValkeyCache(rdb))
}
```

`nitro.ValkeyConfig` (`VALKEY_ADDR`, `VALKEY_USERNAME`, `VALKEY_PASSWORD`,
`VALKEY_DB`, `VALKEY_TLS`, `VALKEY_POOL_SIZE`, `VALKEY_DIAL_TIMEOUT`) falls back
to the `REDIS_*` names, so moving a deployment needs no env change.
`ValkeySessionStore` and `NewValkeyBreakerStore` cover sessions and named
circuit breakers. `nitrogen install valkey` adds the service to
docker-compose and wires `cmd/server`.
