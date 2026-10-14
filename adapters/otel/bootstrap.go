package otel

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
)

var (
	installOnce sync.Once
	installErr  error
	provider    *Provider

	sdkTP *sdktrace.TracerProvider
	sdkMP *sdkmetric.MeterProvider

	forceOff atomic.Bool
)

// ForceDisable desliga telemetria de forma permanente neste processo
// (Config.DisableTelemetry). Impede Ensure e pgx.Open de reinstalar OTel.
func ForceDisable() {
	forceOff.Store(true)
	domain.SetDefaultTracer(domain.NopTracer())
	domain.SetDefaultMeter(domain.NopMeter())
}

// Disabled reports whether telemetry was turned off by env / tests / ForceDisable.
// Opt-out: NITRO_OTEL=0|false|off, OTEL_SDK_DISABLED=true, ForceDisable().
// Em `go test`, fica off salvo NITRO_OTEL_IN_TESTS=1.
func Disabled() bool {
	if forceOff.Load() {
		return true
	}
	if testing.Testing() && os.Getenv("NITRO_OTEL_IN_TESTS") != "1" {
		return true
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv("NITRO_OTEL")))
	if v == "0" || v == "false" || v == "off" || v == "no" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true")
}

// Ensure installs the OpenTelemetry SDK once and registers Tracer/Meter on domain.
// Safe to call from nitro.New and pgx.Open (idempotent).
//
// Export (never mixed with access log by default):
//   - OTEL_EXPORTER_OTLP_ENDPOINT (or *_TRACES/_METRICS) → OTLP HTTP (collector)
//   - NITRO_OTEL_STDOUT=1 → stdout (debug only; avoid in production)
//   - otherwise → SDK on with no exporter (spans/metrics do not go to the console)
//
// Opt-out: NITRO_OTEL=0, OTEL_SDK_DISABLED=true, ForceDisable / DisableTelemetry.
func Ensure(serviceName string) error {
	installOnce.Do(func() {
		if Disabled() {
			domain.SetDefaultTracer(domain.NopTracer())
			domain.SetDefaultMeter(domain.NopMeter())
			return
		}
		if serviceName == "" {
			serviceName = os.Getenv("OTEL_SERVICE_NAME")
		}
		if serviceName == "" {
			serviceName = "nitro"
		}
		installErr = installSDK(serviceName)
		if installErr != nil {
			domain.SetDefaultTracer(domain.NopTracer())
			domain.SetDefaultMeter(domain.NopMeter())
			return
		}
		provider = NewProvider(serviceName)
		domain.SetDefaultTracer(provider.Tracer())
		domain.SetDefaultMeter(provider.Meter())
	})
	return installErr
}

// DefaultProvider returns the Provider after Ensure (nil if disabled/failed).
func DefaultProvider() *Provider {
	_ = Ensure("")
	return provider
}

// ForceFlush sends pending spans/metrics (batch) to the exporter.
func ForceFlush(ctx context.Context) error {
	var first error
	if sdkTP != nil {
		if err := sdkTP.ForceFlush(ctx); err != nil && first == nil {
			first = err
		}
	}
	if sdkMP != nil {
		if err := sdkMP.ForceFlush(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Shutdown ForceFlushes then shuts down TracerProvider/MeterProvider.
// Safe to call multiple times; after Shutdown, defaults revert to nop.
func Shutdown(ctx context.Context) error {
	err := ForceFlush(ctx)
	if sdkTP != nil {
		if e := sdkTP.Shutdown(ctx); e != nil && err == nil {
			err = e
		}
		sdkTP = nil
	}
	if sdkMP != nil {
		if e := sdkMP.Shutdown(ctx); e != nil && err == nil {
			err = e
		}
		sdkMP = nil
	}
	domain.SetDefaultTracer(domain.NopTracer())
	domain.SetDefaultMeter(domain.NopMeter())
	provider = nil
	return err
}

func installSDK(serviceName string) error {
	ctx := context.Background()
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return err
	}

	tp, err := newTracerProvider(ctx, res)
	if err != nil {
		return err
	}
	sdkTP = tp
	otel.SetTracerProvider(tp)

	mp, err := newMeterProvider(ctx, res)
	if err != nil {
		return err
	}
	sdkMP = mp
	otel.SetMeterProvider(mp)
	return nil
}

func useOTLP() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") != ""
}

func useStdout() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("NITRO_OTEL_STDOUT")))
	return v == "1" || v == "true" || v == "on" || v == "yes"
}

func newTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	opts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	switch {
	case useOTLP():
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	case useStdout():
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	// No exporter: spans created in memory and dropped (do not go to the console).
	return sdktrace.NewTracerProvider(opts...), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	opts := []sdkmetric.Option{sdkmetric.WithResource(res)}
	switch {
	case useOTLP():
		exp, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
	case useStdout():
		exp, err := stdoutmetric.New()
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
	}
	return sdkmetric.NewMeterProvider(opts...), nil
}
