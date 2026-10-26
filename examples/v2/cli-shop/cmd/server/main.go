package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/config"
	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/repository"
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

	app := nitro.New(nitro.Config{Env: nitro.ParseEnv(cfg.Env), ServiceName: cfg.ServiceName})
	app.HealthCheck(nitro.NamedHealth("db", func() error {
		return db.Ping(context.Background())
	}))
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		rdb, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
		app.SetCache(nitro.RedisCache(redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       rdb,
		})))
	}
	store := sqladapter.StoreFor[model.Product](db)
	repo := repository.NewProduct(store)

	handler.Register(app, repo)
	handler.RegisterStock(app, db, repo)

	handler.RegisterCategory(app, sqladapter.StoreFor[model.Category](db))

	nitro.Ok(app.Listen(fmt.Sprintf(":%d", cfg.Port)))
}
