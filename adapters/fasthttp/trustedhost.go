package fasthttp

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// TrustedHostMiddleware rejects requests whose Host header does not match
// one of allowedHosts. FastCtx.Hostname()/BaseURL() build links (password
// reset, email verification, redirects, …) straight from the incoming Host
// header with no validation — if the app is reachable via any Host value
// an attacker sends (e.g. no upstream proxy pinning the Host, or a
// misconfigured one), those links can be poisoned to point at an
// attacker-controlled domain (Host header injection / password-reset
// poisoning). Apply this early in the middleware chain, before anything
// that calls Hostname()/BaseURL().
//
// An entry may be an exact host ("app.example.com") or a wildcard
// subdomain ("*.example.com", also matching the bare "example.com" apex).
// Host is compared without port (fasthttp's Host() already excludes it
// unless the client sent a non-default port explicitly).
//
//	app.Use(nitro.TrustedHostMiddleware("app.example.com", "*.example.com"))
func TrustedHostMiddleware(allowedHosts ...string) domain.Handler {
	if len(allowedHosts) == 0 {
		return func(c *domain.Ctx) error { return nil }
	}
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return domain.ErrForbidden
		}
		host := hostWithoutPort(fc.Hostname())
		if host == "" || !hostTrusted(host, allowedHosts) {
			return domain.NewForbidden("untrusted host header")
		}
		return nil
	}
}

func hostWithoutPort(host string) string {
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		return host[:idx]
	}
	return host
}

func hostTrusted(host string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(host, a) {
			return true
		}
		if wildcardSuffix, ok := strings.CutPrefix(a, "*."); ok {
			if strings.EqualFold(host, wildcardSuffix) || strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(wildcardSuffix)) {
				return true
			}
		}
	}
	return false
}
