# BLinq extras (phases 10/11)

Shows offline compile + migrate:

| Demo | What |
|------|------|
| MSSQL MERGE | `Compile` upsert → `MERGE …` |
| Mongo | `CompileFind` + `CompileLookup` |
| Elastic | `CompileSearch` Query DSL |
| GraphQL | `CompileQuery` |
| migrate | In-memory runner (`$1`) and optional Postgres |

```bash
cd examples/v1/blinq-extras && go run .

# real Postgres:
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
go run . -migrate
```

Mongo/Elastic drivers (submodules with deps): see `blinq/provider/mongo/driver` and `blinq/provider/elastic/client` — use `New(db)` / `New(es)` in your apps.
