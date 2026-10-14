# Locale

**Idioma:** [Português](locale.md) · [English](../../guide/locale.md)

```go
nitro.SetLocale("pt-BR")
nitro.SetTimezone("America/Sao_Paulo")
nitro.Locale() // NITRO_LOCALE → LANG → en-US
```

Usado por `FormatDate` / `FormatMoney` / `FormatMeasure` / `Parse*` e por `pkg/convert` (`WithLocale`).

Env:

- `NITRO_LOCALE` — ex. `pt-BR`
- `NITRO_TZ` — timezone IANA
- `NITRO_DB_DRIVER` — dialeto SQL do codegen (`postgres`, `mysql`, `sqlite`, `sqlserver`)
