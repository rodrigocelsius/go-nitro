# UUID (`nitro.UUID`)

**Language:** [English](uuid.md) · [Português](../pt/guide/uuid.md)

Portable identifiers with `Scan`/`Value`/JSON.

```go
type Product struct {
    nitro.Model // ID is nitro.UUID
    Name string `json:"name" db:"name"`
}

id := nitro.NewUUID()
s := id.String()
parsed, err := nitro.ParseUUID(s)
```

## JSON wire

Always the canonical string `"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"`.

## SQL per dialect

| Dialect | Column |
|---------|--------|
| Postgres | `UUID` |
| MySQL | `CHAR(36)` |
| SQLite | `TEXT` |
| SQL Server | `UNIQUEIDENTIFIER` |

`nitro.Model.ID` uses UUID v7 via AutoID (`ApplyEntityDefaults`).
