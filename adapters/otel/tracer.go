package otel

import (
	"context"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Provider implements domain.Tracer using OpenTelemetry.
type Provider struct {
	tracer trace.Tracer
	meter  metric.Meter
}

// NewProvider creates a new Provider.
func NewProvider(serviceName string) *Provider {
	return &Provider{
		tracer: otel.Tracer(serviceName),
		meter:  otel.Meter(serviceName),
	}
}

// Tracer returns the Tracer.
func (p *Provider) Tracer() domain.Tracer {
	return &otelTracer{tracer: p.tracer}
}

// Meter returns the Meter with OpenTelemetry metrics.
func (p *Provider) Meter() domain.Meter {
	return &otelMeter{meter: p.meter}
}

type otelTracer struct {
	tracer trace.Tracer
}

func (t *otelTracer) Start(ctx context.Context, name string) (context.Context, domain.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := t.tracer.Start(ctx, name)
	return ctx, &otelSpan{span: span}
}

// StartServer starts a span with SpanKindServer (HTTP entry).
func (t *otelTracer) StartServer(ctx context.Context, name string) (context.Context, domain.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := t.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindServer))
	return ctx, &otelSpan{span: span}
}

type otelSpan struct {
	span trace.Span
}

func (s *otelSpan) SetAttribute(key string, value any) {
	s.span.SetAttributes(attribute.String(key, toString(value)))
	if key == "db.error" || key == "error" {
		s.span.SetStatus(codes.Error, toString(value))
	}
}

func (s *otelSpan) AddEvent(name string, attrs map[string]any) {
	otelAttrs := make([]trace.EventOption, 0, len(attrs))
	for k, v := range attrs {
		otelAttrs = append(otelAttrs, trace.WithAttributes(attribute.String(k, toString(v))))
	}
	s.span.AddEvent(name, otelAttrs...)
}

func (s *otelSpan) End() {
	s.span.End()
}

func toString(v any) string {
	return fmt.Sprintf("%v", v)
}

// otelMeter implements domain.Meter using OpenTelemetry.
type otelMeter struct {
	meter metric.Meter
}

// otelCounter implements domain.Counter using OpenTelemetry.
type otelCounter struct {
	counter metric.Int64Counter
}

func (m *otelMeter) Counter(name string) domain.Counter {
	c, _ := m.meter.Int64Counter(name)
	return &otelCounter{counter: c}
}

func (c *otelCounter) Add(value int64, attrs ...string) {
	c.counter.Add(context.Background(), value, addOpts(attrs)...)
}

// otelHistogram implements domain.Histogram using OpenTelemetry.
type otelHistogram struct {
	histogram metric.Float64Histogram
}

func (m *otelMeter) Histogram(name string) domain.Histogram {
	h, _ := m.meter.Float64Histogram(name)
	return &otelHistogram{histogram: h}
}

func (h *otelHistogram) Record(value float64, attrs ...string) {
	h.histogram.Record(context.Background(), value, recordOpts(attrs)...)
}

// otelGauge implements domain.Gauge using OTel Float64UpDownCounter.
type otelGauge struct {
	counter metric.Float64UpDownCounter
}

func (m *otelMeter) Gauge(name string) domain.Gauge {
	g, _ := m.meter.Float64UpDownCounter(name)
	return &otelGauge{counter: g}
}

func (g *otelGauge) Set(value float64, attrs ...string) {
	g.counter.Add(context.Background(), value, addOptsF64(attrs)...)
}

func (g *otelGauge) Add(value float64, attrs ...string) {
	g.counter.Add(context.Background(), value, addOptsF64(attrs)...)
}

// parseKeyValues converts a slice of "key=value" into attribute.KeyValue.
func parseKeyValues(attrs []string) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	kvs := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		for i := 0; i < len(a); i++ {
			if a[i] == '=' {
				kvs = append(kvs, attribute.String(a[:i], a[i+1:]))
				break
			}
		}
	}
	return kvs
}

func addOpts(attrs []string) []metric.AddOption {
	kvs := parseKeyValues(attrs)
	if len(kvs) == 0 {
		return nil
	}
	return []metric.AddOption{metric.WithAttributes(kvs...)}
}

func addOptsF64(attrs []string) []metric.AddOption {
	return addOpts(attrs)
}

func recordOpts(attrs []string) []metric.RecordOption {
	kvs := parseKeyValues(attrs)
	if len(kvs) == 0 {
		return nil
	}
	return []metric.RecordOption{metric.WithAttributes(kvs...)}
}
