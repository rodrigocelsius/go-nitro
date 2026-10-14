// Package stress implements basic load testing for Nitro.
package stress

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Config configures the stress test.
type Config struct {
	URL         string
	Connections int
	Duration    time.Duration
	Workers     int
	Method      string
	Body        []byte
	Headers     map[string]string
}

// Result stores stress-test results.
type Result struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalDuration   time.Duration
	AvgLatency      time.Duration
	P50Latency      time.Duration
	P90Latency      time.Duration
	P99Latency      time.Duration
	MaxLatency      time.Duration
	RequestsPerSec  float64
	ErrorsByCode    map[int]int64
	ErrorsByType    map[string]int64
	Latencies       []time.Duration
}

// Run executes the stress test and returns the results.
func Run(cfg Config) (*Result, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	if cfg.Connections <= 0 {
		cfg.Connections = 100
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 10
	}
	if cfg.Duration <= 0 {
		cfg.Duration = 30 * time.Second
	}
	if cfg.Method == "" {
		cfg.Method = "GET"
	}

	result := &Result{
		ErrorsByCode: make(map[int]int64),
		ErrorsByType: make(map[string]int64),
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.Connections,
			MaxIdleConnsPerHost: cfg.Connections,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		latencies   []time.Duration
		totalReqs   atomic.Int64
		successReqs atomic.Int64
		failedReqs  atomic.Int64
	)

	// Start workers
	workersPerConn := cfg.Connections / cfg.Workers
	if workersPerConn < 1 {
		workersPerConn = 1
	}

	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			workerReqs := 0
			workerSuccess := 0
			workerFailed := 0

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				latency := makeRequest(client, cfg, result)

				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()

				totalReqs.Add(1)
				workerReqs++

				if latency > 0 {
					successReqs.Add(1)
					workerSuccess++
				} else {
					failedReqs.Add(1)
					workerFailed++
				}

				_ = workersPerConn
			}
		}(i)
	}

	wg.Wait()

	result.TotalRequests = totalReqs.Load()
	result.SuccessRequests = successReqs.Load()
	result.FailedRequests = failedReqs.Load()
	result.TotalDuration = cfg.Duration
	result.Latencies = latencies

	// Calculate statistics
	if len(latencies) > 0 {
		result.AvgLatency = calculateAvg(latencies)
		result.P50Latency = calculatePercentile(latencies, 50)
		result.P90Latency = calculatePercentile(latencies, 90)
		result.P99Latency = calculatePercentile(latencies, 99)
		result.MaxLatency = calculateMax(latencies)
	}

	if result.TotalDuration.Seconds() > 0 {
		result.RequestsPerSec = float64(result.TotalRequests) / result.TotalDuration.Seconds()
	}

	return result, nil
}

func makeRequest(client *http.Client, cfg Config, result *Result) time.Duration {
	start := time.Now()

	req, err := http.NewRequest(cfg.Method, cfg.URL, nil)
	if err != nil {
		return 0
	}

	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	latency := time.Since(start)

	if resp.StatusCode >= 400 {
		result.ErrorsByCode[resp.StatusCode]++
	}

	return latency
}

func calculateAvg(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	return total / time.Duration(len(latencies))
}

func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	// Simple sort for small-medium datasets
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}

	idx := int(float64(len(sorted)) * float64(percentile) / 100.0)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func calculateMax(latencies []time.Duration) time.Duration {
	var max time.Duration
	for _, l := range latencies {
		if l > max {
			max = l
		}
	}
	return max
}
