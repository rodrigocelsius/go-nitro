package otel

import (
	"context"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// Without any destination the SDK must not be installed (no per-request
// spans); an OTLP endpoint without the linked exporter warns loudly.
func TestHasExporter(t *testing.T) {
	saved := otlpExporters.Load()
	t.Cleanup(func() { otlpExporters.Store(saved) })
	warned := 0
	oldWarn := warnMissingOTLP
	warnMissingOTLP = func() { warned++ }
	t.Cleanup(func() { warnMissingOTLP = oldWarn })

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("NITRO_OTEL_STDOUT", "")
	otlpExporters.Store(nil)
	if hasExporter() {
		t.Fatal("no endpoint, no stdout: nothing to export")
	}

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	if hasExporter() || warned != 1 {
		t.Fatalf("endpoint without linked exporter: hasExporter must be false and warn (warned=%d)", warned)
	}

	RegisterOTLP(OTLPExporters{
		Trace:  func(context.Context) (sdktrace.SpanExporter, error) { return tracetest.NewInMemoryExporter(), nil },
		Metric: func(context.Context) (sdkmetric.Exporter, error) { return nil, nil },
	})
	if !hasExporter() || !useOTLP() {
		t.Fatal("endpoint + registered exporter must export")
	}

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("NITRO_OTEL_STDOUT", "1")
	if !hasExporter() {
		t.Fatal("NITRO_OTEL_STDOUT=1 must export")
	}
}

type headerAdapter map[string]string

func (h headerAdapter) Header(key string, _ ...string) string { return h[key] }

// The HTTP span joins the caller's trace from the W3C traceparent header.
func TestTracingPairJoinsTraceparent(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	pre, post := TracingPair(&otelTracer{tracer: tp.Tracer("t")})
	c := &domain.Ctx{AdapterData: headerAdapter{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	}}
	if err := pre(c); err != nil {
		t.Fatal(err)
	}
	if err := post(c); err != nil {
		t.Fatal(err)
	}
	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("spans=%d", len(ended))
	}
	if got := ended[0].SpanContext().TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace id = %s, want the caller's", got)
	}
	if got := ended[0].Parent().SpanID().String(); got != "00f067aa0ba902b7" {
		t.Fatalf("parent span = %s", got)
	}
}

// Installing the SDK must not fail on a semconv schema mismatch between
// resource.Default() and the service-name attribute (it disabled export).
func TestInstallSDKResourceMerges(t *testing.T) {
	t.Setenv("NITRO_OTEL_STDOUT", "1")
	if err := installSDK("svc"); err != nil {
		t.Fatalf("installSDK: %v", err)
	}
	t.Cleanup(func() { _ = Shutdown(context.Background()) })
}
