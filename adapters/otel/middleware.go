package otel

import (
	"context"
	"fmt"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

const localStartNS = "_otel_start_ns"

// TracingMiddleware starts a span in pre (use with TracingPost / TracingPair).
func TracingMiddleware(tracer domain.Tracer) domain.Handler {
	pre, _ := TracingPair(tracer)
	return pre
}

// TracingPost ends the request span.
func TracingPost() domain.Handler {
	return func(c *domain.Ctx) error {
		if s := c.Span(); s != nil {
			if status := c.Status(); status > 0 {
				s.SetAttribute("http.status_code", status)
				if status >= 500 {
					s.SetAttribute("error", fmt.Sprintf("HTTP %d", status))
				}
			}
			s.End()
		}
		return nil
	}
}

// serverTracer is implemented by the OTel adapter for SpanKindServer.
type serverTracer interface {
	StartServer(ctx context.Context, name string) (context.Context, domain.Span)
}

// TracingPair returns pre+post for HTTP spans (Nitro CompileHandler model).
// The span is placed on the request context for children (e.g. Gateway DB).
func TracingPair(tracer domain.Tracer) (pre, post domain.Handler) {
	if tracer == nil {
		tracer = domain.DefaultTracer()
	}
	pre = func(c *domain.Ctx) error {
		route := c.Route()
		name := c.Method() + " " + route
		var (
			ctx  context.Context
			span domain.Span
		)
		if st, ok := tracer.(serverTracer); ok {
			ctx, span = st.StartServer(c.Context(), name)
		} else {
			ctx, span = tracer.Start(c.Context(), name)
		}
		c.SetContext(ctx)
		span.SetAttribute("http.method", c.Method())
		span.SetAttribute("http.route", route)
		span.SetAttribute("url.path", c.Path())
		if rid := c.RequestID(); rid != "" {
			span.SetAttribute("request.id", rid)
		}
		c.SetSpan(span)
		return nil
	}
	post = TracingPost()
	return pre, post
}

// MetricsMiddleware is the pre of MetricsPair.
func MetricsMiddleware(meter domain.Meter) domain.Handler {
	pre, _ := MetricsPair(meter)
	return pre
}

// MetricsPair measures HTTP duration in post (after the handler).
func MetricsPair(meter domain.Meter) (pre, post domain.Handler) {
	if meter == nil {
		meter = domain.DefaultMeter()
	}
	pre = func(c *domain.Ctx) error {
		c.SetLocal(localStartNS, time.Now().UnixNano())
		return nil
	}
	post = func(c *domain.Ctx) error {
		var duration float64
		if startNs, ok := c.Local(localStartNS).(int64); ok && startNs > 0 {
			duration = float64(time.Now().UnixNano()-startNs) / 1e9
		}
		route := c.Route()
		meter.Counter("http_requests_total").Add(1, "method="+c.Method(), "path="+route)
		meter.Histogram("http_request_duration_seconds").Record(duration, "method="+c.Method(), "path="+route)
		return nil
	}
	return pre, post
}
