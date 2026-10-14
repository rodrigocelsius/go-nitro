package stress

import (
	"fmt"
	"strings"
	"time"
)

// FormatReport returns a formatted stress-test report.
func FormatReport(r *Result) string {
	var sb strings.Builder

	sb.WriteString("\n╔══════════════════════════════════════╗\n")
	sb.WriteString("║       NITRO STRESS TEST REPORT       ║\n")
	sb.WriteString("╠══════════════════════════════════════╣\n")

	sb.WriteString(fmt.Sprintf("║ Total Requests:    %17d ║\n", r.TotalRequests))
	sb.WriteString(fmt.Sprintf("║ Success:           %17d ║\n", r.SuccessRequests))
	sb.WriteString(fmt.Sprintf("║ Failed:            %17d ║\n", r.FailedRequests))
	sb.WriteString(fmt.Sprintf("║ Duration:          %17s ║\n", r.TotalDuration.Round(1)))
	sb.WriteString(fmt.Sprintf("║ Requests/sec:      %17.1f ║\n", r.RequestsPerSec))

	sb.WriteString("╠══════════════════════════════════════╣\n")
	sb.WriteString("║           LATENCY                     ║\n")

	sb.WriteString(fmt.Sprintf("║ Average:           %17s ║\n", formatDuration(r.AvgLatency)))
	sb.WriteString(fmt.Sprintf("║ P50:               %17s ║\n", formatDuration(r.P50Latency)))
	sb.WriteString(fmt.Sprintf("║ P90:               %17s ║\n", formatDuration(r.P90Latency)))
	sb.WriteString(fmt.Sprintf("║ P99:               %17s ║\n", formatDuration(r.P99Latency)))
	sb.WriteString(fmt.Sprintf("║ Max:               %17s ║\n", formatDuration(r.MaxLatency)))

	if len(r.ErrorsByCode) > 0 {
		sb.WriteString("╠══════════════════════════════════════╣\n")
		sb.WriteString("║           ERRORS                      ║\n")
		for code, count := range r.ErrorsByCode {
			sb.WriteString(fmt.Sprintf("║ HTTP %d:           %17d ║\n", code, count))
		}
	}

	sb.WriteString("╚══════════════════════════════════════╝\n")

	return sb.String()
}

func formatDuration(d time.Duration) string {
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
