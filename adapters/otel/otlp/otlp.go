// Package otlp links the official OTLP/HTTP (protobuf) exporters into the
// binary and registers them with the Nitro OpenTelemetry bootstrap:
//
//	import _ "github.com/rodrigocelsius/go-nitro/adapters/otel/v2/otlp"
//
// Export is then enabled by the standard environment variables
// (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_HEADERS, …). Apps generated
// by nitrogen import this package; without it a binary is ~17 MB smaller and
// telemetry stays off unless NITRO_OTEL_STDOUT=1.
package otlp

import (
	"context"

	nitrootel "github.com/rodrigocelsius/go-nitro/adapters/otel/v2"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func init() {
	nitrootel.RegisterOTLP(nitrootel.OTLPExporters{
		Trace: func(ctx context.Context) (sdktrace.SpanExporter, error) {
			return otlptracehttp.New(ctx)
		},
		Metric: func(ctx context.Context) (sdkmetric.Exporter, error) {
			return otlpmetrichttp.New(ctx)
		},
	})
}
