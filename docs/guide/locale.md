# Locale

**Language:** [English](locale.md) · [Português](../pt/guide/locale.md)

```go
nitro.SetLocale("pt-BR")
nitro.SetTimezone("America/Sao_Paulo")
nitro.Locale() // NITRO_LOCALE → LANG → en-US
```

Used by `FormatDate` / `FormatMoney` / `FormatMeasure` / `Parse*` and by `pkg/convert` (`WithLocale`).

Env:

- `NITRO_LOCALE` — e.g. `pt-BR`
- `NITRO_TZ` — IANA timezone
- `NITRO_DB_DRIVER` — SQL dialect for codegen (`postgres`, `mysql`, `sqlite`, `sqlserver`)
