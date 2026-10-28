package log

import (
	"sync"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// mockLogger implements domain.Logger for tests.
type mockLogger struct {
	mu     sync.Mutex
	infos  []string
	debugs []string
	warns  []string
	errors []string
}

func newMockLogger() *mockLogger {
	return &mockLogger{}
}

func (m *mockLogger) Debug(msg string, attrs ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.debugs = append(m.debugs, msg)
}

func (m *mockLogger) Info(msg string, attrs ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infos = append(m.infos, msg)
}

func (m *mockLogger) Warn(msg string, attrs ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warns = append(m.warns, msg)
}

func (m *mockLogger) Error(msg string, attrs ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, msg)
}

func (m *mockLogger) With(attrs ...any) domain.Logger {
	return m
}

func (m *mockLogger) infoCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.infos)
}

func TestAsyncLoggerInfo(t *testing.T) {
	mock := newMockLogger()
	async := NewAsyncLogger(mock, AsyncConfig{BufferSize: 16, BatchSize: 1})

	async.Info("hello")
	async.Info("world")

	async.Flush()

	if mock.infoCount() != 2 {
		t.Fatalf("expected 2 info messages, got %d", mock.infoCount())
	}
}

func TestAsyncLoggerClose(t *testing.T) {
	mock := newMockLogger()
	async := NewAsyncLogger(mock, AsyncConfig{BufferSize: 16, BatchSize: 1})

	async.Info("msg1")
	async.Info("msg2")

	async.Close()

	if mock.infoCount() != 2 {
		t.Fatalf("expected 2 info messages after close, got %d", mock.infoCount())
	}
}

func TestAsyncLoggerDoesNotBlock(t *testing.T) {
	mock := newMockLogger()
	async := NewAsyncLogger(mock, AsyncConfig{BufferSize: 2, BatchSize: 1})

	// Send more messages than the buffer — must not block
	for i := 0; i < 100; i++ {
		async.Info("msg")
	}

	async.Close()
}

func TestAsyncLoggerBatch(t *testing.T) {
	mock := newMockLogger()
	async := NewAsyncLogger(mock, AsyncConfig{BufferSize: 64, BatchSize: 10})

	for i := 0; i < 10; i++ {
		async.Info("msg")
	}

	async.Close()

	if mock.infoCount() != 10 {
		t.Fatalf("expected 10 info messages, got %d", mock.infoCount())
	}
}

func TestAsyncLoggerPending(t *testing.T) {
	mock := newMockLogger()
	async := NewAsyncLogger(mock, AsyncConfig{BufferSize: 64, BatchSize: 1})

	async.Info("msg1")

	pending := async.Pending()
	if pending != 1 {
		t.Fatalf("expected 1 pending, got %d", pending)
	}

	async.Close()
}

func BenchmarkAsyncLogger(b *testing.B) {
	mock := newMockLogger()
	async := NewAsyncLogger(mock, AsyncConfig{BufferSize: 4096, BatchSize: 10})
	defer async.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		async.Info("benchmark message")
	}
}

func BenchmarkSyncLogger(b *testing.B) {
	mock := newMockLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.Info("benchmark message")
	}
}

// withLogger records which logger an entry was written to.
type withLogger struct {
	*mockLogger
	child *mockLogger
}

func (w *withLogger) With(attrs ...any) domain.Logger { return w.child }

func TestAsyncLoggerWithWritesToDerivedLogger(t *testing.T) {
	root := &withLogger{mockLogger: newMockLogger(), child: newMockLogger()}
	async := NewAsyncLogger(root, AsyncConfig{BufferSize: 16, BatchSize: 4})
	defer async.Close()

	async.With("k", "v").Info("child")
	async.Info("root")
	async.Flush()

	if root.child.infoCount() != 1 || root.infoCount() != 1 {
		t.Fatalf("With entries must go to the derived logger: root=%d child=%d", root.infoCount(), root.child.infoCount())
	}
}
