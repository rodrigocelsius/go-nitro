# Cache

Resource reads go through the App cache: with `REDIS_ADDR`, a read-through Redis shared by all replicas and busted on writes; a per-process cache is opt-in per Resource (`CacheTTL`). If Redis goes down, reads go to the database. `nitro.Remember` for any computation. Tests use miniredis (two "replicas" + Redis outage).

Guide: [`docs/guide/cache.md`](../../../docs/guide/cache.md).

```bash
cd examples/v2/cache
go test .
go run .   # :8080, SQLite in ./data unless DATABASE_URL is set
```
