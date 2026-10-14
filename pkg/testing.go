package nitro

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
)

// TestClient helps write in-process HTTP tests against App.
type TestClient struct {
	app     *App
	headers map[string]string
}

// NewTestClient creates a test client for the App.
func NewTestClient(app *App) *TestClient {
	return &TestClient{app: app, headers: make(map[string]string)}
}

// WithHeader returns a shallow copy with an extra header (chainable).
func (tc *TestClient) WithHeader(key, value string) *TestClient {
	clone := *tc
	clone.headers = copyHeaders(tc.headers)
	clone.headers[key] = value
	return &clone
}

// Bearer returns a client with Authorization: Bearer <token>.
func (tc *TestClient) Bearer(token string) *TestClient {
	return tc.WithHeader("Authorization", "Bearer "+token)
}

func copyHeaders(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (tc *TestClient) do(t *testing.T, method, path string, body io.Reader) *fasthttp.Response {
	t.Helper()
	resp, err := tc.app.TestWithHeaders(method, path, body, tc.headers)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// Get performs GET and fails the test on transport error.
func (tc *TestClient) Get(t *testing.T, path string) *fasthttp.Response {
	return tc.do(t, "GET", path, nil)
}

// GetBearer performs GET with Authorization: Bearer <token>.
func (tc *TestClient) GetBearer(t *testing.T, path, token string) *fasthttp.Response {
	return tc.Bearer(token).Get(t, path)
}

// Post performs POST with a string body.
func (tc *TestClient) Post(t *testing.T, path, body string) *fasthttp.Response {
	return tc.do(t, "POST", path, strings.NewReader(body))
}

// PostJSON performs POST serializing body as JSON.
func (tc *TestClient) PostJSON(t *testing.T, path string, body any) *fasthttp.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return tc.WithHeader("Content-Type", "application/json").do(t, "POST", path, bytes.NewReader(data))
}

// Put performs PUT with a string body.
func (tc *TestClient) Put(t *testing.T, path, body string) *fasthttp.Response {
	return tc.do(t, "PUT", path, strings.NewReader(body))
}

// PutJSON performs PUT serializing body as JSON.
func (tc *TestClient) PutJSON(t *testing.T, path string, body any) *fasthttp.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return tc.WithHeader("Content-Type", "application/json").do(t, "PUT", path, bytes.NewReader(data))
}

// Patch performs PATCH with a string body.
func (tc *TestClient) Patch(t *testing.T, path, body string) *fasthttp.Response {
	return tc.do(t, "PATCH", path, strings.NewReader(body))
}

// PatchJSON performs PATCH serializing body as JSON.
func (tc *TestClient) PatchJSON(t *testing.T, path string, body any) *fasthttp.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return tc.WithHeader("Content-Type", "application/json").do(t, "PATCH", path, bytes.NewReader(data))
}

// Delete performs DELETE.
func (tc *TestClient) Delete(t *testing.T, path string) *fasthttp.Response {
	return tc.do(t, "DELETE", path, nil)
}

// AssertStatus fails the test if the status is not the expected value.
func (tc *TestClient) AssertStatus(t *testing.T, resp *fasthttp.Response, want int) {
	t.Helper()
	if resp.StatusCode() != want {
		t.Fatalf("status = %d, want %d\nbody: %s", resp.StatusCode(), want, resp.Body())
	}
}

// AssertBodyContains fails the test if the body does not contain substr.
func (tc *TestClient) AssertBodyContains(t *testing.T, resp *fasthttp.Response, substr string) {
	t.Helper()
	if !strings.Contains(string(resp.Body()), substr) {
		t.Fatalf("body = %q, want contains %q", resp.Body(), substr)
	}
}

// AssertJSON decodes the JSON body into T and fails if invalid.
func AssertJSON[T any](t *testing.T, resp *fasthttp.Response) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(resp.Body(), &v); err != nil {
		t.Fatalf("AssertJSON: %v\nbody: %s", err, resp.Body())
	}
	return v
}

// AssertField checks a top-level JSON field (equality via fmt.Sprint).
func AssertField(t *testing.T, resp *fasthttp.Response, key string, want any) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(resp.Body(), &m); err != nil {
		t.Fatalf("AssertField: %v\nbody: %s", err, resp.Body())
	}
	got, ok := m[key]
	if !ok {
		t.Fatalf("AssertField: missing key %q in %s", key, resp.Body())
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("AssertField %q = %v (%T), want %v (%T)", key, got, got, want, want)
	}
}
