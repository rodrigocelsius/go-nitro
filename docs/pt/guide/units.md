# Units / Measure (`nitro.Measure`)

**Idioma:** [Português](units.md) · [English](../../guide/units.md)

Medidas físicas com conversão entre unidades (m/ft/yd, kg/lb, C/F, …).

```go
m := nitro.Meters(10)
yd := nitro.MustConvert(m, nitro.Yard)
nitro.FormatMeasure(yd, "en-US") // "10.94 yd"

parsed, _ := nitro.ParseMeasure("10 yards", "en-US")
si, _ := parsed.ToSI() // metros
```

Storage canônico preferido: **SI**. Codegen: `Length nitro.Measure \`db:"length"\`` → `length_value` + `length_unit`.

Extensão: `nitro.RegisterUnit(unit, dim, toSI, fromSI)`.

Tag DTO: `transform:"measure=ft,en-US"`.
