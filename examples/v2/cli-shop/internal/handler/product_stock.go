package handler

import (
	"errors"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	productuc "github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/usecase/product"
)

type adjustStockInput struct {
	Delta int64 `json:"delta" validate:"required"`
}

// RegisterStock mounts POST /products/:id/stock (business action → use case).
func RegisterStock(app *nitro.App, uow productuc.UnitOfWork, repo productuc.Repository) {
	app.Post("/products/:id/stock", func(c *nitro.Ctx) error {
		in, err := nitro.Parse[adjustStockInput](c)
		if err != nil {
			return err
		}
		p, err := productuc.AdjustStock(c, uow, repo, c.Param("id"), in.Delta)
		if errors.Is(err, productuc.ErrInsufficientStock) {
			return nitro.Conflict(err.Error())
		}
		if err != nil {
			return err
		}
		return c.OK(p)
	})
}
