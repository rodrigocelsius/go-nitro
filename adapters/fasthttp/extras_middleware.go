package fasthttp

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// ── Skip ─────────────────────────────────────────────────────────────

// SkipMiddleware runs mw unless skip(c) is true (Fiber's skip / Echo's Skipper).
//
//	app.Use(SkipMiddleware(func(c *domain.Ctx) bool { return c.Path() == "/health" }, auth))
func SkipMiddleware(skip func(c *domain.Ctx) bool, mw domain.Handler) domain.Handler {
	return func(c *domain.Ctx) error {
		if skip != nil && skip(c) {
			return nil
		}
		return mw(c)
	}
}

// ── ETag ─────────────────────────────────────────────────────────────

// ETagMiddleware adds a weak ETag to successful GET/HEAD responses and answers
// 304 Not Modified when the client's If-None-Match matches. Register it as a
// post-middleware BEFORE Compress so the tag is computed on the plain body.
func ETagMiddleware() domain.Handler {
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		fctx := fc.FasthttpCtx()
		m := fc.Method()
		if (m != "GET" && m != "HEAD") || fctx.Response.StatusCode() != 200 {
			return nil
		}
		if len(fctx.Response.Header.Peek("ETag")) > 0 {
			return nil
		}
		body := fctx.Response.Body()
		if len(body) == 0 {
			return nil
		}
		h := fnv.New64a()
		_, _ = h.Write(body)
		tag := `W/"` + strconv.Itoa(len(body)) + "-" + strconv.FormatUint(h.Sum64(), 36) + `"`
		fctx.Response.Header.Set("ETag", tag)
		if inm := fc.Header("If-None-Match"); inm != "" && etagMatches(inm, tag) {
			fctx.Response.ResetBody()
			fctx.SetStatusCode(304)
		}
		return nil
	}
}

func etagMatches(ifNoneMatch, tag string) bool {
	if strings.TrimSpace(ifNoneMatch) == "*" {
		return true
	}
	for _, part := range strings.Split(ifNoneMatch, ",") {
		p := strings.TrimSpace(part)
		if p == tag || strings.TrimPrefix(p, "W/") == strings.TrimPrefix(tag, "W/") {
			return true
		}
	}
	return false
}

// ── Server-Timing ────────────────────────────────────────────────────

const localResponseTimeStart = "_rt_start"

// ResponseTimePair returns (pre, post) middlewares that add a
// `Server-Timing: app;dur=<ms>` header (visible in browser dev tools).
func ResponseTimePair() (pre, post domain.Handler) {
	pre = func(c *domain.Ctx) error {
		c.SetLocal(localResponseTimeStart, time.Now().UnixNano())
		return nil
	}
	post = func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		start, ok := c.Local(localResponseTimeStart).(int64)
		if !ok {
			return nil
		}
		ms := float64(time.Now().UnixNano()-start) / 1e6
		fc.FasthttpCtx().Response.Header.Set("Server-Timing", "app;dur="+strconv.FormatFloat(ms, 'f', 2, 64))
		return nil
	}
	return pre, post
}

// errResponseWritten is returned by a pre-middleware that already wrote the
// full response (redirect) to stop the pipeline; handleError ignores it.
var errResponseWritten error = responseWritten{}

type responseWritten struct{}

func (responseWritten) Error() string   { return "nitro: response already written" }
func (responseWritten) StatusCode() int { return 308 }
func (responseWritten) Body() any       { return nil }

// ── HTTPS redirect ───────────────────────────────────────────────────

// RedirectHTTPSMiddleware redirects plain-HTTP requests to https:// with 308
// (method preserved). Convention: loopback hosts are never redirected (local
// development) and X-Forwarded-Proto is honored only from trusted proxies, so
// it works behind a TLS-terminating load balancer without configuration.
func RedirectHTTPSMiddleware() domain.Handler {
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		if fc.Protocol() == "https" {
			return nil
		}
		host := fc.Hostname()
		bare := host
		if i := strings.LastIndex(bare, ":"); i > 0 && !strings.Contains(bare[i:], "]") {
			bare = bare[:i]
		}
		if bare == "" || bare == "localhost" || bare == "127.0.0.1" || bare == "[::1]" || bare == "::1" {
			return nil
		}
		target := "https://" + host + string(fc.FasthttpCtx().RequestURI())
		fc.FasthttpCtx().Redirect(target, 308)
		return errResponseWritten
	}
}

// ── Basic auth / API key ─────────────────────────────────────────────

// BasicAuthMiddleware protects routes with HTTP Basic credentials taken from
// users (username → password). Comparison is constant-time. The authenticated
// username is stored with c.Set("user", name) / c.Get("user").
func BasicAuthMiddleware(users map[string]string, realm ...string) domain.Handler {
	r := "Restricted"
	if len(realm) > 0 && realm[0] != "" {
		r = realm[0]
	}
	hashed := make(map[string][32]byte, len(users))
	for u, p := range users {
		hashed[u] = sha256.Sum256([]byte(p))
	}
	challenge := `Basic realm="` + strings.ReplaceAll(r, `"`, "") + `", charset="UTF-8"`
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return domain.ErrUnauthorized
		}
		deny := func() error {
			fc.SetHeader("WWW-Authenticate", challenge)
			return domain.ErrUnauthorized
		}
		auth := fc.Header("Authorization")
		const prefix = "Basic "
		if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
			return deny()
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(auth[len(prefix):]))
		if err != nil {
			return deny()
		}
		user, pass, found := strings.Cut(string(raw), ":")
		if !found {
			return deny()
		}
		want, known := hashed[user]
		got := sha256.Sum256([]byte(pass))
		// Always compare, even for unknown users, to keep timing uniform.
		if subtle.ConstantTimeCompare(want[:], got[:]) != 1 || !known {
			return deny()
		}
		fc.Set("user", user)
		return nil
	}
}

// APIKeyMiddleware authenticates a request by API key. Convention: the key is
// read from the `X-API-Key` header (the same header the generated OpenAPI spec
// documents), falling back to `Authorization: Bearer <key>`. Keys are compared
// in constant time. Pass the accepted keys (e.g. loaded from the environment).
func APIKeyMiddleware(keys ...string) domain.Handler {
	hashes := make([][32]byte, 0, len(keys))
	for _, k := range keys {
		if k != "" {
			hashes = append(hashes, sha256.Sum256([]byte(k)))
		}
	}
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil || len(hashes) == 0 {
			return domain.ErrUnauthorized
		}
		key := strings.TrimSpace(fc.Header("X-API-Key"))
		if key == "" {
			if a := fc.Header("Authorization"); len(a) > 7 && strings.EqualFold(a[:7], "Bearer ") {
				key = strings.TrimSpace(a[7:])
			}
		}
		if key == "" {
			return domain.ErrUnauthorized
		}
		got := sha256.Sum256([]byte(key))
		match := 0
		for _, h := range hashes {
			match |= subtle.ConstantTimeCompare(h[:], got[:])
		}
		if match != 1 {
			return domain.ErrUnauthorized
		}
		fc.Set("api_key_id", hex.EncodeToString(got[:4])) // short fingerprint, never the key
		return nil
	}
}
