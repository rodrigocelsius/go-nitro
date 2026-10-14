# Convert (`github.com/rodrigocelsius/go-nitro/pkg/convert/v2`)

**Language:** [English](convert.md) · [Português](../pt/guide/convert.md)

Unified API between portable types:

```go
import "github.com/rodrigocelsius/go-nitro/pkg/convert/v2"

u, _ := convert.To[uuid.UUID]("…")
d, _ := convert.To[temporal.Date]("05/08/2026", convert.WithLocale("pt-BR"))
yd, _ := convert.To[measure.Measure]("10 m", convert.WithUnit(measure.Yard))
s, _ := convert.To[string](yd, convert.WithLocale("en-US"))
```

Errors: `ErrInvalidFormat`, `ErrIncompatibleUnit`, `ErrUnsupportedTarget`.
