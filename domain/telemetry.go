package domain

import "context"

// Span represents a tracing span.
type Span interface {
	SetAttribute(key string, value any)
	AddEvent(name string, attrs map[string]any)
	End()
}

// Tracer creates spans. The parent ctx propagates the HTTP span → DB (and other children).
type Tracer interface {
	Start(ctx context.Context, name string) (context.Context, Span)
}

// Meter records metrics.
type Meter interface {
	Counter(name string) Counter
	Histogram(name string) Histogram
	Gauge(name string) Gauge
}

// Counter is a metrics counter.
type Counter interface {
	Add(value int64, attrs ...string)
}

// Histogram is a metrics histogram.
type Histogram interface {
	Record(value float64, attrs ...string)
}

// Gauge is a metrics gauge (a value that can go up and down).
type Gauge interface {
	Set(value float64, attrs ...string)
	Add(value float64, attrs ...string)
}

// --- Global defaults (OTel registers via SetDefaultTracer/Meter) ---

type nopSpan struct{}

func (nopSpan) SetAttribute(string, any)        {}
func (nopSpan) AddEvent(string, map[string]any) {}
func (nopSpan) End()                            {}

type nopTracer struct{}

func (nopTracer) Start(ctx context.Context, _ string) (context.Context, Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	return ctx, nopSpan{}
}

type nopCounter struct{}

func (nopCounter) Add(int64, ...string) {}

type nopHistogram struct{}

func (nopHistogram) Record(float64, ...string) {}

type nopGauge struct{}

func (nopGauge) Set(float64, ...string) {}
func (nopGauge) Add(float64, ...string) {}

type nopMeter struct{}

func (nopMeter) Counter(string) Counter     { return nopCounter{} }
func (nopMeter) Histogram(string) Histogram { return nopHistogram{} }
func (nopMeter) Gauge(string) Gauge         { return nopGauge{} }

var (
	activeTracer Tracer = nopTracer{}
	activeMeter  Meter  = nopMeter{}
)

// SetDefaultTracer sets the global Tracer (Gateway / HTTP).
func SetDefaultTracer(t Tracer) {
	if t == nil {
		activeTracer = nopTracer{}
		return
	}
	activeTracer = t
}

// SetDefaultMeter sets the global Meter.
func SetDefaultMeter(m Meter) {
	if m == nil {
		activeMeter = nopMeter{}
		return
	}
	activeMeter = m
}

// DefaultTracer returns the active tracer (nop until OTel/Install).
func DefaultTracer() Tracer { return activeTracer }

// DefaultMeter returns the active meter.
func DefaultMeter() Meter { return activeMeter }

// NopTracer is an empty tracer (opt-out / tests).
func NopTracer() Tracer { return nopTracer{} }

// NopMeter is an empty meter.
func NopMeter() Meter { return nopMeter{} }
