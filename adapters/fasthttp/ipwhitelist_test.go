package fasthttp

import (
	"net"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// requestFromTrustedProxy builds a RequestCtx whose direct connection
// (RemoteAddr) is the given trusted-proxy IP, so IPWhitelistMiddleware
// consults X-Forwarded-For as it would behind a real reverse proxy.
func requestFromTrustedProxy(uri, proxyIP string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&fasthttp.Request{}, &net.TCPAddr{IP: net.ParseIP(proxyIP)}, nil)
	ctx.Request.SetRequestURI(uri)
	return ctx
}

// TestIPWhitelist_XFFSpoofBlocked reproduces the bypass fixed in vuln.md
// (V-06): an attacker connecting through the single trusted proxy used to
// be able to forge X-Forwarded-For with a whitelisted IP as the *first*
// entry and be treated as that IP. The middleware must instead use the
// rightmost *untrusted* entry (the one the proxy itself appended).
func TestIPWhitelist_XFFSpoofBlocked(t *testing.T) {
	app := New()
	app.Use(IPWhitelistMiddlewareWithTrustedProxies([]string{"127.0.0.1"}, "10.0.0.1"))
	app.Get("/admin", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := requestFromTrustedProxy("/admin", "127.0.0.1")
	// Attacker's real IP (198.51.100.9) connects through the trusted proxy
	// (127.0.0.1) and spoofs the whitelisted IP as the leftmost XFF entry.
	ctx.Request.Header.Set("X-Forwarded-For", "10.0.0.1, 198.51.100.9")

	app.Handler()(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("expected 403 (spoofed XFF must not bypass the whitelist), got %d", ctx.Response.StatusCode())
	}
}

// TestIPWhitelist_LegitimateProxiedClientAllowed verifies the fix does not
// break the normal case: a whitelisted client connecting through the
// trusted proxy, where the proxy appended the real client IP as the
// rightmost (and only) XFF entry.
func TestIPWhitelist_LegitimateProxiedClientAllowed(t *testing.T) {
	app := New()
	app.Use(IPWhitelistMiddlewareWithTrustedProxies([]string{"127.0.0.1"}, "10.0.0.1"))
	app.Get("/admin", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := requestFromTrustedProxy("/admin", "127.0.0.1")
	ctx.Request.Header.Set("X-Forwarded-For", "10.0.0.1")

	app.Handler()(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200 for legitimate whitelisted client, got %d", ctx.Response.StatusCode())
	}
}
