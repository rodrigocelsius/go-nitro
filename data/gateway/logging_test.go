package gateway

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Successful queries are DEBUG (hidden at the default INFO level); slow ones
// are WARN so they stay visible; text lines share the access-log layout.
func TestLogQueryLevels(t *testing.T) {
	var buf bytes.Buffer
	g := &Gateway{logger: newLogger(&buf, domain.LogFormatText, slog.LevelInfo), slow: 100 * time.Millisecond}

	g.logQuery("select", 5*time.Millisecond, 1, false, 0)
	if buf.Len() != 0 {
		t.Fatalf("fast query logged at INFO: %q", buf.String())
	}
	g.logQuery("update", 150*time.Millisecond, 1, false, 1)
	line := buf.String()
	if !strings.HasPrefix(line, "[WARN] - ") || !strings.Contains(line, " - db slow query operation=update") {
		t.Fatalf("slow query line = %q", line)
	}

	buf.Reset()
	g.logger = newLogger(&buf, domain.LogFormatText, slog.LevelDebug)
	g.logQuery("select", 5*time.Millisecond, 1, false, 0)
	if !strings.HasPrefix(buf.String(), "[DEBUG] - ") {
		t.Fatalf("NITRO_LOG_LEVEL=debug must show queries: %q", buf.String())
	}
}

func TestLogFormatJSON(t *testing.T) {
	var buf bytes.Buffer
	g := &Gateway{logger: newLogger(&buf, domain.LogFormatJSON, slog.LevelInfo), slow: time.Millisecond}
	g.logQuery("insert", 2*time.Millisecond, 1, false, 3)
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not JSON: %v %q", err, buf.String())
	}
	if rec["level"] != "WARN" || rec["msg"] != "db slow query" || rec["operation"] != "insert" {
		t.Fatalf("record = %v", rec)
	}
}

func TestSlowQueryFromEnv(t *testing.T) {
	t.Setenv("NITRO_DB_SLOW_MS", "")
	if got := New(stubExec{}).slow; got != domain.DefaultSlowQuery {
		t.Fatalf("default slow = %v", got)
	}
	t.Setenv("NITRO_DB_SLOW_MS", "50")
	if got := New(stubExec{}).slow; got != 50*time.Millisecond {
		t.Fatalf("env slow = %v", got)
	}
	if got := New(stubExec{}, WithSlowQuery(time.Second)).slow; got != time.Second {
		t.Fatalf("WithSlowQuery = %v", got)
	}
}
