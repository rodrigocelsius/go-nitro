package fasthttp

import (
	"io"
	"strings"

	"github.com/valyala/fasthttp"
)

// Test executes an in-process HTTP request without starting a server.
// Useful for unit tests of handlers and middlewares.
//
// Usage:
//
//	app := New()
//	app.Get("/users/:id", getUser)
//	resp, err := app.Test("GET", "/users/123", nil)
func (a *App) Test(method, path string, body io.Reader) (*fasthttp.Response, error) {
	return a.TestWithHeaders(method, path, body, nil)
}

// TestWithHeaders is like Test but sets extra request headers.
func (a *App) TestWithHeaders(method, path string, body io.Reader, headers map[string]string) (*fasthttp.Response, error) {
	if err := a.Err(); err != nil {
		return nil, err
	}
	fctx := &fasthttp.RequestCtx{}

	fctx.Request.Header.SetMethod(method)
	fctx.Request.SetRequestURI(path)

	for k, v := range headers {
		fctx.Request.Header.Set(k, v)
	}

	if body != nil {
		data, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		fctx.Request.SetBody(data)
		if strings.HasPrefix(string(data), "{") {
			fctx.Request.Header.SetContentType("application/json")
		}
	}

	a.Handler()(fctx)

	resp := &fasthttp.Response{}
	fctx.Response.CopyTo(resp)
	return resp, nil
}
