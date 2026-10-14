package gateway

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type recSpan struct {
	attrs map[string]any
	ended bool
}

func (s *recSpan) SetAttribute(k string, v any) {
	if s.attrs == nil {
		s.attrs = map[string]any{}
	}
	s.attrs[k] = v
}
func (s *recSpan) AddEvent(string, map[string]any) {}
func (s *recSpan) End()                            { s.ended = true }

type recTracer struct {
	mu     sync.Mutex
	starts []recStart
}

type recStart struct {
	ctx  context.Context
	name string
	span *recSpan
}

func (t *recTracer) Start(ctx context.Context, name string) (context.Context, domain.Span) {
	sp := &recSpan{}
	t.mu.Lock()
	t.starts = append(t.starts, recStart{ctx: ctx, name: name, span: sp})
	t.mu.Unlock()
	return ctx, sp
}

type recMeter struct {
	mu       sync.Mutex
	counters map[string][]recAdd
	histos   map[string][]recRec
}

type recAdd struct {
	n     int64
	attrs []string
}
type recRec struct {
	v     float64
	attrs []string
}

func (m *recMeter) Counter(name string) domain.Counter {
	return &recCounter{m: m, name: name}
}
func (m *recMeter) Histogram(name string) domain.Histogram {
	return &recHistogram{m: m, name: name}
}
func (m *recMeter) Gauge(string) domain.Gauge { return nopGauge{} }

type nopGauge struct{}

func (nopGauge) Set(float64, ...string) {}
func (nopGauge) Add(float64, ...string) {}

type recCounter struct {
	m    *recMeter
	name string
}

func (c *recCounter) Add(n int64, attrs ...string) {
	c.m.mu.Lock()
	defer c.m.mu.Unlock()
	if c.m.counters == nil {
		c.m.counters = map[string][]recAdd{}
	}
	c.m.counters[c.name] = append(c.m.counters[c.name], recAdd{n: n, attrs: append([]string{}, attrs...)})
}

type recHistogram struct {
	m    *recMeter
	name string
}

func (h *recHistogram) Record(v float64, attrs ...string) {
	h.m.mu.Lock()
	defer h.m.mu.Unlock()
	if h.m.histos == nil {
		h.m.histos = map[string][]recRec{}
	}
	h.m.histos[h.name] = append(h.m.histos[h.name], recRec{v: v, attrs: append([]string{}, attrs...)})
}

type errExec struct{ err error }

func (e errExec) Query(context.Context, string, []any) (Rows, error) { return nil, e.err }
func (e errExec) QueryRow(context.Context, string, []any) Row        { return errRow{err: e.err} }
func (e errExec) Exec(context.Context, string, []any) (Result, error) {
	return nil, e.err
}

type errRow struct{ err error }

func (e errRow) Scan(...any) error { return e.err }

func TestGateway_EmitsSpanAndMetricsOnExec(t *testing.T) {
	tr := &recTracer{}
	m := &recMeter{}
	gw := New(&stubExec{}, WithTracer(tr), WithMeter(m), DisableLogger())

	parent := context.WithValue(context.Background(), "http", true)
	_, err := gw.Exec(parent, "insert", "INSERT INTO t VALUES ($1)", []any{1})
	if err != nil {
		t.Fatal(err)
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.starts) != 1 || tr.starts[0].name != "db.insert" {
		t.Fatalf("starts=%+v", tr.starts)
	}
	if tr.starts[0].ctx.Value("http") != true {
		t.Fatal("span must receive parent request context")
	}
	if !tr.starts[0].span.ended {
		t.Fatal("span not ended")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	adds := m.counters["db.query.count"]
	if len(adds) != 1 || adds[0].n != 1 {
		t.Fatalf("count=%+v", adds)
	}
	if !hasAttr(adds[0].attrs, "status=ok") || !hasAttr(adds[0].attrs, "operation=insert") {
		t.Fatalf("attrs=%v", adds[0].attrs)
	}
}

func TestGateway_MetricsOnError(t *testing.T) {
	tr := &recTracer{}
	m := &recMeter{}
	gw := New(errExec{err: errors.New("boom")}, WithTracer(tr), WithMeter(m), DisableLogger())

	_, err := gw.Exec(context.Background(), "update", "UPDATE t SET x=1", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	adds := m.counters["db.query.count"]
	if len(adds) != 1 {
		t.Fatalf("want 1 metric on error, got %+v", adds)
	}
	if !hasAttr(adds[0].attrs, "status=error") {
		t.Fatalf("attrs=%v", adds[0].attrs)
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.starts[0].span.attrs["db.error"] != "boom" {
		t.Fatalf("attrs=%v", tr.starts[0].span.attrs)
	}
}

func TestGateway_QueryRowSpanOnlyOnScan(t *testing.T) {
	tr := &recTracer{}
	gw := New(&stubExec{}, WithTracer(tr), DisableMeter(), DisableLogger())

	_ = gw.QueryRow(context.Background(), "SELECT 1", nil)
	tr.mu.Lock()
	n := len(tr.starts)
	tr.mu.Unlock()
	if n != 0 {
		t.Fatalf("QueryRow must not start span before Scan, got %d", n)
	}

	row := gw.QueryRow(context.Background(), "SELECT 1", nil)
	if err := row.Scan(new(int)); err != nil {
		t.Fatal(err)
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.starts) != 1 || tr.starts[0].name != "db.query_row" {
		t.Fatalf("starts=%+v", tr.starts)
	}
	if !tr.starts[0].span.ended {
		t.Fatal("span not ended")
	}
}

func hasAttr(attrs []string, want string) bool {
	for _, a := range attrs {
		if a == want {
			return true
		}
	}
	return false
}
