package nitro

import (
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
