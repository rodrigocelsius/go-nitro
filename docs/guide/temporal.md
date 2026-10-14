# Temporal (`Date`, `TimeOfDay`, `DateTime`)

**Language:** [English](temporal.md) · [Português](../pt/guide/temporal.md)

Calendar/clock types without timezone ambiguity in the domain.

| Type | JSON | Meaning |
|------|------|-------------|
| `nitro.Date` | `"YYYY-MM-DD"` | civil day |
| `nitro.TimeOfDay` | `"HH:mm:ss"` | time of day |
| `nitro.DateTime` | RFC3339 `Z` | UTC instant |

```go
day := nitro.MustParseDateISO("2026-08-05")
open, _ := nitro.ParseTimeOfDayISO("19:30:00")
dt := nitro.CombineLocal(day, open, nitro.Timezone())
d2, t2 := nitro.SplitLocal(dt, nitro.Timezone())

nitro.FormatDate(day, "pt-BR")       // 05/08/2026
nitro.ParseDate("05/08/2026", "pt-BR")
_ = d2
_ = t2
```

**Rule:** use `nitro.DateTime` on `Model` (not `time.Time`). Internal instants are always UTC; timezone only in `Format` / `CombineLocal`.

## SQL

| Type | PG | MySQL | SQLite | SQL Server |
|------|-----|-------|--------|------------|
| Date | `DATE` | `DATE` | `TEXT` | `DATE` |
| TimeOfDay | `TIME` | `TIME` | `TEXT` | `TIME` |
| DateTime | `TIMESTAMPTZ` | `DATETIME(6)` | `TEXT` | `DATETIMEOFFSET` |

DTO tags: `transform:"date=pt-BR"` / `transform:"datetime=pt-BR"`.
