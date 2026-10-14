package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAuthJWTStub(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("go.mod", []byte("module example.com/shop\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll("internal/config", 0755)
	_ = os.MkdirAll("cmd/server", 0755)
	cfg := `package config

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Config struct {
	Port int ` + "`env:\"PORT\" default:\"8080\"`" + `
}

func Load() (Config, error) { return nitro.ConfigOf[Config]() }
`
	_ = os.WriteFile("internal/config/config.go", []byte(cfg), 0644)
	main := `package main

import (
	"fmt"
	"log"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"example.com/shop/internal/config"
	"example.com/shop/internal/handler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	app := nitro.New(nitro.Config{Env: nitro.EnvDevelopment})
	handler.Register(app)
	nitro.Ok(app.Listen(fmt.Sprintf(":%d", cfg.Port)))
}
`
	_ = os.WriteFile("cmd/server/main.go", []byte(main), 0644)
	_ = os.MkdirAll("internal/handler", 0755)
	_ = os.WriteFile("internal/handler/routes.go", []byte("package handler\nimport nitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\nfunc Register(app *nitro.App) {}\n"), 0644)

	installAuthJWT()

	b, err := os.ReadFile("internal/auth/jwt.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "SetupJWT") || !strings.Contains(string(b), "AuthRoutes") {
		t.Fatalf("stub: %s", b)
	}
	if !strings.Contains(string(b), "RequireRole") {
		t.Fatal("expected RequireRole sample")
	}
	if !strings.Contains(string(b), "/admin") {
		t.Fatal("expected /admin dashboard")
	}
	mainB, _ := os.ReadFile("cmd/server/main.go")
	if !strings.Contains(string(mainB), "auth.SetupJWT") {
		t.Fatalf("main not wired: %s", mainB)
	}
	envB, _ := os.ReadFile(".env")
	if !strings.Contains(string(envB), "JWT_SECRET=") {
		t.Fatalf(".env: %s", envB)
	}
	cfgB, _ := os.ReadFile(filepath.Join("internal/config/config.go"))
	if !strings.Contains(string(cfgB), "JWTSecret") {
		t.Fatalf("config: %s", cfgB)
	}
}
