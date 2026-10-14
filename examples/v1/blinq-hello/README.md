# blinq-hello

BLinq hello world: **model → migrate:make → Store → Resource**.

## 1. Model

`models.go` — struct with tags (`db`, `validate`, `blinq:"index"`, …).

## 2. Generate a migration

```bash
# from the example folder (or your project)
nitrogen migrate:make models.go --name create_hellos
# → migrations/YYYYMMDDHHMMSS_create_hellos.sql (goose Up/Down)
```

## 3. Run

On startup: `db.MigrateUp(ctx)` applies `./migrations`.

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
go run .

curl -X POST http://localhost:8080/hellos -d '{"message":"world"}'
curl http://localhost:8080/hellos
```

Next: [`../db-blinq`](../db-blinq/) (filters) · [`../db-blinq-handlers`](../db-blinq-handlers/) (DSL).
