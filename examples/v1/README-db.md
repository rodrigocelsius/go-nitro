# PostgreSQL data-access examples

Same domain (`products`) and a similar HTTP contract. **Every stack goes through the Nitro Gateway** (logs ON; `NITRO_DB_QUIET=1` turns them off).

> **Default:** BLinq (`blinq-hello`, `db-blinq`, `db-blinq-handlers`). GORM / sqlx / sqlc exist only as alternatives when the requirement asks for them.

| Example | Stack | Gateway |
|---------|-------|---------|
| **CA + BLinq** | CLI layout + `Store` + `WhereIf` | [`cleanarch-blinq`](./cleanarch-blinq/) |
| **Tx (SQLite)** | `db.Tx` parent+child, no Docker | [`sql-tx`](./sql-tx/) |
| **BLinq hello** | minimal: Store + Resource | [`blinq-hello`](./blinq-hello/) |
| **BLinq** | `Store[T]` + Resource | Store → DataAPI → Gateway |
| **BLinq handlers** | custom routes (list/get/…) + DSL | [`db-blinq-handlers`](./db-blinq-handlers/) |
| **BLinq (legacy naming)** | Resource + `/products/search` (WhereIf) | same ([`db-linq`](./db-linq/)) |
| **BLinq extras** | MERGE / Mongo / ES / GQL / migrate | compile offline; migrate via pool |
| **GORM** | `pgx.OpenEnv` → `db.SQL()` → GORM | automatic Gateway |
| **sqlx** | `pgx.OpenEnv` → `db.SQL()` → sqlx | same |
| **sqlc** | `pgx.OpenEnv` → `db.DBTX()` → Queries | same |

> The old `data/runtime` package (LINQ) was **removed**. `db-linq` and `db-blinq` use BLinq.

## Prerequisite

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
# optional: NITRO_DB_QUIET=1
```

## Shared API (CRUD)

```
GET    /products?active=true&min_price=100&q=key
GET    /products/:id
POST   /products
PATCH  /products/:id
DELETE /products/:id
```

`db-blinq` / `db-linq`: Resource. `cleanarch-blinq`: CA handlers + `WhereIf`. `db-blinq-handlers`: custom routes + `WhereIf`. `db-linq` also has `/products/search`.

## Run

```bash
cd cleanarch-blinq && go run ./cmd/server
cd blinq-hello && go run .
cd db-blinq && go run .
cd db-blinq-handlers && go run .
cd db-linq && go run .
cd blinq-extras && go run .
cd db-gorm && go run .
cd db-sqlx && go run .
cd db-sqlc && go run .
```
