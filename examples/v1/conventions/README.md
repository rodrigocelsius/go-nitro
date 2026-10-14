# conventions

Zero-configuration tour of Nitro's conventions: typed handlers, chainable middleware
(`app.CORS().ETag().Compress().ResponseTime()`), `BasicAuth` / `APIKeyAuth` / `Skip`,
and `app.Resource` defaults (plural path, snake_case, auto tenant scope).

```bash
NITRO_ENV=development go run .
go test ./...
```

Reference: [`docs/guide/conventions.md`](../../../docs/guide/conventions.md).
