// Order business example — app.Resource CRUD + Commands/Rules/Queries on
// top of Postgres + BLinq, with multi-table writes (orders + invoices) via
// sqladapter.DB.Tx.
//
// Run:
//
//	docker compose up -d
//	cp .env.example .env
//	go run ./cmd/server
//
// See README.md for the full request walkthrough (register → login →
// place → submit → approve/cancel → queries/pending).
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/config"
	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	db, err := sqladapter.OpenEnv(ctx)
	if err != nil {
		log.Fatalf("db: %v (set DATABASE_URL / DB_DSN and NITRO_DB_DRIVER)", err)
	}
	defer db.Close()
	if err := db.MigrateUp(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		// Development-only fallback so `go run ./cmd/server` works out of
		// the box — never do this in a real app. Set JWT_SECRET (see
		// .env.example) in anything beyond local development.
		jwtSecret = "dev-only-insecure-secret-change-me"
	}
	auth := nitro.NewAuth(nitro.AuthConfig{
		JWTSecret:      jwtSecret,
		AccessTokenTTL: 30 * time.Minute,
	}).WithMemoryStore()

	app := nitro.New(nitro.Config{Env: nitro.ParseEnv(cfg.Env), ServiceName: cfg.ServiceName})
	app.HealthCheck(nitro.NamedHealth("db", func() error {
		return db.Ping(context.Background())
	}))
	app.AuthRoutes("/auth", auth)

	store := sqladapter.StoreOf[model.Order](db, "orders")
	handler.Register(app, db, store, nitro.JWTMiddleware(auth.Provider()))

	nitro.Ok(app.Listen(fmt.Sprintf(":%d", cfg.Port)))
}
