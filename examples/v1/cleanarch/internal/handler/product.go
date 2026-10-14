package handler

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/model"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/repository"
	productuc "github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/usecase/product"
)

type ProductHandler struct {
	repo *repository.ProductRepository
}

func (h *ProductHandler) List(c *nitro.Ctx) error {
	items, err := productuc.List(c, h.repo)
	if err != nil {
		return err
	}
	return c.OK(nitro.Map{"items": items, "total": len(items)})
}

func (h *ProductHandler) Get(c *nitro.Ctx) error {
	p, err := productuc.Get(c, h.repo, c.Param("id"))
	if err != nil {
		return err
	}
	return c.OK(p)
}

func (h *ProductHandler) Create(c *nitro.Ctx) error {
	in, err := nitro.Parse[model.Product](c)
	if err != nil {
		return err
	}
	p, err := productuc.Create(c, h.repo, &in)
	if err != nil {
		return err
	}
	return c.Created(p)
}

func (h *ProductHandler) Update(c *nitro.Ctx) error {
	in, err := nitro.Parse[model.Product](c)
	if err != nil {
		return err
	}
	in.ID, err = nitro.ParseUUID(c.Param("id"))
	if err != nil {
		return nitro.BadRequest("invalid id")
	}
	p, err := productuc.Update(c, h.repo, &in)
	if err != nil {
		return err
	}
	return c.OK(p)
}

func (h *ProductHandler) Delete(c *nitro.Ctx) error {
	if err := productuc.Delete(c, h.repo, c.Param("id")); err != nil {
		return err
	}
	return c.NoContent()
}

func (h *ProductHandler) Discount(c *nitro.Ctx) error {
	body, err := nitro.Parse[struct {
		Percent float64 `json:"percent" validate:"required,gt=0,lte=100"`
	}](c)
	if err != nil {
		return err
	}
	p, err := productuc.ApplyDiscount(c, h.repo, c.Param("id"), body.Percent)
	if err != nil {
		return err
	}
	return c.OK(p)
}
