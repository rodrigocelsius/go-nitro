package fasthttp

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// MiddlewareMetrics stores metrics for a middleware.
type MiddlewareMetrics struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
	Count    int64         `json:"count"`
	AvgNs    int64         `json:"avgNs"`
}

// RequestStats stores request statistics.
type RequestStats struct {
	TotalRequests  int64   `json:"totalRequests"`
	SuccessCount   int64   `json:"successCount"`
	ErrorCount     int64   `json:"errorCount"`
	RequestsPerSec float64 `json:"requestsPerSec"`
	AvgLatencyNs   int64   `json:"avgLatencyNs"`
	P50LatencyNs   int64   `json:"p50LatencyNs"`
	P95LatencyNs   int64   `json:"p95LatencyNs"`
	P99LatencyNs   int64   `json:"p99LatencyNs"`
	ErrorRate      float64 `json:"errorRate"`
	UptimeSeconds  float64 `json:"uptimeSeconds"`
}

// MetricsCollector collects middleware and request metrics.
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]*middlewareStats
	enabled bool

	// Request stats
	totalRequests atomic.Int64
	successCount  atomic.Int64
	errorCount    atomic.Int64
	totalLatency  atomic.Int64
	latencies     []int64
	latencyMu     sync.Mutex
	startTime     time.Time
}

type middlewareStats struct {
	totalDuration time.Duration
	count         int64
}

// NewMetricsCollector creates a new collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics:   make(map[string]*middlewareStats),
		latencies: make([]int64, 0, 1000),
		startTime: time.Now(),
	}
}

// Wrap returns a handler that times the wrapped handler.
// The name is used as the key in the metrics map.
func (mc *MetricsCollector) Wrap(name string, h domain.Handler) domain.Handler {
	if mc == nil {
		return h
	}
	mc.mu.RLock()
	enabled := mc.enabled
	mc.mu.RUnlock()
	if !enabled {
		return h
	}

	return func(c *domain.Ctx) error {
		start := time.Now()
		err := h(c)
		elapsed := time.Since(start)

		mc.record(name, elapsed)
		return err
	}
}

// RecordRequest records a request with its status code and latency.
func (mc *MetricsCollector) RecordRequest(statusCode int, latency time.Duration) {
	mc.totalRequests.Add(1)
	mc.totalLatency.Add(int64(latency))

	if statusCode >= 400 {
		mc.errorCount.Add(1)
	} else {
		mc.successCount.Add(1)
	}

	mc.latencyMu.Lock()
	mc.latencies = append(mc.latencies, latency.Nanoseconds())
	// Keep only the last 10000 samples
	if len(mc.latencies) > 10000 {
		mc.latencies = mc.latencies[len(mc.latencies)-10000:]
	}
	mc.latencyMu.Unlock()
}

// GetRequestStats returns request statistics.
func (mc *MetricsCollector) GetRequestStats() RequestStats {
	total := mc.totalRequests.Load()
	var avgLatency, p50, p95, p99 int64
	var errorRate float64

	mc.latencyMu.Lock()
	if len(mc.latencies) > 0 {
		// Copy and sort for percentiles
		sorted := make([]int64, len(mc.latencies))
		copy(sorted, mc.latencies)
		// Insertion sort (efficient for partially ordered data)
		for i := 1; i < len(sorted); i++ {
			key := sorted[i]
			j := i - 1
			for j >= 0 && sorted[j] > key {
				sorted[j+1] = sorted[j]
				j--
			}
			sorted[j+1] = key
		}

		totalLat := mc.totalLatency.Load()
		avgLatency = totalLat / int64(len(sorted))

		p50Idx := len(sorted) * 50 / 100
		p95Idx := len(sorted) * 95 / 100
		p99Idx := len(sorted) * 99 / 100
		if p50Idx >= len(sorted) {
			p50Idx = len(sorted) - 1
		}
		if p95Idx >= len(sorted) {
			p95Idx = len(sorted) - 1
		}
		if p99Idx >= len(sorted) {
			p99Idx = len(sorted) - 1
		}

		p50 = sorted[p50Idx]
		p95 = sorted[p95Idx]
		p99 = sorted[p99Idx]
	}
	mc.latencyMu.Unlock()

	if total > 0 {
		errorRate = float64(mc.errorCount.Load()) / float64(total) * 100
	}

	uptime := time.Since(mc.startTime).Seconds()
	var rps float64
	if uptime > 0 {
		rps = float64(total) / uptime
	}

	return RequestStats{
		TotalRequests:  total,
		SuccessCount:   mc.successCount.Load(),
		ErrorCount:     mc.errorCount.Load(),
		RequestsPerSec: rps,
		AvgLatencyNs:   avgLatency,
		P50LatencyNs:   p50,
		P95LatencyNs:   p95,
		P99LatencyNs:   p99,
		ErrorRate:      errorRate,
		UptimeSeconds:  uptime,
	}
}

