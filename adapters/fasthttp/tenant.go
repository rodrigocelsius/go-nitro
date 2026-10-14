package fasthttp

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// TenantConfig configures TenantMiddleware (shared-schema multi-tenancy).
//
// Resolution order when resolving a request tenant:
//  1. Existing Ctx local / JWT claim already set by JWTMiddleware (c.TenantID())
//  2. HTTP header (Header, default "X-Tenant-ID") — ONLY when AllowHeader is
//     explicitly set to true (opt-in; see the warning on AllowHeader)
//
// When Require is true and no tenant can be resolved, the middleware returns 401.
type TenantConfig struct {
	// Header is the request header used when the JWT has no tenant_id claim.
	// Empty → "X-Tenant-ID".
	Header string
	// Require, when true, rejects the request if no tenant was resolved.
	Require bool
	// AllowHeader enables the header fallback. nil/false (default) = the
	// header is never read: the tenant comes only from the authenticated
	// identity (JWT claim). SECURITY: a client-supplied tenant header lets any
	// caller pick another tenant's data (IDOR). Enable it only for trusted
	// internal callers or a gateway that strips/overwrites the header, and
	// never together with untrusted clients.
	AllowHeader *bool
}

func (cfg TenantConfig) headerName() string {
	if cfg.Header != "" {
		return cfg.Header
	}
	return "X-Tenant-ID"
}

func (cfg TenantConfig) headerAllowed() bool {
	return cfg.AllowHeader != nil && *cfg.AllowHeader
}

// TenantMiddleware resolves the current tenant into request locals so
// c.TenantID() and ResourceConfig.TenantScope work for the rest of the chain.
//
// Prefer placing it after JWTMiddleware when tenants come from claims:
//
//	app.Use(nitro.JWTMiddleware(provider))
//	app.Use(nitro.TenantMiddleware(nitro.TenantConfig{Require: true}))
//
// Header fallback is for trusted internal callers or BFF-injected headers —
// do not expose AllowHeader to untrusted clients without also validating
// membership of the claimed tenant against the authenticated user.
func TenantMiddleware(cfg ...TenantConfig) domain.Handler {
	var c TenantConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}
	header := c.headerName()
	allowHeader := c.headerAllowed()
	require := c.Require

	return func(ctx *domain.Ctx) error {
		fc, ok := ctx.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}

		id := fc.TenantID()
		if id == "" && allowHeader {
			id = strings.TrimSpace(fc.Header(header))
			if id != "" {
				fc.SetTenantID(id)
			}
		}
		if require && fc.TenantID() == "" {
			return &domain.UnauthorizedError{Message: "missing tenant"}
		}
		return nil
	}
}
