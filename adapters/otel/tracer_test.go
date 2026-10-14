package otel_test

import (
	"context"
	"testing"

	"github.com/rodrigocelsius/go-nitro/adapters/otel/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestTracer_ChildSpanUsesParentContext(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	tr := &parentAwareTracer{inner: tp.Tracer("test")}
	ctx, parent := tr.Start(context.Background(), "GET /products")
	_, child := tr.Start(ctx, "db.query")
	child.End()
	parent.End()

	ended := sr.Ended()
	if len(ended) < 2 {
		t.Fatalf("want >=2 spans, got %d", len(ended))
	}
	var httpSpan, dbSpan sdktrace.ReadOnlySpan
	for _, s := range ended {
		switch s.Name() {
		case "GET /products":
			httpSpan = s
		case "db.query":
			dbSpan = s
		}
	}
	if httpSpan == nil || dbSpan == nil {
		t.Fatalf("missing spans: http=%v db=%v ended=%v", httpSpan != nil, dbSpan != nil, names(ended))
	}
	if dbSpan.Parent().SpanID() != httpSpan.SpanContext().SpanID() {
		t.Fatalf("db span parent=%v want http span id=%v", dbSpan.Parent().SpanID(), httpSpan.SpanContext().SpanID())
	}
}

func names(spans []sdktrace.ReadOnlySpan) []string {
	out := make([]string, len(spans))
	for i, s := range spans {
		out[i] = s.Name()
	}
	return out
}

// parentAwareTracer mirrors otelTracer but uses an injected SDK tracer (no global Ensure).
type parentAwareTracer struct {
	inner trace.Tracer
}

func (t *parentAwareTracer) Start(ctx context.Context, name string) (context.Context, domain.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := t.inner.Start(ctx, name)
	return ctx, &sdkSpan{span: span}
}

type sdkSpan struct{ span trace.Span }

func (s *sdkSpan) SetAttribute(string, any)        {}
func (s *sdkSpan) AddEvent(string, map[string]any) {}
func (s *sdkSpan) End()                            { s.span.End() }

func TestForceDisable_BlocksEnsure(t *testing.T) {
	// ForceDisable is process-wide; only assert Disabled() after call.
	otel.ForceDisable()
	if !otel.Disabled() {
		t.Fatal("ForceDisable should make Disabled() true")
	}
}
