package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdInstallAuth(kind string) {
	switch kind {
	case "auth-jwt", "jwt", "auth":
		installAuthJWT()
	case "auth-keycloak", "keycloak":
		installAuthKeycloak()
	default:
		fmt.Fprintf(os.Stderr, "nitrogen: unknown auth %q\n", kind)
		os.Exit(1)
	}
}

func installAuthJWT() {
	secret := randomDevPassword() + randomDevPassword()
	vars := map[string]string{
		"JWT_SECRET": secret,
	}
	writeInstallEnv("auth-jwt", vars)
	patchConfigAuthJWT()
	writeAuthJWTStub()
	fmt.Println("nitrogen: auth-jwt ready — register/login at /auth, protected GET /profile, admin GET /admin/dashboard (RequireRole)")
	fmt.Println("nitrogen: tip — rotate JWT_SECRET before any shared deploy")
}

func installAuthKeycloak() {
	vars := map[string]string{
		"KEYCLOAK_BASE_URL":      "http://127.0.0.1:8081",
		"KEYCLOAK_REALM":         "nitro",
		"KEYCLOAK_CLIENT_ID":     "nitro-api",
		"KEYCLOAK_CLIENT_SECRET": "changeme",
	}
	writeInstallEnv("auth-keycloak", vars)
	patchConfigAuthKeycloak()
	writeAuthKeycloakStub()
	fmt.Println("nitrogen: auth-keycloak ready — routes under /auth, protected GET /api/me")
	fmt.Println("nitrogen: tip — set KEYCLOAK_* to your realm")
}

func patchConfigAuthJWT() {
	path := configPath()
	b, err := os.ReadFile(path)
	if err != nil {
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		content := `package config

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Config struct {
	Port        int    ` + "`env:\"PORT\" default:\"8080\"`" + `
	Env         string ` + "`env:\"NITRO_ENV\" default:\"production\"`" + `
	ServiceName string ` + "`env:\"SERVICE_NAME\" default:\"app\"`" + `
	JWTSecret   string ` + "`env:\"JWT_SECRET\"`" + `
}

func Load() (Config, error) {
	// Export .env / .env.local to the process (real env vars win) so code that
	// reads os.Getenv — sqladapter.OpenEnv, OTEL, REDIS_ADDR — sees the same
	// values as Config. No-op when the files are absent (containers).
	nitro.LoadDotEnv()
	return nitro.ConfigOf[Config]()
}
`
		if err := writeFile(path, content); err != nil {
			fatalWrite(err)
		}
		return
	}
	src := string(b)
	if strings.Contains(src, "JWTSecret") || strings.Contains(src, "JWT_SECRET") {
		return
	}
	field := "\tJWTSecret   string `env:\"JWT_SECRET\"`\n"
	idx := strings.Index(src, "type Config struct {")
	if idx >= 0 {
		rest := src[idx:]
		end := strings.Index(rest, "\n}")
		if end >= 0 {
			insertAt := idx + end
			src = src[:insertAt] + "\n" + field + src[insertAt:]
			_ = writeFileFmt(path, []byte(src), 0644)
			fmt.Println("nitrogen: config + JWTSecret")
		}
	}
}

func patchConfigAuthKeycloak() {
	path := configPath()
	b, err := os.ReadFile(path)
	if err != nil {
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		content := `package config

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Config struct {
	Port        int    ` + "`env:\"PORT\" default:\"8080\"`" + `
	Env         string ` + "`env:\"NITRO_ENV\" default:\"production\"`" + `
	ServiceName string ` + "`env:\"SERVICE_NAME\" default:\"app\"`" + `
	Keycloak    KeycloakConfig
}

func Load() (Config, error) {
	// Export .env / .env.local to the process (real env vars win) so code that
	// reads os.Getenv — sqladapter.OpenEnv, OTEL, REDIS_ADDR — sees the same
	// values as Config. No-op when the files are absent (containers).
	nitro.LoadDotEnv()
	return nitro.ConfigOf[Config]()
}

type KeycloakConfig struct {
	BaseURL      string ` + "`env:\"KEYCLOAK_BASE_URL\"`" + `
	Realm        string ` + "`env:\"KEYCLOAK_REALM\"`" + `
	ClientID     string ` + "`env:\"KEYCLOAK_CLIENT_ID\"`" + `
	ClientSecret string ` + "`env:\"KEYCLOAK_CLIENT_SECRET\"`" + `
}
`
		if err := writeFile(path, content); err != nil {
			fatalWrite(err)
		}
		return
	}
	src := string(b)
	if strings.Contains(src, "type KeycloakConfig") {
		return
	}
	field := "\tKeycloak    KeycloakConfig\n"
	idx := strings.Index(src, "type Config struct {")
	if idx >= 0 {
		rest := src[idx:]
		end := strings.Index(rest, "\n}")
		if end >= 0 {
			insertAt := idx + end
			src = src[:insertAt] + "\n" + field + src[insertAt:]
		}
	}
	src = strings.TrimSpace(src) + `

type KeycloakConfig struct {
	BaseURL      string ` + "`env:\"KEYCLOAK_BASE_URL\"`" + `
	Realm        string ` + "`env:\"KEYCLOAK_REALM\"`" + `
	ClientID     string ` + "`env:\"KEYCLOAK_CLIENT_ID\"`" + `
	ClientSecret string ` + "`env:\"KEYCLOAK_CLIENT_SECRET\"`" + `
}
`
	_ = writeFileFmt(path, []byte(src), 0644)
	fmt.Println("nitrogen: config + KeycloakConfig")
}

