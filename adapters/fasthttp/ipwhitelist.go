package fasthttp

import (
	"net"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// IPWhitelistMiddleware rejects requests from IPs not on the list.
// If the whitelist is empty (or has no valid entry) every request is
// REJECTED: this middleware guards sensitive endpoints (pprof, metrics,
// dashboard), and an empty list usually means a mis-read env var — failing
// open there would silently expose them. To allow everyone, do not install it.
// Supports single IPs and CIDR ranges (e.g. "192.168.1.0/24").
// trustedProxies is a list of trusted reverse-proxy IPs/CIDRs.
// When set, the middleware uses X-Forwarded-For only if the direct IP
// is in the trusted proxy list.
func IPWhitelistMiddleware(whitelist ...string) domain.Handler {
	return IPWhitelistMiddlewareWithTrustedProxies(nil, whitelist...)
}

// IPWhitelistMiddlewareWithTrustedProxies sets trusted proxies
// for correct X-Forwarded-For handling.
func IPWhitelistMiddlewareWithTrustedProxies(trustedProxies []string, whitelist ...string) domain.Handler {
	if len(whitelist) == 0 {
		return func(c *domain.Ctx) error {
			return domain.ErrForbidden
		}
	}

	nets := make([]*net.IPNet, 0, len(whitelist))
	ips := make(map[string]bool)

	for _, entry := range whitelist {
		if _, ipnet, err := net.ParseCIDR(entry); err == nil {
			nets = append(nets, ipnet)
		} else if ip := net.ParseIP(entry); ip != nil {
			ips[ip.String()] = true
		}
	}

	// Parse trusted proxy networks
	trustedNets := make([]*net.IPNet, 0, len(trustedProxies))
	trustedIPs := make(map[string]bool)
	for _, entry := range trustedProxies {
		if _, ipnet, err := net.ParseCIDR(entry); err == nil {
			trustedNets = append(trustedNets, ipnet)
		} else if ip := net.ParseIP(entry); ip != nil {
			trustedIPs[ip.String()] = true
		}
	}

	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return domain.ErrForbidden
		}
		fctx := fc.FasthttpCtx()

		// Convention: no explicit proxy list → use ClientIP(), which honors
		// forwarded headers only from trusted proxies (private networks by
		// default, Config.TrustedProxies to override).
		var clientIP net.IP
		if len(trustedNets) == 0 && len(trustedIPs) == 0 {
			clientIP = net.ParseIP(fc.ClientIP())
		} else {
			// Explicit list: legacy per-middleware behavior.
			directIP := net.ParseIP(fctx.RemoteIP().String())
			if directIP == nil {
				return domain.ErrForbidden
			}
			clientIP = directIP
			if isTrustedProxy(directIP, trustedNets, trustedIPs) {
				if xff := string(fctx.Request.Header.Peek("X-Forwarded-For")); xff != "" {
					if ip := rightmostUntrustedIP(xff, trustedNets, trustedIPs); ip != "" {
						clientIP = net.ParseIP(ip)
					}
				}
			}
		}

		if clientIP == nil {
			return domain.ErrForbidden
		}

		if ips[clientIP.String()] {
			return nil
		}

		for _, ipnet := range nets {
			if ipnet.Contains(clientIP) {
				return nil
			}
		}

		return domain.ErrForbidden
	}
}

// isTrustedProxy checks if an IP is in the trusted proxy list.
func isTrustedProxy(ip net.IP, nets []*net.IPNet, ips map[string]bool) bool {
	if ips[ip.String()] {
		return true
	}
	for _, ipnet := range nets {
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}
