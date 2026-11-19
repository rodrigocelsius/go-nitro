package nitro

import (
	"strings"
	"testing"
)

func TestHandleRegistersByMethod(t *testing.T) {
	app := New(Config{Env: EnvDevelopment, ServiceName: "handle-test"})
	ok := func(c *Ctx) error { return c.OK(map[string]string{"m": c.Method()}) }
	g := app.Group("/v1")
	for _, m := range []string{"GET", "post", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"} {
		if app.Handle(m, "/a", ok) == nil || g.Handle(m, "/b", ok) == nil {
			t.Fatalf("Handle(%s) returned nil", m)
		}
	}
	for _, path := range []string{"/a", "/v1/b"} {
		for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
			resp, err := app.Test(m, path, nil)
			if err != nil || resp.StatusCode() != 200 || !strings.Contains(string(resp.Body()), m) {
				t.Fatalf("%s %s: %v %d %s", m, path, err, resp.StatusCode(), resp.Body())
			}
		}
	}
	if app.Handle("TRACE", "/x", ok) != nil || g.Handle("BREW", "/y", ok) != nil {
		t.Fatal("unsupported method registered")
	}
	err := app.Err()
	if err == nil || !strings.Contains(err.Error(), `"TRACE"`) || !strings.Contains(err.Error(), "/v1/y") {
		t.Fatalf("config errors: %v", err)
	}
}
