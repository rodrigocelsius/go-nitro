# blinq/migrate

Migrações SQL versionadas para BLinq / Nitro.

## API

- `Migration` — `Version`, `Name`, `UpSQL`, `DownSQL`
- `FSSource` — pares `NNNNNN_name.up.sql` / `.down.sql`
- `GooseFSSource` — `NNNNNN_name.sql` com `-- +goose Up` / `-- +goose Down`
- `Runner` — `Up`, `Down(steps)`, `Status`
- Tabela `blinq_schema_migrations`
- **BindPostgres (default):** metadado usa `$1`, `$2` (pgx)
- **BindQMark:** metadado usa `?`

## Postgres (Nitro)

```go
pool, _ := pgx.New(ctx, dsn)
r := &migrate.Runner{
    DB:     pgx.AdaptMigrate(pool),
    Source: migrate.FSSource{FS: migrationsFS},
    // Bind: migrate.BindPostgres, // default
}
_ = r.Up(ctx)
```

## Geração

`codegen/migration` gera SQL estilo goose (arquivo único + timestamp).

```bash
nitrogen migrate:make models.go --name create_users
# migrations/20260718120000_create_users.sql
```

Na app: `db.MigrateUp(ctx)` lê `./migrations`. Embed: `db.MigrateFS(ctx, fsys)`.

## Exemplo

[`examples/v1/blinq-extras`](../../examples/v1/blinq-extras) — `go run .` (memory) e `go run . -migrate` (Postgres).
