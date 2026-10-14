package log

import (
	"log/slog"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// SlogLogger implements domain.Logger using log/slog.
type SlogLogger struct {
	logger *slog.Logger
}

// NewSlogLogger creates a new SlogLogger.
func NewSlogLogger(handler slog.Handler) *SlogLogger {
	if handler == nil {
		handler = slog.Default().Handler()
	}
	return &SlogLogger{
		logger: slog.New(handler),
	}
}

func (l *SlogLogger) Debug(msg string, attrs ...any) {
	l.logger.Debug(msg, attrs...)
}

func (l *SlogLogger) Info(msg string, attrs ...any) {
	l.logger.Info(msg, attrs...)
}

func (l *SlogLogger) Warn(msg string, attrs ...any) {
	l.logger.Warn(msg, attrs...)
}

func (l *SlogLogger) Error(msg string, attrs ...any) {
	l.logger.Error(msg, attrs...)
}

func (l *SlogLogger) With(attrs ...any) domain.Logger {
	return &SlogLogger{logger: l.logger.With(attrs...)}
}

// Logger returns the inner *slog.Logger.
func (l *SlogLogger) Logger() *slog.Logger {
	return l.logger
}
