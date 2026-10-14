package domain

import (
	"log/slog"
	"testing"
	"time"
)

func TestLogEnvConventions(t *testing.T) {
	t.Setenv("NITRO_LOG_FORMAT", "")
	t.Setenv("NITRO_LOG_LEVEL", "")
	t.Setenv("NITRO_DB_SLOW_MS", "")
	if LogFormatFromEnv() != LogFormatText || LogLevelFromEnv() != slog.LevelInfo || SlowQueryFromEnv() != DefaultSlowQuery {
		t.Fatal("defaults must be text / info / 200ms")
	}
	t.Setenv("NITRO_LOG_FORMAT", "JSON")
	t.Setenv("NITRO_LOG_LEVEL", "warning")
	t.Setenv("NITRO_DB_SLOW_MS", "75")
	if LogFormatFromEnv() != LogFormatJSON || LogLevelFromEnv() != slog.LevelWarn || SlowQueryFromEnv() != 75*time.Millisecond {
		t.Fatal("env overrides not applied")
	}
	t.Setenv("NITRO_DB_SLOW_MS", "abc")
	if SlowQueryFromEnv() != DefaultSlowQuery {
		t.Fatal("invalid NITRO_DB_SLOW_MS must fall back to the default")
	}
	for l, tag := range map[slog.Level]string{slog.LevelDebug: "[DEBUG]", slog.LevelInfo: "[INFO]", slog.LevelWarn: "[WARN]", slog.LevelError: "[ERROR]"} {
		if LevelTag(l) != tag {
			t.Errorf("LevelTag(%v) = %q", l, LevelTag(l))
		}
	}
}
