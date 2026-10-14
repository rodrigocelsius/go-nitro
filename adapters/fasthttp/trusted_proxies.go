package fasthttp

import (
	"net"
	"strings"
	"sync/atomic"
)

// trustSet is an immutable set of proxy networks that may set
// X-Forwarded-For / X-Real-IP / X-Forwarded-Proto.
type trustSet struct {
	nets []*net.IPNet
}

func (t *trustSet) contains(ip net.IP) bool {
	if t == nil || ip == nil {
		return false
	}
	for _, n := range t.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// privateProxyCIDRs is the convention default: loopback, link-local and
// private (RFC 1918 / ULA) networks. A reverse proxy, load balancer, Docker
// bridge or Kubernetes ingress sits in one of these ranges, while a client on
// the public internet never does — so it cannot forge forwarded headers.
// Same default as Echo's IP extractor.
var privateProxyCIDRs = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"169.254.0.0/16", "fe80::/10", "fc00::/7",
}

// PrivateProxyKeyword expands to the private-network defaults inside
// SetTrustedProxies / Config.TrustedProxies, e.g. {"private", "203.0.113.0/24"}.
const PrivateProxyKeyword = "private"

var (
	defaultTrust = mustParseTrust(privateProxyCIDRs)
	globalTrust  atomic.Pointer[trustSet]
)

func init() { globalTrust.Store(defaultTrust) }

func mustParseTrust(cidrs []string) *trustSet {
	ts := &trustSet{}
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic(err) // static, package-internal list
		}
		ts.nets = append(ts.nets, n)
	}
	return ts
}

// parseTrust builds a trust set from IPs, CIDRs and the "private" keyword.
func parseTrust(cidrs []string) (*trustSet, error) {
	ts := &trustSet{}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		switch {
		case c == "":
			continue
		case strings.EqualFold(c, PrivateProxyKeyword):
			ts.nets = append(ts.nets, mustParseTrust(privateProxyCIDRs).nets...)
			continue
		}
		if ip := net.ParseIP(c); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			ts.nets = append(ts.nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			return nil, err
		}
		ts.nets = append(ts.nets, n)
	}
	return ts, nil
}

// SetTrustedProxies replaces the process-wide trusted proxy set (used by apps
// that do not set Config.TrustedProxies). Entries are IPs, CIDRs or "private".
//
// Convention (no call needed): loopback + private networks are trusted. Call
// it with CIDRs of a public edge (Cloudflare, an internet-facing LB), or with
// no arguments to trust no proxy at all.
func SetTrustedProxies(cidrs ...string) error {
	ts, err := parseTrust(cidrs)
	if err != nil {
		return err
	}
	globalTrust.Store(ts)
	return nil
}

// ResetTrustedProxies restores the convention default (private networks).
func ResetTrustedProxies() { globalTrust.Store(defaultTrust) }

// trust returns the trust set for this request: the app's own
// (Config.TrustedProxies) or the process-wide one.
func (c *FastCtx) trust() *trustSet {
	if c.app != nil && c.app.trusted != nil {
		return c.app.trusted
	}
	return globalTrust.Load()
}

func (c *FastCtx) peerMaySetForwardedHeaders(ipStr string) bool {
	return c.trust().contains(net.ParseIP(ipStr))
}
