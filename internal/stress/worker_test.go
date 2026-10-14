package stress

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRun_Basic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	result, err := Run(Config{
		URL:         server.URL,
		Connections: 10,
		Duration:    2 * time.Second,
		Workers:     2,
	})

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.TotalRequests == 0 {
		t.Error("TotalRequests = 0, want > 0")
	}
	if result.SuccessRequests == 0 {
		t.Error("SuccessRequests = 0, want > 0")
	}
	if result.AvgLatency <= 0 {
		t.Error("AvgLatency <= 0, want > 0")
	}
	if result.RequestsPerSec <= 0 {
		t.Error("RequestsPerSec <= 0, want > 0")
	}
}

func TestRun_HTTPErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	result, err := Run(Config{
		URL:         server.URL,
		Connections: 5,
		Duration:    1 * time.Second,
		Workers:     1,
	})

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ErrorsByCode[500] == 0 {
		t.Error("ErrorsByCode[500] = 0, want > 0")
	}
}

func TestRun_EmptyURL(t *testing.T) {
	_, err := Run(Config{URL: ""})
	if err == nil {
		t.Error("Run() with empty URL should error")
	}
}

func TestFormatReport(t *testing.T) {
	result := &Result{
		TotalRequests:   1000,
		SuccessRequests: 950,
		FailedRequests:  50,
		TotalDuration:   10 * time.Second,
		AvgLatency:      5 * time.Millisecond,
		P50Latency:      3 * time.Millisecond,
		P90Latency:      15 * time.Millisecond,
		P99Latency:      50 * time.Millisecond,
		MaxLatency:      200 * time.Millisecond,
		RequestsPerSec:  100.0,
		ErrorsByCode:    map[int]int64{500: 50},
	}

	report := FormatReport(result)

	if len(report) == 0 {
		t.Error("FormatReport() returned empty string")
	}
	// Check key sections exist
	if !contains(report, "NITRO STRESS TEST REPORT") {
		t.Error("Report missing header")
	}
	if !contains(report, "Total Requests:") {
		t.Error("Report missing Total Requests")
	}
	if !contains(report, "LATENCY") {
		t.Error("Report missing LATENCY section")
	}
}

func TestPercentile(t *testing.T) {
	latencies := make([]time.Duration, 100)
	for i := range latencies {
		latencies[i] = time.Duration(i) * time.Millisecond
	}

	p50 := calculatePercentile(latencies, 50)
	p90 := calculatePercentile(latencies, 90)
	p99 := calculatePercentile(latencies, 99)

	if p50 < 40*time.Millisecond || p50 > 60*time.Millisecond {
		t.Errorf("P50 = %v, want ~50ms", p50)
	}
	if p90 < 80*time.Millisecond || p90 > 100*time.Millisecond {
		t.Errorf("P90 = %v, want ~90ms", p90)
	}
	if p99 < 90*time.Millisecond || p99 > 100*time.Millisecond {
		t.Errorf("P99 = %v, want ~99ms", p99)
	}
}

func BenchmarkRun(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Run(Config{
			URL:         server.URL,
			Connections: 5,
			Duration:    100 * time.Millisecond,
			Workers:     1,
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func init() {
	_ = fmt.Sprintf // ensure fmt is used
}
