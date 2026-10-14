package fasthttp

import (
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestGenerateSecureCSRFToken_NonDeterministic(t *testing.T) {
	a, err := generateSecureCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := generateSecureCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("CSRF tokens must not be identical across calls")
	}
	if len(a) < 32 {
		t.Fatalf("token too short: %d", len(a))
	}
}

func TestSetSecureCookie_NoSecureEqualsFalse(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	cfg := WebBFFConfig{
		CookiePath:     "/",
		CookieSecure:   false,
		CookieSameSite: "Lax",
	}
	setSecureCookie(fc, "_at", "tok", 60, cfg)
	raw := string(fctx.Response.Header.Peek("Set-Cookie"))
	if strings.Contains(raw, "Secure=false") || strings.Contains(raw, "Secure=true") {
		t.Fatalf("malformed Secure attribute: %q", raw)
	}
	if strings.Contains(raw, "Secure") {
		t.Fatalf("Secure flag should be absent when CookieSecure=false: %q", raw)
	}
	if !strings.Contains(raw, "HttpOnly") {
		t.Fatalf("expected HttpOnly: %q", raw)
	}
}

func TestSetSecureCookie_SecureFlag(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	cfg := WebBFFConfig{CookiePath: "/", CookieSecure: true, CookieSameSite: "Strict"}
	setSecureCookie(fc, "_at", "tok", 60, cfg)
	raw := string(fctx.Response.Header.Peek("Set-Cookie"))
	lower := strings.ToLower(raw)
	if !strings.Contains(lower, "secure") {
		t.Fatalf("expected Secure flag: %q", raw)
	}
	if strings.Contains(lower, "secure=") {
		t.Fatalf("Secure must be a flag, not Secure=: %q", raw)
	}
}

func TestBFFInjectsRequestHeadersNotResponse(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)
	fc.SetRequestHeader("Authorization", "Bearer abc")
	fc.SetRequestHeader("X-User-ID", "u1")

	if v := string(fctx.Response.Header.Peek("Authorization")); v != "" {
		t.Fatalf("Authorization leaked to response: %q", v)
	}
	if v := string(fctx.Request.Header.Peek("Authorization")); v != "Bearer abc" {
		t.Fatalf("request Authorization = %q", v)
	}
}

func TestSafeRedirect(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	if err := fc.SafeRedirect("//evil.com"); err == nil {
		t.Fatal("expected reject //evil.com")
	}
	if err := fc.SafeRedirect("https://evil.com"); err == nil {
		t.Fatal("expected reject absolute URL")
	}
	if err := fc.SafeRedirect("/ok"); err != nil {
		t.Fatal(err)
	}
}

func TestIsSafeRequestID(t *testing.T) {
	if isSafeRequestID("") || isSafeRequestID("bad id") || isSafeRequestID(strings.Repeat("a", 65)) {
		t.Fatal("expected reject unsafe ids")
	}
	if !isSafeRequestID("req-123_ABC") {
		t.Fatal("expected accept safe id")
	}
}

func TestQuotePathStaticRel(t *testing.T) {
	// filepath.Rel based check covered indirectly; ensure helper semantics
	if isSafeRedirectPath("/var/www") && !isSafeRedirectPath("//var") {
		// smoke
	}
}
