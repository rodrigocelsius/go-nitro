package pgx

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

func init() {
	scan.AsFloat64Hook = func(v any) (float64, bool) {
		n, ok := v.(pgtype.Numeric)
		if !ok {
			return 0, false
		}
		f, err := n.Float64Value()
		if err != nil || !f.Valid {
			return 0, true
		}
		return f.Float64, true
	}
}

// Re-export of BLinq scan converters for a single adapter import.
var (
	AsString  = scan.AsString
	AsBool    = scan.AsBool
	AsInt64   = scan.AsInt64
	AsInt     = scan.AsInt
	AsFloat64 = scan.AsFloat64
	AsTime    = scan.AsTime
)