func configPath() string {
	if hasInternalLayout() {
		return "internal/config/config.go"
	}
	return "config/config.go"
}

func writeAuthJWTStub() {
	mod := modulePath()
	if !(hasInternalLayout() && mod != "") {
		fmt.Println("nitrogen: tip — wire NewAuth + AuthRoutes in main (see examples/v1/auth)")
		return
	}
	dir := "internal/auth"
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "jwt.go")
	body := fmt.Sprintf(`package auth

import (
	"log"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"%s/internal/config"
)

// SetupJWT registers /auth routes and returns the auth handle for JWTMiddleware.
func SetupJWT(app *nitro.App, cfg config.Config) *nitro.Auth {
	secret := cfg.JWTSecret
	if secret == "" {
		// A silent fixed fallback here would ship the exact same JWT
		// secret to every app scaffolded from this template — anyone
		// could forge tokens for any deployment that forgot to set
		// JWT_SECRET. Only tolerate the empty secret in development;
		// fail the boot everywhere else instead of a fallback (see
		// vuln.md V-22).
		if cfg.Env != "development" {
			log.Fatal("auth: JWT_SECRET is required outside development — set it in .env / the environment")
		}
		secret = "dev-only-change-me"
	}
	a := nitro.NewAuth(nitro.AuthConfig{
		JWTSecret:       secret,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}).WithMemoryStore()
	app.AuthRoutes("/auth", a)
	protected := app.Group("", nitro.JWTMiddleware(a.Provider()))
	protected.Get("/profile", func(c *nitro.Ctx) error {
		claims, err := c.Claims()
		if err != nil {
			return err
		}
		return c.OK(nitro.Map{"user_id": claims.UserID, "email": claims.Email, "roles": claims.Roles})
	})
	admin := app.Group("/admin", nitro.JWTMiddleware(a.Provider()), nitro.RequireRole("admin"))
	admin.Get("/dashboard", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"message": "admin dashboard", "user_id": c.UserID()})
	})
	return a
}
`, mod)
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
	patchMainAuthCall(mod, "SetupJWT")
}

func writeAuthKeycloakStub() {
	mod := modulePath()
	if !(hasInternalLayout() && mod != "") {
		fmt.Println("nitrogen: tip — wire NewKeycloak in main (see examples/v1/auth-keycloak)")
		return
	}
	dir := "internal/auth"
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "keycloak.go")
	body := fmt.Sprintf(`package auth

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"%s/internal/config"
)

// SetupKeycloak registers Keycloak auth routes and a protected /api/me.
func SetupKeycloak(app *nitro.App, cfg config.Config) *nitro.KeycloakService {
	kc, err := nitro.NewKeycloak(nitro.KeycloakConfig{
		BaseURL:      cfg.Keycloak.BaseURL,
		Realm:        cfg.Keycloak.Realm,
		ClientID:     cfg.Keycloak.ClientID,
		ClientSecret: cfg.Keycloak.ClientSecret,
	})
	if err != nil {
		panic("keycloak: " + err.Error())
	}
	app.KeycloakAuthRoutes("/auth", kc)
	api := app.Group("/api", nitro.JWTMiddleware(kc.Provider()))
	api.Get("/me", func(c *nitro.Ctx) error {
		claims, err := c.Claims()
		if err != nil {
			return err
		}
		return c.OK(nitro.Map{"user_id": claims.UserID, "email": claims.Email, "roles": claims.Roles})
	})
	admin := app.Group("/api/admin",
		nitro.JWTMiddleware(kc.Provider()),
		nitro.RequireRole("admin", "super_admin"),
	)
	admin.Get("/ping", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"ok": true})
	})
	return kc
}
`, mod)
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
	patchMainAuthCall(mod, "SetupKeycloak")
}

func patchMainAuthCall(mod, setupFn string) {
	path := "cmd/server/main.go"
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	src := string(b)
	if strings.Contains(src, "auth."+setupFn) {
		return
	}
	if !strings.Contains(src, "/internal/auth\"") {
		src = strings.Replace(src, fmt.Sprintf("\t\"%s/internal/handler\"\n", mod),
			fmt.Sprintf("\t\"%s/internal/auth\"\n\t\"%s/internal/handler\"\n", mod, mod), 1)
	}
	wire := fmt.Sprintf("\t_ = auth.%s(app, cfg)\n", setupFn)
	if strings.Contains(src, "handler.Register(") {
		src = strings.Replace(src, "\thandler.Register(", wire+"\thandler.Register(", 1)
	}
	_ = writeFileFmt(path, []byte(src), 0644)
	fmt.Printf("nitrogen: wired auth.%s in cmd/server/main.go\n", setupFn)
}
