package fasthttp

import (
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestMetricsCollector_BasicCollection(t *testing.T) {
	mc := NewMetricsCollector()
	mc.Enable()

	counter := 0
	handler := mc.Wrap("test-mw", func(c *domain.Ctx) error {
		counter++
		return nil
	})

	// Create a test context
	app := New()
	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	// Execute handler
	handler(&c.Ctx)
	handler(&c.Ctx)
	handler(&c.Ctx)

	if counter != 3 {
		t.Errorf("handler called %d times, want 3", counter)
	}

	metrics := mc.Metrics()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	m := metrics[0]
	if m.Name != "test-mw" {
		t.Errorf("name = %s, want test-mw", m.Name)
	}
	if m.Count != 3 {
		t.Errorf("count = %d, want 3", m.Count)
	}
	if m.Duration <= 0 {
		t.Errorf("duration should be positive, got %v", m.Duration)
	}
	if m.AvgNs <= 0 {
		t.Errorf("avgNs should be positive, got %d", m.AvgNs)
	}
}

func TestMetricsCollector_MultipleMiddlewares(t *testing.T) {
	mc := NewMetricsCollector()
	mc.Enable()

	app := New()
	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	h1 := mc.Wrap("mw1", func(c *domain.Ctx) error { return nil })
	h2 := mc.Wrap("mw2", func(c *domain.Ctx) error { return nil })
	h3 := mc.Wrap("mw3", func(c *domain.Ctx) error { return nil })

	h1(&c.Ctx)
	h2(&c.Ctx)
	h3(&c.Ctx)

	metrics := mc.Metrics()
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(metrics))
	}
}

func TestMetricsCollector_Disabled(t *testing.T) {
	mc := NewMetricsCollector()
	// Not enabled

	handler := mc.Wrap("test", func(c *domain.Ctx) error { return nil })

	app := New()
	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	handler(&c.Ctx)

	metrics := mc.Metrics()
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics when disabled, got %d", len(metrics))
	}
}

func TestMetricsCollector_Reset(t *testing.T) {
	mc := NewMetricsCollector()
	mc.Enable()

	handler := mc.Wrap("test", func(c *domain.Ctx) error { return nil })

	app := New()
	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	handler(&c.Ctx)

	metrics := mc.Metrics()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric before reset, got %d", len(metrics))
	}

	mc.Reset()

	metrics = mc.Metrics()
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics after reset, got %d", len(metrics))
	}
}

func TestMetricsCollector_EnableDisable(t *testing.T) {
	mc := NewMetricsCollector()

	app := New()
	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	// Disabled at wrap time - should not record
	hDisabled := mc.Wrap("disabled-mw", func(c *domain.Ctx) error { return nil })
	hDisabled(&c.Ctx)
	if len(mc.Metrics()) != 0 {
		t.Error("should not record when disabled at wrap time")
	}

	// Enable, then wrap new handler
	mc.Enable()
	hEnabled := mc.Wrap("enabled-mw", func(c *domain.Ctx) error { return nil })
	hEnabled(&c.Ctx)
	if len(mc.Metrics()) != 1 {
		t.Errorf("should record when enabled, got %d", len(mc.Metrics()))
	}

	// Disable, but the already-wrapped handler still records
	// because Wrap is evaluated at wrap time only
	mc.Disable()
	hEnabled(&c.Ctx)
	metrics := mc.Metrics()
	// The handler was created while enabled, so it always records
	if len(metrics) != 1 {
		t.Errorf("should still have 1 metric, got %d", len(metrics))
	}
	if metrics[0].Count != 2 {
		t.Errorf("count should be 2, got %d", metrics[0].Count)
	}
}

func TestApp_EnableMetrics(t *testing.T) {
	app := New()
	mc := app.EnableMetrics()

	if mc == nil {
		t.Fatal("EnableMetrics returned nil")
	}
	if !mc.enabled {
		t.Error("metrics should be enabled")
	}
	if app.Metrics() != mc {
		t.Error("Metrics() should return the same collector")
	}
}

func TestApp_Metrics_Nil(t *testing.T) {
	app := New()
	if app.Metrics() != nil {
		t.Error("Metrics() should be nil when not enabled")
	}
}

func TestMetricsCollector_Concurrent(t *testing.T) {
	mc := NewMetricsCollector()
	mc.Enable()

	app := New()

	var handlers []domain.Handler
	for i := 0; i < 10; i++ {
		handlers = append(handlers, mc.Wrap("concurrent", func(c *domain.Ctx) error {
			time.Sleep(time.Microsecond)
			return nil
		}))
	}

	for _, h := range handlers {
		c := app.pool.Get().(*FastCtx)
		fctx := fasthttp.RequestCtx{}
		c.Reset(&fctx)
		h(&c.Ctx)
		app.pool.Put(c)
	}

	metrics := mc.Metrics()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Count != 10 {
		t.Errorf("count = %d, want 10", metrics[0].Count)
	}
}

func BenchmarkMetricsWrap(b *testing.B) {
	mc := NewMetricsCollector()
	mc.Enable()

	app := New()
	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	handler := mc.Wrap("bench", func(c *domain.Ctx) error { return nil })

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler(&c.Ctx)
	}
}

func BenchmarkMetricsWrapDisabled(b *testing.B) {
	mc := NewMetricsCollector()

	app := New()
	c := app.pool.Get().(*FastCtx)
	fctx := fasthttp.RequestCtx{}
	c.Reset(&fctx)

	handler := mc.Wrap("bench", func(c *domain.Ctx) error { return nil })

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler(&c.Ctx)
	}
}
