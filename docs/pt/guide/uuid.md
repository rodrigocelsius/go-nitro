# UUID (`nitro.UUID`)

**Idioma:** [Português](uuid.md) · [English](../../guide/uuid.md)

Identificadores portáteis com `Scan`/`Value`/JSON.

```go
type Product struct {
    nitro.Model // ID é nitro.UUID
    Name string `json:"name" db:"name"`
}

id := nitro.NewUUID()
s := id.String()
parsed, err := nitro.ParseUUID(s)
```

## Wire JSON

Sempre string canônica `"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"`.

## SQL por dialeto

| Dialeto | Coluna |
|---------|--------|
| Postgres | `UUID` |
| MySQL | `CHAR(36)` |
| SQLite | `TEXT` |
| SQL Server | `UNIQUEIDENTIFIER` |

`nitro.Model.ID` usa UUID v7 via AutoID (`ApplyEntityDefaults`).
