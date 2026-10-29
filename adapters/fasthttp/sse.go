package fasthttp

import (
	"bufio"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// FastSSE implements domain.SSE.
//
// Every event is flushed to the client as soon as it is written. A write or flush
// error means the client went away; handlers should stop when Send returns one.
type FastSSE struct {
	w      io.Writer
	flush  func() error
	closed bool
}

// Send sends an SSE event with JSON data.
func (s *FastSSE) Send(event string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.SendRaw(event, string(jsonData))
}

// SendRaw sends an SSE event with raw data.
//
// Data containing newlines is split across several `data:` lines, which the client
// joins back with "\n"; a bare newline inside one `data:` line would end the event
// early and corrupt the stream.
func (s *FastSSE) SendRaw(event, data string) error {
	if s.closed {
		return io.ErrClosedPipe
	}
	var b strings.Builder
	b.WriteString("event: ")
	b.WriteString(event)
	b.WriteByte('\n')
	for line := range strings.SplitSeq(strings.ReplaceAll(data, "\r\n", "\n"), "\n") {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	if _, err := io.WriteString(s.w, b.String()); err != nil {
		return err
	}
	return s.doFlush()
}

// Retry sets the client reconnection time in milliseconds.
func (s *FastSSE) Retry(ms int) error {
	if _, err := io.WriteString(s.w, "retry: "+strconv.Itoa(ms)+"\n\n"); err != nil {
		return err
	}
	return s.doFlush()
}

// Close closes the SSE connection.
func (s *FastSSE) Close() error {
	s.closed = true
	return nil
}

func (s *FastSSE) doFlush() error {
	if s.flush == nil {
		return nil
	}
	return s.flush()
}

// SSEHandler creates a Server-Sent Events handler.
//
// The handler runs in fasthttp's body stream writer, after the route handler has
// returned, so each event reaches the client when it is sent instead of when the
// whole response is complete. Two consequences for the handler:
//
//   - It must not touch the request *Ctx: it is recycled once the route handler
//     returns. Copy what you need (params, claims, tenant, body) before building
//     the handler, and use context.WithoutCancel(c.Context()) for a context that
//     outlives the route handler.
//   - Its error cannot change the status code, which is already sent. A failed
//     Send means the client disconnected; return.
func SSEHandler(handler domain.SSEHandler) domain.Handler {
	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx).FasthttpCtx()

		fctx.Response.Header.SetContentType("text/event-stream")
		fctx.Response.Header.Set("Cache-Control", "no-cache")
		fctx.Response.Header.Set("Connection", "keep-alive")
		fctx.Response.Header.Set("X-Accel-Buffering", "no")

		fctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			sse := &FastSSE{w: w, flush: w.Flush}
			_ = handler(sse)
			_ = w.Flush()
		})
		return nil
	}
}
