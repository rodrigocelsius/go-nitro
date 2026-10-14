# Geo (`Latitude`, `Longitude`, `LatLng`)

**Language:** [English](geo.md) · [Português](../pt/guide/geo.md)

WGS84 in decimal degrees. In memory: **`float64`**. Canonical scale: **`MaxFracDigits = 9`** (~0.11 mm).

```go
lat, _ := nitro.NewLatitude(-23.5505199)
lng, _ := nitro.NewLongitude(-46.6333094)
p := nitro.MustLatLng(-23.55, -46.63)

nitro.DistanceMeters(sp, rj) // Haversine, meters
nitro.FormatLatLng(p, "pt-BR") // DMS
nitro.ParseLatLng("-23.55,-46.63")
```

## SQL per dialect

| Dialect | Column |
|---------|--------|
| Postgres | `NUMERIC(12,9)` |
| MySQL / MariaDB | `DECIMAL(12,9)` |
| SQL Server | `DECIMAL(12,9)` |
| SQLite | `TEXT` (decimal string) |

`LatLng` with `db:"location"` → `location_lat` + `location_lng`.

Never `REAL` / `float4`. `Value` writes a decimal string; `Scan` accepts NUMERIC/`float64`/string.
