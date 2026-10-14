package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
	"github.com/redis/go-redis/v9"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2/internal/config"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2/internal/model"
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
	store := sqladapter.StoreOf[model.Product](db, "products")
	handler.Register(app, store)

	nitro.Ok(app.Listen(fmt.Sprintf(":%d", cfg.Port)))
}
