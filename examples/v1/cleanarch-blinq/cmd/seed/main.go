package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2/internal/seed"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	_ = os.Setenv("NITRO_DB_QUIET", "1")
	db, err := sqladapter.OpenEnv(ctx)
	if err != nil {
		log.Fatalf("db: %v (set NITRO_DB_DRIVER + DATABASE_URL)", err)
	}
	defer db.Close()
	if err := seed.SeedProduct(ctx, db); err != nil {
		log.Fatalf("%s: %v", "SeedProduct", err)
	}
	log.Println("SeedProduct ok")
	log.Println("seed complete")
}