// record records middleware execution time.
func (mc *MetricsCollector) record(name string, d time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	stats, ok := mc.metrics[name]
	if !ok {
		stats = &middlewareStats{}
		mc.metrics[name] = stats
	}
	stats.totalDuration += d
	stats.count++
}

// Metrics returns collected metrics.
func (mc *MetricsCollector) Metrics() []MiddlewareMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make([]MiddlewareMetrics, 0, len(mc.metrics))
	for name, stats := range mc.metrics {
		avgNs := int64(0)
		if stats.count > 0 {
			avgNs = int64(stats.totalDuration) / stats.count
		}
		result = append(result, MiddlewareMetrics{
			Name:     name,
			Duration: stats.totalDuration,
			Count:    stats.count,
			AvgNs:    avgNs,
		})
	}
	return result
}

// Reset clears all collected metrics.
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics = make(map[string]*middlewareStats)
	mc.latencies = mc.latencies[:0]
	mc.totalRequests.Store(0)
	mc.successCount.Store(0)
	mc.errorCount.Store(0)
	mc.totalLatency.Store(0)
}

// Enable turns metrics collection on.
func (mc *MetricsCollector) Enable() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.enabled = true
}

// Disable turns metrics collection off.
func (mc *MetricsCollector) Disable() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.enabled = false
}

// PrometheusHandler returns a handler that serves Prometheus text metrics.
func PrometheusHandler(mc *MetricsCollector) domain.Handler {
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		fctx := fc.FasthttpCtx()

		stats := mc.GetRequestStats()
		fctx.Response.Header.SetContentType("text/plain; version=0.0.4; charset=utf-8")

		var out strings.Builder
		out.WriteString("# HELP nitro_requests_total Total number of requests\n")
		out.WriteString("# TYPE nitro_requests_total counter\n")
		fmt.Fprintf(&out, "nitro_requests_total %d\n", stats.TotalRequests)

		out.WriteString("# HELP nitro_requests_success Successful requests\n")
		out.WriteString("# TYPE nitro_requests_success counter\n")
		fmt.Fprintf(&out, "nitro_requests_success %d\n", stats.SuccessCount)

		out.WriteString("# HELP nitro_requests_errors Error requests\n")
		out.WriteString("# TYPE nitro_requests_errors counter\n")
		fmt.Fprintf(&out, "nitro_requests_errors %d\n", stats.ErrorCount)

		out.WriteString("# HELP nitro_request_duration_seconds Request duration in seconds\n")
		out.WriteString("# TYPE nitro_request_duration_seconds summary\n")
		fmt.Fprintf(&out, "nitro_request_duration_seconds{quantile=\"0.5\"} %.6f\n", float64(stats.P50LatencyNs)/1e9)
		fmt.Fprintf(&out, "nitro_request_duration_seconds{quantile=\"0.95\"} %.6f\n", float64(stats.P95LatencyNs)/1e9)
		fmt.Fprintf(&out, "nitro_request_duration_seconds{quantile=\"0.99\"} %.6f\n", float64(stats.P99LatencyNs)/1e9)
		fmt.Fprintf(&out, "nitro_request_duration_seconds_sum %.6f\n", float64(stats.TotalRequests*stats.AvgLatencyNs)/1e9)
		fmt.Fprintf(&out, "nitro_request_duration_seconds_count %d\n", stats.TotalRequests)

		out.WriteString("# HELP nitro_requests_per_second Requests per second\n")
		out.WriteString("# TYPE nitro_requests_per_second gauge\n")
		fmt.Fprintf(&out, "nitro_requests_per_second %.2f\n", stats.RequestsPerSec)

		out.WriteString("# HELP nitro_error_rate Error rate percentage\n")
		out.WriteString("# TYPE nitro_error_rate gauge\n")
		fmt.Fprintf(&out, "nitro_error_rate %.2f\n", stats.ErrorRate)

		out.WriteString("# HELP nitro_uptime_seconds Server uptime in seconds\n")
		out.WriteString("# TYPE nitro_uptime_seconds gauge\n")
		fmt.Fprintf(&out, "nitro_uptime_seconds %.0f\n", stats.UptimeSeconds)

		fctx.SetBodyString(out.String())
		return nil
	}
}
