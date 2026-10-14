package fasthttp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// FastSSE implements domain.SSE.
type FastSSE struct {
	w      io.Writer
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
func (s *FastSSE) SendRaw(event, data string) error {
	if s.closed {
		return io.ErrClosedPipe
	}
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	_, err := fmt.Fprint(s.w, msg)
	return err
}

// Retry sets the client reconnection time in milliseconds.
func (s *FastSSE) Retry(ms int) error {
	_, err := fmt.Fprintf(s.w, "retry: %d\n\n", ms)
	return err
}

// Close closes the SSE connection.
func (s *FastSSE) Close() error {
	s.closed = true
	return nil
}

// SSEHandler creates a Server-Sent Events handler.
func SSEHandler(handler domain.SSEHandler) domain.Handler {
	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx).FasthttpCtx()

		fctx.Response.Header.SetContentType("text/event-stream")
		fctx.Response.Header.Set("Cache-Control", "no-cache")
		fctx.Response.Header.Set("Connection", "keep-alive")
		fctx.Response.Header.Set("X-Accel-Buffering", "no")

		sse := &FastSSE{w: fctx}
		return handler(sse)
	}
}
