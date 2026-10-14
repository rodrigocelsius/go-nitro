package main

import (
	"fmt"
	"log"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cli-shop/v2/internal/config"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cli-shop/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cli-shop/v2/internal/model"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	app := nitro.New()
	repo := nitro.NewMemoryRepo[model.Invoice]()
	handler.Register(app, repo)

	handler.RegisterProduct(app, nitro.NewMemoryRepo[model.Product]())

	nitro.Ok(app.Listen(fmt.Sprintf(":%d", cfg.Port)))
}
