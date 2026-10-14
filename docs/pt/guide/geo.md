# Geo (`Latitude`, `Longitude`, `LatLng`)

**Idioma:** [Português](geo.md) · [English](../../guide/geo.md)

WGS84 em graus decimais. Em memória: **`float64`**. Escala canônica: **`MaxFracDigits = 9`** (~0,11 mm).

```go
lat, _ := nitro.NewLatitude(-23.5505199)
lng, _ := nitro.NewLongitude(-46.6333094)
p := nitro.MustLatLng(-23.55, -46.63)

nitro.DistanceMeters(sp, rj) // Haversine, metros
nitro.FormatLatLng(p, "pt-BR") // DMS
nitro.ParseLatLng("-23.55,-46.63")
```

## SQL por dialeto

| Dialeto | Coluna |
|---------|--------|
| Postgres | `NUMERIC(12,9)` |
| MySQL / MariaDB | `DECIMAL(12,9)` |
| SQL Server | `DECIMAL(12,9)` |
| SQLite | `TEXT` (string decimal) |

`LatLng` com `db:"location"` → `location_lat` + `location_lng`.

Nunca `REAL` / `float4`. `Value` grava string decimal; `Scan` aceita NUMERIC/`float64`/string.
