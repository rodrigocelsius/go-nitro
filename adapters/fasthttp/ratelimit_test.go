package fasthttp

import (
	"net"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestSlidingWindowRateLimitMiddleware(t *testing.T) {
	app := New()
	handler := SlidingWindowRateLimitMiddleware(3, time.Second, DefaultKeyFunc)
	app.Use(handler)
	app.Get("/test", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	for i := 0; i < 3; i++ {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/test")
		app.Handler()(ctx)
		if ctx.Response.StatusCode() != 200 {
			t.Fatalf("request %d: expected 200, got %d", i, ctx.Response.StatusCode())
		}
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)
	if ctx.Response.StatusCode() != 429 {
		t.Fatalf("expected 429, got %d", ctx.Response.StatusCode())
	}
}

func TestAPIKeyKeyFunc(t *testing.T) {
	app := New()
	app.Get("/test", func(c *domain.Ctx) error {
		key := APIKeyKeyFunc()(c)
		c.AdapterData.(*FastCtx).SetBody([]byte(key))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.Set("X-API-Key", "my-secret-key")
	app.Handler()(ctx)

	if string(ctx.Response.Body()) != "my-secret-key" {
		t.Fatalf("expected my-secret-key, got %s", string(ctx.Response.Body()))
	}
}

func TestCompositeKeyFunc(t *testing.T) {
	app := New()
	app.Get("/test", func(c *domain.Ctx) error {
		key := CompositeKeyFunc()(c)
		c.AdapterData.(*FastCtx).SetBody([]byte(key))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	body := string(ctx.Response.Body())
	if body == "" {
		t.Fatal("expected non-empty composite key")
	}

	ctx2 := &fasthttp.RequestCtx{}
	ctx2.Request.SetRequestURI("/test")
	ctx2.Request.Header.Set("X-API-Key", "key123")
	app.Handler()(ctx2)

	body2 := string(ctx2.Response.Body())
	if body2 != body+":key123" {
		t.Fatalf("expected composite key with api key, got %s", body2)
	}
}

func TestRightmostUntrustedIP(t *testing.T) {
	trustedNets := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	trustedIPs := map[string]bool{"127.0.0.1": true}

	tests := []struct {
		name string
		xff  string
		want string
	}{
		// Cliente legítimo atrás de UM proxy confiável: proxy anexou o IP real.
		{"single hop", "203.0.113.7", "203.0.113.7"},
		// Atacante forjou header e passou pelo proxy: proxy anexou o IP real
		// do atacante à direita — o IP forjado NÃO pode ser usado.
		{"forged first entry", "1.2.3.4, 198.51.100.9", "198.51.100.9"},
		// Chain: cliente -> proxy1(trusted) -> proxy2(trusted) -> servidor.
		{"chained proxies", "1.2.3.4, 10.0.0.5, 10.0.0.6", "1.2.3.4"},
		{"all trusted falls back", "10.0.0.5, 127.0.0.1", ""},
		{"malformed skipped", "garbage, 198.51.100.9", "198.51.100.9"},
	}
	for _, tt := range tests {
		if got := rightmostUntrustedIP(tt.xff, trustedNets, trustedIPs); got != tt.want {
			t.Errorf("%s: rightmostUntrustedIP(%q) = %q, want %q", tt.name, tt.xff, got, tt.want)
		}
	}
}

func mustCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	return ipnet
}

// Regression: the limiter used to ignore the key (one global bucket), so a
// single abusive client could lock out everyone else.
func TestRateLimit_IsolatedPerKey(t *testing.T) {
	app := New()
	app.Use(SlidingWindowRateLimitMiddleware(2, time.Minute, APIKeyKeyFunc()))
	app.Get("/x", func(c *domain.Ctx) error { return nil })
	status := func(key string) int {
		r, _ := app.TestWithHeaders("GET", "/x", nil, map[string]string{"X-API-Key": key})
		return r.StatusCode()
	}
	for i := 0; i < 2; i++ {
		if got := status("attacker"); got != 200 {
			t.Fatalf("attacker req %d: %d", i, got)
		}
	}
	if got := status("attacker"); got != 429 {
		t.Fatalf("attacker 3rd request should be limited, got %d", got)
	}
	if got := status("victim"); got != 200 {
		t.Fatalf("victim must not be affected by attacker, got %d", got)
	}
}

func TestPerKeySlidingWindow_BoundedKeys(t *testing.T) {
	p := newPerKeySlidingWindow(1, time.Minute, 3)
	for _, k := range []string{"a", "b", "c"} {
		if !p.Allow(k) {
			t.Fatalf("first request for %s should pass", k)
		}
	}
	// Table full: new keys share the overflow window instead of growing the map.
	if !p.Allow("d") {
		t.Fatal("first overflow request should pass")
	}
	if p.Allow("e") {
		t.Fatal("overflow window is shared and fail-closed")
	}
	if len(p.windows) != 3 {
		t.Fatalf("map grew past maxKeys: %d", len(p.windows))
	}
	// Existing keys keep their own state.
	if p.Allow("a") {
		t.Fatal("a already used its budget")
	}
}

func TestPerKeySlidingWindow_SweepsExpired(t *testing.T) {
	p := newPerKeySlidingWindow(1, 20*time.Millisecond, 2)
	p.Allow("a")
	p.Allow("b")
	time.Sleep(40 * time.Millisecond)
	if !p.Allow("c") {
		t.Fatal("expired keys must be swept to make room")
	}
	if len(p.windows) > 2 {
		t.Fatalf("map size %d", len(p.windows))
	}
}
