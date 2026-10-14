package ports

import "github.com/rodrigocelsius/go-nitro/domain/v2"

// TelemetryProvider provides tracing and metrics.
type TelemetryProvider interface {
	Tracer() domain.Tracer
	Meter() domain.Meter
}
