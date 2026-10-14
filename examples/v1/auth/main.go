// Auth example — Nitro v1 syntax.
//
// Demonstrates: NewAuth + AuthRoutes + Claims/UserID + Defaults.
//
// Run:
//
//	go run main.go
//
// Test:
//
//	curl -X POST http://localhost:8080/auth/register -d '{"email":"user@test.com","password":"12345678","name":"John"}'
//	curl -X POST http://localhost:8080/auth/login -d '{"email":"user@test.com","password":"12345678"}'
//	curl -H "Authorization: Bearer <token>" http://localhost:8080/profile
package main

import (
	"os"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// Development-only fallback so `go run main.go` works out of the
		// box — never do this in a real app. Always set JWT_SECRET via
		// the environment/config in anything beyond local development.
		jwtSecret = "dev-only-insecure-secret-change-me"
	}
	auth := nitro.NewAuth(nitro.AuthConfig{
		JWTSecret:       jwtSecret,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}).WithMemoryStore()

	app := nitro.New()
	app.AuthRoutes("/auth", auth)

	protected := app.Group("", nitro.JWTMiddleware(auth.Provider()))
	protected.Get("/profile", profileHandler)

	admin := app.Group("/admin", nitro.JWTMiddleware(auth.Provider()), nitro.RequireRole("admin"))
	admin.Get("/dashboard", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"message": "admin dashboard", "user_id": c.UserID()})
	})

	nitro.Ok(app.Listen(":8080"))
}

func profileHandler(c *nitro.Ctx) error {
	claims, err := c.Claims()
	if err != nil {
		return err
	}
	return c.OK(nitro.Map{
		"user_id": claims.UserID,
		"email":   claims.Email,
		"roles":   claims.Roles,
	})
}
