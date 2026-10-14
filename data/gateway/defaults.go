package gateway

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// defaultLogger writes to stderr in the app-wide log convention
// (NITRO_LOG_FORMAT / NITRO_LOG_LEVEL): JSON like the access log's json
// format, or text lines shaped like the access log ("[LEVEL] - date - msg").
func defaultLogger() domain.Logger {
	return newLogger(os.Stderr, domain.LogFormatFromEnv(), domain.LogLevelFromEnv())
}

func newLogger(w io.Writer, format string, level slog.Level) domain.Logger {
	var h slog.Handler
	if format == domain.LogFormatJSON {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	} else {
		h = &lineHandler{w: w, level: level, mu: &sync.Mutex{}}
	}
	return &slogBridge{l: slog.New(h)}
}

// lineHandler renders "[INFO] - 2006/01/02 15:04:05 - msg key=value …",
// matching the text access log so HTTP and DB lines read as one stream.
type lineHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	mu    *sync.Mutex
}

func (h *lineHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *lineHandler) Handle(_ context.Context, r slog.Record) error {
	t := r.Time
	if t.IsZero() {
		t = time.Now()
	}
	buf := make([]byte, 0, 160)
	buf = append(buf, domain.LevelTag(r.Level)...)
	buf = append(buf, " - "...)
	buf = t.AppendFormat(buf, "2006/01/02 15:04:05")
	buf = append(buf, " - "...)
	buf = append(buf, r.Message...)
	appendAttr := func(a slog.Attr) bool {
		buf = fmt.Appendf(buf, " %s=%v", a.Key, a.Value.Resolve())
		return true
	}
	for _, a := range h.attrs {
		appendAttr(a)
	}
	r.Attrs(appendAttr)
	buf = append(buf, '\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return err
}

func (h *lineHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cp := *h
	cp.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &cp
}

func (h *lineHandler) WithGroup(string) slog.Handler { return h }

type slogBridge struct{ l *slog.Logger }

func (s *slogBridge) Debug(msg string, attrs ...any) { s.l.Debug(msg, attrs...) }
func (s *slogBridge) Info(msg string, attrs ...any)  { s.l.Info(msg, attrs...) }
func (s *slogBridge) Warn(msg string, attrs ...any)  { s.l.Warn(msg, attrs...) }
func (s *slogBridge) Error(msg string, attrs ...any) { s.l.Error(msg, attrs...) }
func (s *slogBridge) With(attrs ...any) domain.Logger {
	return &slogBridge{l: s.l.With(attrs...)}
}

// defaultTracer / defaultMeter usam o global do domain (OTel via otel.Ensure).
func defaultTracer() domain.Tracer { return domain.DefaultTracer() }
func defaultMeter() domain.Meter   { return domain.DefaultMeter() }
