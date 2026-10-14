// Auth Keycloak example — Nitro v1.
//
// Demonstrates:
//   - nitro.NewKeycloak (JWKS TokenProvider + login/refresh)
//   - JWTMiddleware + RequireRole with realm/client roles flattened
//
// Requires a running Keycloak (or mock). Defaults assume:
//
//	KEYCLOAK_BASE_URL=http://localhost:8080
//	KEYCLOAK_REALM=scheduler
//	KEYCLOAK_CLIENT_ID=scheduler-api
//	KEYCLOAK_CLIENT_SECRET=…
package main

import (
	"log"
	"os"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
	cfg := nitro.KeycloakConfig{
		BaseURL:      env("KEYCLOAK_BASE_URL", "http://localhost:8080"),
		Realm:        env("KEYCLOAK_REALM", "scheduler"),
		ClientID:     env("KEYCLOAK_CLIENT_ID", "scheduler-api"),
		ClientSecret: os.Getenv("KEYCLOAK_CLIENT_SECRET"),
	}

	kc, err := nitro.NewKeycloak(cfg)
	if err != nil {
		log.Fatalf("keycloak: %v", err)
	}

	app := nitro.New()

	app.KeycloakAuthRoutes("/auth", kc)

	api := app.Group("/api", nitro.JWTMiddleware(kc.Provider()))
	api.Get("/me", nitro.Me())

	admin := app.Group("/api/admin",
		nitro.JWTMiddleware(kc.Provider()),
		nitro.RequireRole("admin", "super_admin"),
	)
	admin.Get("/ping", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"ok": true})
	})

	nitro.Ok(app.Listen(":8081"))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
