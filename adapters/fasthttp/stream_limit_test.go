package fasthttp

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp/fasthttputil"
)

// repeatReader yields n bytes without holding them in memory. Wrapped in
// io.NopCloser it has no known length, so net/http sends it chunked.
type repeatReader struct{ n int }

func (r *repeatReader) Read(p []byte) (int, error) {
	if r.n <= 0 {
		return 0, io.EOF
	}
	if len(p) > r.n {
		p = p[:r.n]
	}
	for i := range p {
		p[i] = 'a'
	}
	r.n -= len(p)
	return len(p), nil
}

func streamingApp(t *testing.T, limit int64, read *int64, readErr *error) *http.Client {
	t.Helper()
	app := New(domain.Config{Server: domain.ServerConfig{StreamRequestBody: true}})
	app.Use(MaxBodySizeMiddleware(limit))
	app.Post("/up", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		n, err := io.Copy(io.Discard, fc.FasthttpCtx().RequestBodyStream())
		*read, *readErr = n, err
		if err != nil {
			return err
		}
		return fc.Text("ok")
	})
	ln := fasthttputil.NewInmemoryListener()
	srv := app.serverConfig()
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = ln.Close() })
	return &http.Client{Transport: &http.Transport{
		Dial: func(string, string) (net.Conn, error) { return ln.Dial() },
	}}
}

// With StreamRequestBody the server does not enforce MaxRequestBodySize; a
// chunked body must still stop at the limit instead of being buffered whole.
func TestMaxBodySize_StreamedChunkedBodyIsCapped(t *testing.T) {
	const limit = 1 << 20
	var read int64
	var readErr error
	cl := streamingApp(t, limit, &read, &readErr)

	req, _ := http.NewRequest("POST", "http://nitro/up", io.NopCloser(&repeatReader{n: 32 << 20}))
	// The server answers and closes the connection without reading the rest
	// of the body, so the client may see the close while it is still sending
	// instead of the 413.
	resp, err := cl.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode != 413 {
			t.Fatalf("status = %d, want 413", resp.StatusCode)
		}
	}
	var tooLarge *domain.PayloadTooLargeError
	if !errors.As(readErr, &tooLarge) || read > limit {
		t.Fatalf("handler read %d bytes (limit %d), err = %v", read, limit, readErr)
	}
}

// Streamed bodies within the limit reach the handler intact — chunked and
// with Content-Length — and the connection stays usable afterwards.
func TestMaxBodySize_StreamedBodyWithinLimit(t *testing.T) {
	const limit = 1 << 20
	var read int64
	var readErr error
	cl := streamingApp(t, limit, &read, &readErr)

	bodies := map[string]func() io.Reader{
		"chunked at the limit": func() io.Reader { return io.NopCloser(&repeatReader{n: limit}) },
		"chunked small":        func() io.Reader { return io.NopCloser(&repeatReader{n: 100}) },
		"content-length":       func() io.Reader { return strings.NewReader(strings.Repeat("a", 300_000)) },
	}
	want := map[string]int64{"chunked at the limit": limit, "chunked small": 100, "content-length": 300_000}
	for name, body := range bodies {
		for i := 0; i < 2; i++ { // twice: the second request reuses the connection
			resp, err := cl.Post("http://nitro/up", "application/octet-stream", body())
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 || read != want[name] || readErr != nil {
				t.Fatalf("%s #%d: status=%d read=%d err=%v", name, i, resp.StatusCode, read, readErr)
			}
		}
	}

	// Content-Length over the limit is refused before the handler runs. The
	// server answers without reading the body, so the client may see the
	// connection close while it is still sending instead of the 413.
	read = -1
	resp, err := cl.Post("http://nitro/up", "application/octet-stream", strings.NewReader(strings.Repeat("a", limit+1)))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode != 413 {
			t.Fatalf("status = %d, want 413", resp.StatusCode)
		}
	}
	if read != -1 {
		t.Fatalf("handler ran on an oversized body (read %d bytes)", read)
	}
}
