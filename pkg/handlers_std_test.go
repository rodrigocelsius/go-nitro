package nitro_test

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func TestAppInfo(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Info("demo", "1.2.3")
	tc := nitro.NewTestClient(app)
	res := tc.Get(t, "/")
	tc.AssertStatus(t, res, 200)
	tc.AssertBodyContains(t, res, `"app":"demo"`)
	tc.AssertBodyContains(t, res, `"version":"1.2.3"`)
	tc.AssertBodyContains(t, res, `"status":"operational"`)
}

func TestMe_Unauthorized(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Get("/api/me", nitro.Me())
	tc := nitro.NewTestClient(app)
	res := tc.Get(t, "/api/me")
	tc.AssertStatus(t, res, 401)
}

func TestJSONCell(t *testing.T) {
	if got := nitro.JSONCell([]string{"a", "b"}); got != `["a","b"]` {
		t.Fatalf("got %q", got)
	}
	if got := nitro.JSONCell(nil); got != "[]" {
		t.Fatalf("nil=%q", got)
	}
}
