package main

import (
	"fmt"
	"log"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/config"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	repo := repository.NewProductMemory()
	app := nitro.New()
	handler.Register(app, repo)

	nitro.Ok(app.Listen(fmt.Sprintf(":%d", cfg.Port)))
}
