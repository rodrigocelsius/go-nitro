# Nitro examples (v1)

Index: [`../README.md`](../README.md). Postgres comparison: [`README-db.md`](README-db.md).

## Start here (v2)

| Example | What it shows |
|---------|----------------|
| [`cli-shop`](cli-shop/) | A project generated **100% by `nitrogen`** (`new` + `make:scaffold … --resource [--tenant]`), with generated tests |
| [`conventions`](conventions/) | Zero-config tour: typed handlers, `app.CORS().ETag().Compress()`, `BasicAuth`/`APIKeyAuth`/`Skip`, Resource defaults |

## Business Resource (v2)

| Example | What it shows |
|---------|----------------|
| [`order-business`](order-business/) | `Commands`, `Queries`, `Rules` on `ResourceConfig` + usecase layer |
| [`resource-aftercreate`](resource-aftercreate/) | `AfterCreate` hook — related row on CRUD create |
| [`resource-dx`](resource-dx/) | `ExtraRoutes`, `OnMutate`, filters |
| [`resource-tenant-scoped`](resource-tenant-scoped/) | `TenantMiddleware` + `TenantScope` — shared-schema IDOR isolation |
| [`sql-tx`](sql-tx/) | `db.Tx` — multi-table writes in one transaction |
| [`pos-sales`](pos-sales/) | `sqladapter.Related` — header (`Sale`) + line items (`SaleItem`), atomic checkout, add/patch item with recalculation |

Docs: [`docs/guide/business-resource.md`](../../docs/guide/business-resource.md) · [`docs/guide/resource-hooks.md`](../../docs/guide/resource-hooks.md) · [`docs/guide/resource-security.md`](../../docs/guide/resource-security.md)
