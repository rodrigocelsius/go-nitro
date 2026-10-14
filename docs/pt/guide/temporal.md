# Temporal (`Date`, `TimeOfDay`, `DateTime`)

**Idioma:** [Português](temporal.md) · [English](../../guide/temporal.md)

Tipos de calendário/relógio sem ambiguidade de fuso no domínio.

| Tipo | JSON | Significado |
|------|------|-------------|
| `nitro.Date` | `"YYYY-MM-DD"` | dia civil |
| `nitro.TimeOfDay` | `"HH:mm:ss"` | hora do dia |
| `nitro.DateTime` | RFC3339 `Z` | instante UTC |

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

**Regra:** use `nitro.DateTime` no `Model` (não `time.Time`). Instantes internos sempre UTC; fuso só em `Format` / `CombineLocal`.

## SQL

| Tipo | PG | MySQL | SQLite | SQL Server |
|------|-----|-------|--------|------------|
| Date | `DATE` | `DATE` | `TEXT` | `DATE` |
| TimeOfDay | `TIME` | `TIME` | `TEXT` | `TIME` |
| DateTime | `TIMESTAMPTZ` | `DATETIME(6)` | `TEXT` | `DATETIMEOFFSET` |

Tags DTO: `transform:"date=pt-BR"` / `transform:"datetime=pt-BR"`.
