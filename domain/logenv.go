package domain

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Log conventions shared by the HTTP access log and the DB gateway, so one
// app writes one log format with zero configuration:
//
//	NITRO_LOG_FORMAT=text|json   (default text)
//	NITRO_LOG_LEVEL=debug|info|warn|error (default info)
//	NITRO_DB_SLOW_MS=200         (successful queries are DEBUG; slower ones WARN)
const (
	LogFormatText = "text"
	LogFormatJSON = "json"

	// DefaultSlowQuery is the default NITRO_DB_SLOW_MS threshold.
	DefaultSlowQuery = 200 * time.Millisecond
)

// LogFormatFromEnv returns NITRO_LOG_FORMAT: LogFormatJSON or LogFormatText.
func LogFormatFromEnv() string {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("NITRO_LOG_FORMAT")), LogFormatJSON) {
		return LogFormatJSON
	}
	return LogFormatText
}

// LogLevelFromEnv returns NITRO_LOG_LEVEL (default slog.LevelInfo).
func LogLevelFromEnv() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NITRO_LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// SlowQueryFromEnv returns NITRO_DB_SLOW_MS (default DefaultSlowQuery).
func SlowQueryFromEnv() time.Duration {
	if ms, err := strconv.Atoi(strings.TrimSpace(os.Getenv("NITRO_DB_SLOW_MS"))); err == nil && ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return DefaultSlowQuery
}

// LevelTag is the bracketed level shown in text logs: [DEBUG], [INFO], [WARN], [ERROR].
func LevelTag(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "[DEBUG]"
	case l < slog.LevelWarn:
		return "[INFO]"
	case l < slog.LevelError:
		return "[WARN]"
	default:
		return "[ERROR]"
	}
}
