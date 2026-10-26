# Business commands

Business actions on a Resource instead of ad-hoc routes: `Commands` (`place`, `submit`, `approve` with role `manager` only when total > 10 000, `cancel` with a validated body), `Rules` (a cancelled order rejects every command) and a named `Query`. Each Command is one transaction (status + audit row). JWT protects every route by default (`SetDefaultResourceMiddleware`).

Guide: [`docs/guide/business-resource.md`](../../../docs/guide/business-resource.md).

```bash
cd examples/v2/business-commands
go test .
go run .   # :8080, SQLite in ./data unless DATABASE_URL is set
```
