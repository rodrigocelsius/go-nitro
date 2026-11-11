package fasthttp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// An oversized body must be rejected with 413 and never reach the handler.
func TestMaxBodySize_StopsPipeline(t *testing.T) {
	app := New()
	app.Use(MaxBodySizeMiddleware(10))
	ran := false
	app.Post("/x", func(c *domain.Ctx) error {
		ran = true
		return c.AdapterData.(*FastCtx).JSON(200, map[string]string{"ok": "1"})
	})
	resp, err := app.Test("POST", "/x", strings.NewReader(strings.Repeat("a", 100)))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 413 || ran {
		t.Fatalf("status=%d handlerRan=%v, want 413 and handler skipped", resp.StatusCode(), ran)
	}
	if !strings.Contains(string(resp.Body()), `"max_bytes":10`) {
		t.Errorf("body = %s", resp.Body())
	}
}

// The deadline context must be alive while the handler runs.
func TestTimeoutMiddleware_ContextAliveInHandler(t *testing.T) {
	app := New()
	app.Use(TimeoutMiddleware(5 * time.Second))
	var inHandler error
	var hasDeadline bool
	var captured context.Context
	app.Get("/x", func(c *domain.Ctx) error {
		captured = c.Context()
		inHandler = captured.Err()
		_, hasDeadline = captured.Deadline()
		return nil
	})
	resp, _ := app.Test("GET", "/x", nil)
	if inHandler != nil || !hasDeadline {
		t.Fatalf("in handler: err=%v hasDeadline=%v", inHandler, hasDeadline)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status = %d", resp.StatusCode())
	}
	if captured.Err() == nil {
		t.Error("deadline context must be released (cancelled) once the request finishes")
	}
}

func TestTimeoutMiddleware_Exceeded503(t *testing.T) {
	app := New()
	app.Get("/slow", func(c *domain.Ctx) error {
		<-c.Context().Done()
		return c.Context().Err()
	}, TimeoutMiddleware(20*time.Millisecond))
	resp, _ := app.Test("GET", "/slow", nil)
	if resp.StatusCode() != 503 {
		t.Fatalf("status = %d, want 503 (body %s)", resp.StatusCode(), resp.Body())
	}
}

func TestRequestTimeoutConfig_Exceeded503(t *testing.T) {
	app := New(domain.Config{RequestTimeout: 20 * time.Millisecond})
	app.Get("/slow", func(c *domain.Ctx) error {
		<-c.Context().Done()
		return nil
	})
	resp, _ := app.Test("GET", "/slow", nil)
	if resp.StatusCode() != 503 {
		t.Fatalf("status = %d, want 503", resp.StatusCode())
	}
}

// A pre-middleware that wrote its own response stops the pipeline with ErrAbort.
func TestAbort_KeepsWrittenResponse(t *testing.T) {
	app := New()
	app.Use(func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		_ = fc.JSON(451, map[string]string{"error": "blocked"})
		return fc.Abort()
	})
	ran := false
	app.Get("/x", func(c *domain.Ctx) error { ran = true; return nil })
	resp, _ := app.Test("GET", "/x", nil)
	if resp.StatusCode() != 451 || ran || !strings.Contains(string(resp.Body()), "blocked") {
		t.Fatalf("status=%d ran=%v body=%s", resp.StatusCode(), ran, resp.Body())
	}
}

// A panic still produces the access-log / tracing post-middlewares and the
// OnDone cleanups.
func TestPanic_RunsPostMiddlewaresAndCleanups(t *testing.T) {
	app := New()
	postStatus, done := 0, false
	app.Use(func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).OnDone(func() { done = true })
		return nil
	})
	app.UsePost(func(c *domain.Ctx) error {
		postStatus = c.AdapterData.(*FastCtx).FasthttpCtx().Response.StatusCode()
		return nil
	})
	app.Get("/p", func(c *domain.Ctx) error { panic("boom") })
	resp, _ := app.Test("GET", "/p", nil)
	if resp.StatusCode() != 500 || postStatus != 500 || !done {
		t.Fatalf("status=%d postStatus=%d done=%v", resp.StatusCode(), postStatus, done)
	}
	if strings.Contains(string(resp.Body()), "boom") {
		t.Errorf("panic value leaked without Config.Debug: %s", resp.Body())
	}
}

func TestRateLimit_RetryAfter(t *testing.T) {
	app := New()
	app.Get("/x", func(c *domain.Ctx) error { return nil },
		SlidingWindowRateLimitMiddleware(1, time.Minute, func(*domain.Ctx) string { return "k" }))
	app.Test("GET", "/x", nil)
	resp, _ := app.Test("GET", "/x", nil)
	if resp.StatusCode() != 429 || string(resp.Header.Peek("Retry-After")) != "60" {
		t.Fatalf("status=%d Retry-After=%q", resp.StatusCode(), resp.Header.Peek("Retry-After"))
	}
}

func TestServerConfig_Defaults(t *testing.T) {
	srv := New().serverConfig()
	if srv.ReadBufferSize != DefaultReadBufferSize || srv.ReadTimeout != DefaultReadTimeout {
		t.Fatalf("defaults: buf=%d read=%v", srv.ReadBufferSize, srv.ReadTimeout)
	}
	srv = New(domain.Config{Server: domain.ServerConfig{ReadBufferSize: 32 << 10, IdleTimeout: time.Second, Concurrency: 10}}).serverConfig()
	if srv.ReadBufferSize != 32<<10 || srv.IdleTimeout != time.Second || srv.Concurrency != 10 {
		t.Fatalf("overrides not applied: %+v", srv)
	}
}
