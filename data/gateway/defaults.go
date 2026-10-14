package gateway

import (
	"log/slog"
	"os"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// defaultLogger uses slog on stderr (query logs on by default).
func defaultLogger() domain.Logger {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	return &slogBridge{l: slog.New(h)}
}

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
