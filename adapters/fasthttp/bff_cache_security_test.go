package fasthttp

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func newCachedBFF(t *testing.T, upstream http.HandlerFunc) (*App, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		upstream(w, r)
	}))
	t.Cleanup(srv.Close)
	cfg := BFFConfig{
		Services:     []ServiceConfig{{Name: "svc", BaseURL: srv.URL, Timeout: 2 * time.Second}},
		CacheEnabled: true,
		CacheTTL:     time.Minute,
	}
	app := New()
	app.Get("/me", BFFProxyHandler(cfg, "svc", "/me"))
	return app, &hits
}

func get(t *testing.T, app *App, path string, h map[string]string) (int, string) {
	t.Helper()
	r, err := app.TestWithHeaders("GET", path, nil, h)
	if err != nil {
		t.Fatal(err)
	}
	return r.StatusCode(), string(r.Body())
}

// Regression: the cache key ignored the caller, so user B received user A's
// cached response for the same URL.
func TestBFFCache_NotSharedAcrossUsers(t *testing.T) {
	app, _ := newCachedBFF(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data-for:" + r.Header.Get("Authorization")))
	})
	_, a := get(t, app, "/me", map[string]string{"Authorization": "Bearer alice"})
	_, b := get(t, app, "/me", map[string]string{"Authorization": "Bearer bob"})
	if a != "data-for:Bearer alice" || b != "data-for:Bearer bob" {
		t.Fatalf("cross-user leak: alice=%q bob=%q", a, b)
	}
}

func TestBFFCache_HitsForSameUser(t *testing.T) {
	app, hits := newCachedBFF(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	h := map[string]string{"Authorization": "Bearer alice"}
	get(t, app, "/me", h)
	get(t, app, "/me", h)
	if n := atomic.LoadInt32(hits); n != 1 {
		t.Fatalf("upstream hits=%d, want 1 (second served from cache)", n)
	}
}

func TestBFFCache_SkipsPrivateAndSetCookie(t *testing.T) {
	for name, hdr := range map[string][2]string{
		"private":    {"Cache-Control", "private"},
		"no-store":   {"Cache-Control", "no-store"},
		"set-cookie": {"Set-Cookie", "sid=1"},
	} {
		t.Run(name, func(t *testing.T) {
			app, hits := newCachedBFF(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(hdr[0], hdr[1])
				w.Write([]byte("x"))
			})
			get(t, app, "/me", nil)
			get(t, app, "/me", nil)
			if n := atomic.LoadInt32(hits); n != 2 {
				t.Fatalf("hits=%d, want 2 (not cacheable)", n)
			}
		})
	}
}

func TestBFFCache_Bounded(t *testing.T) {
	c := newBFFCache(3, 10)
	for i := 0; i < 50; i++ {
		c.set("k"+strconv.Itoa(i), []byte("v"), time.Minute)
	}
	if len(c.entries) > 3 {
		t.Fatalf("cache grew to %d entries (max 3): query-string flood would exhaust memory", len(c.entries))
	}
	c.set("big", make([]byte, 11), time.Minute)
	if _, ok := c.get("big"); ok {
		t.Fatal("oversized entry must not be cached")
	}
}

func TestBFFCache_RejectsHugeUpstream(t *testing.T) {
	app, _ := newCachedBFF(t, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<20)
		for i := 0; i < 11; i++ {
			w.Write(buf)
		}
	})
	if code, _ := get(t, app, "/me", nil); code != 500 {
		t.Fatalf("status=%d, want 500 for >10MiB upstream", code)
	}
}
