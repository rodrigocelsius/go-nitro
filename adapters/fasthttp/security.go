package fasthttp

import "github.com/rodrigocelsius/go-nitro/domain/v2"

const defaultCSP = "default-src 'self'"

var (
	headerXContentTypeOptions = []byte("X-Content-Type-Options")
	headerXFrameOptions       = []byte("X-Frame-Options")
	headerReferrerPolicy      = []byte("Referrer-Policy")
	headerCSP                 = []byte("Content-Security-Policy")
	headerHSTS                = []byte("Strict-Transport-Security")
	headerXXSSProtection      = []byte("X-Xss-Protection")
	headerXPermittedCross     = []byte("X-Permitted-Cross-Domain-Policies")
	headerPermissionsPolicy   = []byte("Permissions-Policy")

	valueNosniff  = []byte("nosniff")
	valueDeny     = []byte("DENY")
	valueReferrer = []byte("strict-origin-when-cross-origin")
	valueHSTS     = []byte("max-age=31536000; includeSubDomains; preload")
	valueXXSS     = []byte("1; mode=block")
	valueNone     = []byte("none")
)

// securityHeadersMiddleware returns a domain.Handler that writes default
// security headers on the response. Uses AdapterData to access
// *fasthttp.RequestCtx without a type assertion.
func securityHeadersMiddleware(cfg domain.SecurityConfig, env domain.Env) domain.Handler {
	hstsEnabled := !cfg.DisableHSTS && env != domain.EnvDevelopment
	csp := []byte(defaultCSP)
	if cfg.CSP != "" {
		csp = []byte(cfg.CSP)
	}

	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx).FasthttpCtx()
		h := &fctx.Response.Header
		h.SetCanonical(headerXContentTypeOptions, valueNosniff)
		h.SetCanonical(headerXFrameOptions, valueDeny)
		h.SetCanonical(headerReferrerPolicy, valueReferrer)
		h.SetCanonical(headerCSP, csp)
		h.SetCanonical(headerXXSSProtection, valueXXSS)
		h.SetCanonical(headerXPermittedCross, valueNone)
		h.SetCanonical(headerPermissionsPolicy, valueNone)
		if hstsEnabled {
			h.SetCanonical(headerHSTS, valueHSTS)
		}
		return nil
	}
}

// SecurityHeadersMiddleware is the public version of the security middleware.
// Use it explicitly when the developer wants full control.
func SecurityHeadersMiddleware() domain.Handler {
	return securityHeadersMiddleware(domain.SecurityConfig{}, domain.EnvProduction)
}

// SecurityHeadersWithConfig customizes security headers.
func SecurityHeadersWithConfig(cfg domain.SecurityConfig) domain.Handler {
	return securityHeadersMiddleware(cfg, domain.EnvProduction)
}

// applySecurityDefaults injects the security middleware as a global
// pre-middleware unless explicitly disabled.
func (a *App) applySecurityDefaults(cfg domain.Config) {
	if cfg.Security.DisableHeaders {
		return
	}
	a.globalPre = append(a.globalPre, securityHeadersMiddleware(cfg.Security, cfg.Env))
}
