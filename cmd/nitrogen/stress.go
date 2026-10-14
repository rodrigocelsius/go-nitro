package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func cmdStress(args []string) {
	fs := flag.NewFlagSet("stress", flag.ExitOnError)
	url := fs.String("url", "http://localhost:3080", "Target URL")
	connections := fs.Int("connections", 100, "Concurrent connections")
	duration := fs.Duration("duration", 30*time.Second, "Test duration")
	workers := fs.Int("workers", 10, "Worker goroutines")
	method := fs.String("method", "GET", "HTTP method")
	fs.Parse(args)

	fmt.Printf("nitrogen: stress test starting...\n")
	fmt.Printf("  URL:         %s\n", *url)
	fmt.Printf("  Connections: %d\n", *connections)
	fmt.Printf("  Duration:    %s\n", *duration)
	fmt.Printf("  Workers:     %d\n", *workers)
	fmt.Printf("  Method:      %s\n", *method)
	fmt.Println()

	result := runStressTest(*url, *connections, *duration, *workers, *method)
	printStressReport(result)
}

func runStressTest(url string, connections int, duration time.Duration, workers int, method string) *stressResult {
	result := &stressResult{
		ErrorsByCode: make(map[int]int64),
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        connections,
			MaxIdleConnsPerHost: connections,
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

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()
				req, err := http.NewRequest(method, url, nil)
				if err != nil {
					failedReqs.Add(1)
					totalReqs.Add(1)
					continue
				}

				resp, err := client.Do(req)
				if err != nil {
					failedReqs.Add(1)
					totalReqs.Add(1)
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				latency := time.Since(start)
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()

				totalReqs.Add(1)
				successReqs.Add(1)

				if resp.StatusCode >= 400 {
					mu.Lock()
					result.ErrorsByCode[resp.StatusCode]++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	result.TotalRequests = totalReqs.Load()
	result.SuccessRequests = successReqs.Load()
	result.FailedRequests = failedReqs.Load()
	result.TotalDuration = duration

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		result.AvgLatency = total / time.Duration(len(latencies))
		result.P50Latency = latencies[len(latencies)*50/100]
		result.P90Latency = latencies[len(latencies)*90/100]
		result.P99Latency = latencies[len(latencies)*99/100]
		result.MaxLatency = latencies[len(latencies)-1]
	}

	if duration.Seconds() > 0 {
		result.RequestsPerSec = float64(result.TotalRequests) / duration.Seconds()
	}

	return result
}

func printStressReport(r *stressResult) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║       NITRO STRESS TEST REPORT       ║")
	fmt.Println("╠══════════════════════════════════════╣")
	fmt.Printf("║ Total Requests:    %17d ║\n", r.TotalRequests)
	fmt.Printf("║ Success:           %17d ║\n", r.SuccessRequests)
	fmt.Printf("║ Failed:            %17d ║\n", r.FailedRequests)
	fmt.Printf("║ Duration:          %17s ║\n", r.TotalDuration.Round(time.Second))
	fmt.Printf("║ Requests/sec:      %17.1f ║\n", r.RequestsPerSec)

	fmt.Println("╠══════════════════════════════════════╣")
	fmt.Println("║           LATENCY                     ║")
	fmt.Printf("║ Average:           %17s ║\n", fmtDuration(r.AvgLatency))
	fmt.Printf("║ P50:               %17s ║\n", fmtDuration(r.P50Latency))
	fmt.Printf("║ P90:               %17s ║\n", fmtDuration(r.P90Latency))
	fmt.Printf("║ P99:               %17s ║\n", fmtDuration(r.P99Latency))
	fmt.Printf("║ Max:               %17s ║\n", fmtDuration(r.MaxLatency))

	if len(r.ErrorsByCode) > 0 {
		fmt.Println("╠══════════════════════════════════════╣")
		fmt.Println("║           ERRORS                      ║")
		for code, count := range r.ErrorsByCode {
			fmt.Printf("║ HTTP %d:           %17d ║\n", code, count)
		}
	}

	fmt.Println("╚══════════════════════════════════════╝")
}

func fmtDuration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	case d >= time.Microsecond:
		return fmt.Sprintf("%.0fµs", float64(d.Nanoseconds())/1000.0)
	default:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
}

type stressResult struct {
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
}
