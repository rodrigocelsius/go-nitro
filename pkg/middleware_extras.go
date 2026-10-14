package nitro

import (
	nitroadapter "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"
)

// Skip runs mw for every request except those where skip returns true
// (Fiber's `Next`, Echo's `Skipper`).
//
//	app.Use(nitro.Skip(func(c *nitro.Ctx) bool { return c.Path() == "/health" }, auth))
func Skip(skip func(c *Ctx) bool, mw Handler) Handler {
	return func(c *Ctx) error {
		if skip != nil && skip(c) {
			return nil
		}
		return mw(c)
	}
}

// BasicAuth protects routes with HTTP Basic credentials (username → password).
// Constant-time comparison; the user name is available as c.Get("user").
//
//	app.Use(nitro.BasicAuth(map[string]string{"admin": os.Getenv("ADMIN_PASSWORD")}))
func BasicAuth(users map[string]string, realm ...string) Handler {
	return wrapAdapter(nitroadapter.BasicAuthMiddleware(users, realm...))
}

// APIKeyAuth accepts requests carrying one of keys in X-API-Key or as a
// Bearer token (constant-time comparison).
//
//	app.Use(nitro.APIKeyAuth(os.Getenv("API_KEY")))
func APIKeyAuth(keys ...string) Handler {
	return wrapAdapter(nitroadapter.APIKeyMiddleware(keys...))
}

// RedirectHTTPS redirects plain-HTTP requests to HTTPS with 308. Loopback
// hosts are never redirected and X-Forwarded-Proto is honored only from
// trusted proxies (Config.TrustedProxies; private ranges by default).
func RedirectHTTPS() Handler {
	return wrapAdapter(nitroadapter.RedirectHTTPSMiddleware())
}

// ETag adds weak ETags to successful GET/HEAD responses and answers 304 when
// If-None-Match matches (chainable, post-middleware). Call it BEFORE
// app.Compress() so the tag is computed on the uncompressed body.
func (a *App) ETag() *App {
	return a.UsePostDomain(nitroadapter.ETagMiddleware())
}

// ResponseTime adds a Server-Timing header ("app;dur=…") to every response.
func (a *App) ResponseTime() *App {
	pre, post := nitroadapter.ResponseTimePair()
	a.inner.Use(pre)
	a.inner.UsePost(post)
	return a
}
