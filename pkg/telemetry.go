package nitro

import (
	"context"
	"time"

	oteladapter "github.com/rodrigocelsius/go-nitro/adapters/otel/v2"
)

// EnsureTelemetry installs OpenTelemetry (OTLP if configured; otherwise SDK without stdout)
// and registers global Tracer/Meter. Called automatically by New() unless
// DisableTelemetry / NITRO_OTEL=0. Debug console: NITRO_OTEL_STDOUT=1.
func EnsureTelemetry(serviceName string) error {
	return oteladapter.Ensure(serviceName)
}

// TelemetryDisabled reports opt-out (ForceDisable, NITRO_OTEL=0, OTEL_SDK_DISABLED, tests).
func TelemetryDisabled() bool {
	return oteladapter.Disabled()
}

// ShutdownTelemetry flushes and shuts down OTel providers (also on New's OnShutdown).
func ShutdownTelemetry(ctx context.Context) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	return oteladapter.Shutdown(ctx)
}

// FlushTelemetry sends pending spans/metrics without shutting down the SDK.
func FlushTelemetry(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return oteladapter.ForceFlush(ctx)
}
