package nitro

import nitroadapter "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"

// JSONCell marshals v as a CSV cell (JSON text). See fasthttp.JSONCell.
func JSONCell(v any) string {
	return nitroadapter.JSONCell(v)
}
