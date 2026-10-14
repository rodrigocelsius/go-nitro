package fasthttp

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
)

// JWTMiddleware returns a middleware that validates JWT Bearer tokens.
//
// Usage:
//
//	app.Get("/protected", handler, nitro.JWTMiddleware(tokenProvider))
func JWTMiddleware(provider ports.TokenProvider) domain.Handler {
	return func(c *domain.Ctx) error {
		auth := c.HeaderParam("Authorization")
		if auth == "" {
			return &domain.UnauthorizedError{Message: "missing authorization header"}
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return &domain.UnauthorizedError{Message: "invalid authorization format"}
		}

		claims, err := provider.ValidateToken(c.Context(), parts[1])
		if err != nil {
			return &domain.UnauthorizedError{Message: "invalid or expired token"}
		}

		fctx := c.AdapterData.(*FastCtx)
		fctx.Set("user_id", claims.UserID)
		fctx.Set("email", claims.Email)
		fctx.Set("roles", claims.Roles)
		fctx.Set("claims", claims)
		if claims.TenantID != "" {
			fctx.Set("tenant_id", claims.TenantID)
		}

		return nil
	}
}

// SessionMiddleware returns a middleware that authenticates via session cookie.
func SessionMiddleware(store domain.SessionStore, cookieName string) domain.Handler {
	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx)
		sessionID := fctx.Cookie(cookieName)
		if sessionID == "" {
			return &domain.UnauthorizedError{Message: "no session"}
		}

		sess, err := store.Get(c.Context(), sessionID)
		if err != nil {
			return &domain.UnauthorizedError{Message: "invalid session"}
		}

		fctx.Set("session", sess)
		fctx.Set("user_id", sess.UserID)
		if userID, ok := sess.Data["user_id"].(string); ok {
			fctx.Set("user_id", userID)
		}
		if tenantID, ok := sess.Data["tenant_id"].(string); ok && tenantID != "" {
			fctx.Set("tenant_id", tenantID)
		}

		return nil
	}
}

// RequireRole returns a middleware that checks if the user has one of the required roles.
// Must be used after JWTMiddleware or SessionMiddleware.
func RequireRole(roles ...string) domain.Handler {
	return RequireRoles(roles)
}

type requireRoleOptions struct {
	bypass []string
}

// RequireRoleOption configures RequireRoles.
type RequireRoleOption func(*requireRoleOptions)

// WithBypassRoles allows listed roles to pass even if not in the required set
// (ex.: super_admin bypasses RequireRoles([]string{"admin"}, WithBypassRoles("super_admin"))).
func WithBypassRoles(roles ...string) RequireRoleOption {
	return func(o *requireRoleOptions) {
		o.bypass = append(o.bypass, roles...)
	}
}

// RequireRoles is RequireRole with optional bypass roles.
func RequireRoles(roles []string, opts ...RequireRoleOption) domain.Handler {
	cfg := requireRoleOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}
	required := append([]string(nil), roles...)
	bypass := append([]string(nil), cfg.bypass...)
	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx)
		userRoles, ok := fctx.Get("roles").([]string)
		if !ok {
			return &domain.ForbiddenError{Message: "no roles found"}
		}
		for _, userRole := range userRoles {
			for _, b := range bypass {
				if userRole == b {
					return nil
				}
			}
			for _, req := range required {
				if userRole == req {
					return nil
				}
			}
		}
		return &domain.ForbiddenError{Message: "insufficient permissions"}
	}
}
