# Units / Measure (`nitro.Measure`)

**Language:** [English](units.md) · [Português](../pt/guide/units.md)

Physical measures with conversion between units (m/ft/yd, kg/lb, C/F, …).

```go
m := nitro.Meters(10)
yd := nitro.MustConvert(m, nitro.Yard)
nitro.FormatMeasure(yd, "en-US") // "10.94 yd"

parsed, _ := nitro.ParseMeasure("10 yards", "en-US")
si, _ := parsed.ToSI() // meters
```

Preferred canonical storage: **SI**. Codegen: `Length nitro.Measure \`db:"length"\`` → `length_value` + `length_unit`.

Extension: `nitro.RegisterUnit(unit, dim, toSI, fromSI)`.

DTO tag: `transform:"measure=ft,en-US"`.
