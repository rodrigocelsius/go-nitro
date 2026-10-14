package nitro

import (
	"net/http"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestAdapt_ServesStdHandler(t *testing.T) {
	h := AdaptFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Std", r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(r.Method))
	})
	c := &Ctx{}
	rc := &fasthttp.RequestCtx{}
	rc.Request.SetRequestURI("/hello")
	rc.Request.Header.SetMethod("POST")
	c.Reset(rc)
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rc.Response.StatusCode() != http.StatusTeapot || string(rc.Response.Body()) != "POST" ||
		string(rc.Response.Header.Peek("X-Std")) != "/hello" {
		t.Fatalf("unexpected response: %d %q", rc.Response.StatusCode(), rc.Response.Body())
	}
}

func TestAdapt_NilPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	Adapt(nil)
}
