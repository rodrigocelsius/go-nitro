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

// A handler that finished its work and wrote a response after the deadline
// keeps that response (a 503 would make the client repeat a POST that
// succeeded), and the post-middlewares see the status the client receives.
func TestTimeout_KeepsCompletedResponse(t *testing.T) {
	app := New(domain.Config{RequestTimeout: 10 * time.Millisecond})
	postStatus := 0
	app.UsePost(func(c *domain.Ctx) error {
		postStatus = c.AdapterData.(*FastCtx).FasthttpCtx().Response.StatusCode()
		return nil
	})
	app.Post("/pay", func(c *domain.Ctx) error {
		time.Sleep(30 * time.Millisecond)
		return c.AdapterData.(*FastCtx).JSON(200, map[string]string{"charged": "yes"})
	})
	resp, _ := app.Test("POST", "/pay", strings.NewReader("{}"))
	if resp.StatusCode() != 200 || postStatus != 200 || !strings.Contains(string(resp.Body()), "charged") {
		t.Fatalf("status=%d postStatus=%d body=%s", resp.StatusCode(), postStatus, resp.Body())
	}
}

// The 503 of a missed deadline is known before the post-middlewares run, so
// the access log does not record a 200/500 the client never saw.
func TestTimeout_PostMiddlewareSees503(t *testing.T) {
	for name, h := range map[string]domain.Handler{
		"context error": func(c *domain.Ctx) error { <-c.Context().Done(); return c.Context().Err() },
		"no response":   func(c *domain.Ctx) error { <-c.Context().Done(); return nil },
	} {
		app := New(domain.Config{RequestTimeout: 10 * time.Millisecond})
		postStatus := 0
		app.UsePost(func(c *domain.Ctx) error {
			postStatus = c.AdapterData.(*FastCtx).FasthttpCtx().Response.StatusCode()
			return nil
		})
		app.Get("/slow", h)
		resp, _ := app.Test("GET", "/slow", nil)
		if resp.StatusCode() != 503 || postStatus != 503 || !strings.Contains(string(resp.Body()), "request timeout") {
			t.Errorf("%s: status=%d postStatus=%d body=%s", name, resp.StatusCode(), postStatus, resp.Body())
		}
	}
}

// An HTTP error chosen by the handler is not rewritten into a 503.
func TestTimeout_KeepsHTTPError(t *testing.T) {
	app := New(domain.Config{RequestTimeout: 10 * time.Millisecond})
	app.Get("/x", func(c *domain.Ctx) error {
		time.Sleep(30 * time.Millisecond)
		return domain.ErrNotFound
	})
	resp, _ := app.Test("GET", "/x", nil)
	if resp.StatusCode() != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode())
	}
}

// A panic inside a post-middleware must not run the earlier ones again.
func TestPanic_InPostMiddlewareDoesNotRerunPrevious(t *testing.T) {
	app := New()
	first, last := 0, 0
	app.UsePost(func(c *domain.Ctx) error { first++; return nil })
	app.UsePost(func(c *domain.Ctx) error { panic("post boom") })
	app.UsePost(func(c *domain.Ctx) error { last++; return nil })
	app.Get("/x", func(c *domain.Ctx) error { return nil })
	resp, _ := app.Test("GET", "/x", nil)
	if resp.StatusCode() != 500 || first != 1 || last != 1 {
		t.Fatalf("status=%d first=%d last=%d, want 500, 1, 1", resp.StatusCode(), first, last)
	}
}

// Both routers resolve the same paths, treat the trailing slash of a
// registered route as optional and report the route template.
func TestRouters_Parity(t *testing.T) {
	cases := []struct {
		path   string
		status int
		route  string
	}{
		{"/users/1", 200, "/users/:id"},
		{"/users/1/posts", 200, "/users/:name/posts"},
		{"/users/", 404, ""},
		{"/users/1/", 404, ""},
		{"/list", 200, "/list/"},
		{"/list/", 200, "/list/"},
		{"/dyn/7", 200, "/dyn/:id/"},
		{"/dyn/7/", 200, "/dyn/:id/"},
		{"/plain/", 404, ""},
	}
	for _, linear := range []bool{false, true} {
		app := New(domain.Config{UseLinearRouter: linear})
		h := func(c *domain.Ctx) error {
			return c.AdapterData.(*FastCtx).Text(c.Route() + "|" + c.Param("id") + "|" + c.Param("name"))
		}
		app.Get("/users/:name/posts", h)
		app.Get("/users/:id", h)
		app.Get("/list/", h)
		app.Get("/dyn/:id/", h)
		app.Get("/plain", h)
		for _, tc := range cases {
			resp, _ := app.Test("GET", tc.path, nil)
			if resp.StatusCode() != tc.status {
				t.Errorf("linear=%v %s: status = %d, want %d", linear, tc.path, resp.StatusCode(), tc.status)
				continue
			}
			if tc.status == 200 && !strings.HasPrefix(string(resp.Body()), tc.route+"|") {
				t.Errorf("linear=%v %s: body = %s, want route %s", linear, tc.path, resp.Body(), tc.route)
			}
		}
		// A pattern that failed halfway must not leave its parameter behind.
		resp, _ := app.Test("GET", "/users/1", nil)
		if string(resp.Body()) != "/users/:id|1|" {
			t.Errorf("linear=%v /users/1: body = %s", linear, resp.Body())
		}
	}
}
