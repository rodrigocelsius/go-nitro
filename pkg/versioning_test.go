package nitro

import (
	"testing"
	"time"
)

func TestDeprecated_Headers(t *testing.T) {
	app := New(Config{DisableStartupMessage: true})
	v1 := app.Group("/v1", Deprecated(time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC), "/v2"))
	v1.Get("/ping", func(c *Ctx) error { return c.Text("pong") })
	resp, err := app.Test("GET", "/v1/ping", nil)
	if err != nil || resp.StatusCode() != 200 {
		t.Fatalf("%v %d", err, resp.StatusCode())
	}
	h := &resp.Header
	if string(h.Peek("Deprecation")) != "true" ||
		string(h.Peek("Sunset")) != "Wed, 30 Jun 2027 00:00:00 GMT" ||
		string(h.Peek("Link")) != `</v2>; rel="successor-version"` {
		t.Fatalf("headers: %s", h.String())
	}
}
