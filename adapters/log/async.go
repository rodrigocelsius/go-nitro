package log

import (
	"runtime"
	"sync"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// logEntry represents a log entry for async dispatch.
type logEntry struct {
	level string
	msg   string
	attrs []any
}

// AsyncLogger is an asynchronous wrapper around domain.Logger.
// Log entries are sent via a channel and processed in batches.
type AsyncLogger struct {
	inner     domain.Logger
	ch        chan logEntry
	done      chan struct{}
	batchSize int
	wg        sync.WaitGroup
}

// AsyncConfig configures the async logger.
type AsyncConfig struct {
	// BufferSize is the channel buffer size.
	BufferSize int
	// BatchSize is how many entries to process per batch.
	BatchSize int
}

// NewAsyncLogger creates a new async logger.
func NewAsyncLogger(inner domain.Logger, cfg ...AsyncConfig) *AsyncLogger {
	config := AsyncConfig{
		BufferSize: 1024,
		BatchSize:  1,
	}
	if len(cfg) > 0 {
		config = cfg[0]
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 1024
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 1
	}

	al := &AsyncLogger{
		inner:     inner,
		ch:        make(chan logEntry, config.BufferSize),
		done:      make(chan struct{}),
		batchSize: config.BatchSize,
	}

	al.wg.Add(1)
	go al.processLoop()

	return al
}

func (al *AsyncLogger) processLoop() {
	defer al.wg.Done()

	for {
		select {
		case entry, ok := <-al.ch:
			if !ok {
				al.drain()
				return
			}
			batch := []logEntry{entry}
			// Collect up to a full batch of what is already queued; stop as
			// soon as the channel is empty (a bare break would only leave the
			// select and spin through the remaining iterations).
		collect:
			for i := 1; i < al.batchSize; i++ {
				select {
				case e, ok := <-al.ch:
					if !ok {
						al.writeBatch(batch)
						al.drain()
						return
					}
					batch = append(batch, e)
				default:
					break collect
				}
			}
			al.writeBatch(batch)
		case <-al.done:
			al.drain()
			return
		}
	}
}

// drain processes all remaining entries in the channel.
func (al *AsyncLogger) drain() {
	for {
		select {
		case entry := <-al.ch:
			al.writeEntry(entry)
		default:
			return
		}
	}
}

func (al *AsyncLogger) writeBatch(batch []logEntry) {
	for _, entry := range batch {
		al.writeEntry(entry)
	}
}

func (al *AsyncLogger) writeEntry(entry logEntry) {
	switch entry.level {
	case "debug":
		al.inner.Debug(entry.msg, entry.attrs...)
	case "info":
		al.inner.Info(entry.msg, entry.attrs...)
	case "warn":
		al.inner.Warn(entry.msg, entry.attrs...)
	case "error":
		al.inner.Error(entry.msg, entry.attrs...)
	}
}

func (al *AsyncLogger) send(level, msg string, attrs []any) {
	select {
	case al.ch <- logEntry{level: level, msg: msg, attrs: attrs}:
	default:
		// Channel full — drop entry (does not block the caller)
	}
}

func (al *AsyncLogger) Debug(msg string, attrs ...any) {
	al.send("debug", msg, attrs)
}

func (al *AsyncLogger) Info(msg string, attrs ...any) {
	al.send("info", msg, attrs)
}

func (al *AsyncLogger) Warn(msg string, attrs ...any) {
	al.send("warn", msg, attrs)
}

func (al *AsyncLogger) Error(msg string, attrs ...any) {
	al.send("error", msg, attrs)
}

func (al *AsyncLogger) With(attrs ...any) domain.Logger {
	return &AsyncLogger{
		inner:     al.inner.With(attrs...),
		ch:        al.ch,
		done:      al.done,
		batchSize: al.batchSize,
	}
}

// Flush waits until all pending entries are processed.
func (al *AsyncLogger) Flush() {
	// Wait until the channel is drained
	for len(al.ch) > 0 {
		// Yield to the batch processor
		runtime.Gosched()
	}
}

// Close closes the async logger and waits for final processing.
func (al *AsyncLogger) Close() {
	close(al.done)
	al.wg.Wait()
}

// Pending returns the number of pending entries in the buffer.
func (al *AsyncLogger) Pending() int {
	return len(al.ch)
}
