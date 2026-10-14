package transform

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2/geo"
	"github.com/rodrigocelsius/go-nitro/domain/v2/locale"
	"github.com/rodrigocelsius/go-nitro/domain/v2/measure"
	"github.com/rodrigocelsius/go-nitro/domain/v2/money"
	"github.com/rodrigocelsius/go-nitro/domain/v2/temporal"
)

// transformDate normalizes display/ISO date strings to YYYY-MM-DD.
// arg is locale (e.g. pt-BR).
func transformDate(s, arg string) string {
	loc := locale.WithLocale(arg)
	d, err := temporal.ParseDate(s, loc)
	if err != nil {
		return s
	}
	return d.String()
}

// transformDateTime normalizes to RFC3339 UTC.
func transformDateTime(s, arg string) string {
	loc := locale.WithLocale(arg)
	dt, err := temporal.ParseDateTime(s, loc)
	if err != nil {
		return s
	}
	return dt.FormatRFC3339()
}

// transformMoney arg: "BRL" or "BRL,pt-BR" — stores major float string + currency as "amount_minor|CUR".
func transformMoney(s, arg string) string {
	cur, loc := "", locale.Locale()
	parts := strings.Split(arg, ",")
	if len(parts) >= 1 && strings.TrimSpace(parts[0]) != "" {
		cur = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		loc = locale.WithLocale(parts[1])
	}
	m, err := money.Parse(s, cur, loc)
	if err != nil {
		return s
	}
	return m.String() // "1050 BRL"
}

// transformMeasure arg: "ft" or "ft,en-US" — converts to target unit and formats "value unit".
func transformMeasure(s, arg string) string {
	unitStr, loc := "", locale.Locale()
	parts := strings.Split(arg, ",")
	if len(parts) >= 1 {
		unitStr = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		loc = locale.WithLocale(parts[1])
	}
	m, err := measure.ParseMeasure(s, loc)
	if err != nil {
		return s
	}
	if unitStr != "" {
		converted, err := measure.Convert(m, measure.Unit(unitStr))
		if err != nil {
			return s
		}
		m = converted
	}
	return measure.FormatMeasure(m, loc)
}

func transformLat(s string) string {
	la, err := geo.ParseLatitude(s)
	if err != nil {
		return s
	}
	return la.String()
}

func transformLng(s string) string {
	lo, err := geo.ParseLongitude(s)
	if err != nil {
		return s
	}
	return lo.String()
}

func transformLatLng(s string) string {
	p, err := geo.ParseLatLng(s)
	if err != nil {
		return s
	}
	return p.String()
}
