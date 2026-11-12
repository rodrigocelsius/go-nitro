package nitro

import (
	"errors"
	"time"
)

// Me returns a handler for the current authenticated principal.
//
//	app.Get("/api/me", nitro.Me(), jwt)
//
// Response: { "user_id", "email", "roles" }.
func Me() Handler {
	return func(c *Ctx) error {
		claims, err := c.Claims()
		if err != nil {
			return err
		}
		email := ""
		var roles []string
		if claims != nil {
			email = claims.Email
			roles = claims.Roles
		}
		return c.OK(Map{
			"user_id": c.UserID(),
			"email":   email,
			"roles":   roles,
		})
	}
}

// MeRoute registers GET /api/me with middleware (typically JWT).
//
//	app.MeRoute(jwt)
func (a *App) MeRoute(mws ...Handler) *App {
	a.Get("/api/me", Me(), mws...)
	return a
}

// AppInfo returns a standard root/status payload.
//
//	app.Get("/", nitro.AppInfo("transporte-api", "0.1.0"))
func AppInfo(name, version string) Handler {
	return func(c *Ctx) error {
		return c.OK(Map{
			"app":     name,
			"version": version,
			"status":  "operational",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// Info registers GET / with AppInfo(name, version).
func (a *App) Info(name, version string) *App {
	a.Get("/", AppInfo(name, version))
	return a
}

// JWKSRoute registers GET path (conventionally /.well-known/jwks.json)
// answering the provider's public keys as a JSON Web Key Set, so other
// services can validate the tokens. Only public keys are published; an
// HS256 provider yields an empty set.
//
//	app.JWKSRoute("/.well-known/jwks.json", jwtp)
func (a *App) JWKSRoute(path string, provider interface{ JWKS() ([]byte, error) }, mws ...Handler) *App {
	if provider == nil {
		return a.fail(errors.New("nitro: JWKSRoute requires a provider created with JWTProviderWithKeys"))
	}
	a.Get(path, func(c *Ctx) error {
		set, err := provider.JWKS()
		if err != nil {
			return err
		}
		// Short cache: clients pick up a rotated key within minutes.
		c.SetHeader("Cache-Control", "public, max-age=300")
		return c.Write(200, set)
	}, mws...)
	return a
}
