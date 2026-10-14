package fasthttp

import (
	"net"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func ctxFromPeer(peer string, hdr map[string]string) *FastCtx {
	rc := &fasthttp.RequestCtx{}
	rc.Init(&fasthttp.Request{}, &net.TCPAddr{IP: net.ParseIP(peer)}, nil)
	for k, v := range hdr {
		rc.Request.Header.Set(k, v)
	}
	c := &FastCtx{}
	c.Reset(rc)
	return c
}

// Regression: ClientIP used the LEFTMOST X-Forwarded-For entry, which the
// client controls even behind a trusted proxy (the proxy appends on the right).
func TestClientIP_UsesRightmostUntrusted(t *testing.T) {
	if err := SetTrustedProxies("10.0.0.0/8"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ResetTrustedProxies() })

	// attacker 198.51.100.9 forges 1.2.3.4; proxy 10.0.0.5 appended the real peer.
	c := ctxFromPeer("10.0.0.7", map[string]string{"X-Forwarded-For": "1.2.3.4, 198.51.100.9, 10.0.0.5"})
	if got := c.ClientIP(); got != "198.51.100.9" {
		t.Fatalf("ClientIP=%q, want 198.51.100.9 (forged leftmost entry must be ignored)", got)
	}
	// Untrusted peer: headers ignored entirely.
	c = ctxFromPeer("203.0.113.5", map[string]string{"X-Forwarded-For": "1.2.3.4"})
	if got := c.ClientIP(); got != "203.0.113.5" {
		t.Fatalf("ClientIP=%q for untrusted peer", got)
	}
}

func TestProtocol_ForwardedProtoOnlyFromTrustedPeer(t *testing.T) {
	c := ctxFromPeer("203.0.113.5", map[string]string{"X-Forwarded-Proto": "https"})
	if got := c.Protocol(); got != "http" {
		t.Fatalf("untrusted peer must not set protocol, got %q", got)
	}
	if err := SetTrustedProxies("10.0.0.0/8"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ResetTrustedProxies() })
	c = ctxFromPeer("10.0.0.7", map[string]string{"X-Forwarded-Proto": "https"})
	if got := c.Protocol(); got != "https" {
		t.Fatalf("trusted peer: got %q", got)
	}
	c = ctxFromPeer("10.0.0.7", map[string]string{"X-Forwarded-Proto": "javascript:"})
	if got := c.Protocol(); got != "http" {
		t.Fatalf("garbage proto accepted: %q", got)
	}
}

// Convention over configuration: with no setup at all, a private/loopback
// peer (reverse proxy, Docker, k8s ingress) is trusted and a public peer is not.
func TestClientIP_ConventionDefaultTrustsPrivatePeersOnly(t *testing.T) {
	ResetTrustedProxies()
	c := ctxFromPeer("10.1.2.3", map[string]string{"X-Forwarded-For": "9.9.9.9, 203.0.113.7"})
	if got := c.ClientIP(); got != "203.0.113.7" {
		t.Fatalf("private proxy: ClientIP=%q, want 203.0.113.7", got)
	}
	c = ctxFromPeer("198.51.100.20", map[string]string{"X-Forwarded-For": "9.9.9.9"})
	if got := c.ClientIP(); got != "198.51.100.20" {
		t.Fatalf("public peer must not spoof: ClientIP=%q", got)
	}
}

func TestClientIP_PerAppTrustedProxiesOverride(t *testing.T) {
	ResetTrustedProxies()
	// Public edge (e.g. Cloudflare) + the private convention via keyword.
	app := New(domain.Config{TrustedProxies: []string{"private", "198.51.100.0/24"}})
	c := ctxFromPeer("198.51.100.20", map[string]string{"X-Forwarded-For": "203.0.113.7"})
	c.app = app
	if got := c.ClientIP(); got != "203.0.113.7" {
		t.Fatalf("configured edge: ClientIP=%q", got)
	}
	// Empty non-nil list: trust nobody, even private peers.
	none := New(domain.Config{TrustedProxies: []string{}})
	c = ctxFromPeer("10.1.2.3", map[string]string{"X-Forwarded-For": "203.0.113.7"})
	c.app = none
	if got := c.ClientIP(); got != "10.1.2.3" {
		t.Fatalf("no-trust app: ClientIP=%q", got)
	}
	bad := New(domain.Config{TrustedProxies: []string{"not-a-cidr"}})
	if bad.Err() == nil {
		t.Fatal("invalid TrustedProxies must be a config error")
	}
}

func TestRateLimitDefaultKey_PerClientBehindPrivateProxy(t *testing.T) {
	ResetTrustedProxies()
	a := ctxFromPeer("10.0.0.5", map[string]string{"X-Forwarded-For": "203.0.113.1"})
	b := ctxFromPeer("10.0.0.5", map[string]string{"X-Forwarded-For": "203.0.113.2"})
	if DefaultKeyFunc(&a.Ctx) == DefaultKeyFunc(&b.Ctx) {
		t.Fatal("clients behind the same proxy must get separate rate-limit buckets")
	}
}

func TestBindQuery_ConventionDefaultsToSnakeCase(t *testing.T) {
	type q struct {
		Shout     bool
		UnitPrice float64
		Page      int    `query:"p"`
		Skip      string `query:"-"`
	}
	app := New(domain.Config{})
	app.Get("/q", func(c *domain.Ctx) error {
		var in q
		if err := c.AdapterData.(*FastCtx).BindQuery(&in); err != nil {
			return err
		}
		return c.AdapterData.(*FastCtx).JSON(200, map[string]any{"s": in.Shout, "p": in.UnitPrice, "pg": in.Page, "sk": in.Skip})
	})
	r, _ := app.Test("GET", "/q?shout=true&unit_price=2.5&p=3&skip=x", nil)
	if b := string(r.Body()); !strings.Contains(b, `"s":true`) || !strings.Contains(b, `"p":2.5`) || !strings.Contains(b, `"pg":3`) || !strings.Contains(b, `"sk":""`) {
		t.Fatalf("convention binding: %s", b)
	}
}
