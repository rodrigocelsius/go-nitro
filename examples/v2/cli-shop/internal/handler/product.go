package handler

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
	productuc "github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/usecase/product"
)

type ProductHandler struct {
	repo productuc.Repository
}

func (h *ProductHandler) List(c *nitro.Ctx) error {
	page := c.Int("page", 1)
	per := c.Int("per_page", 20)
	items, err := productuc.List(c, h.repo, productuc.Filter{
		Q:       c.Query("q"),
		Sort:    c.Query("sort"),
		Page:    page,
		PerPage: per,
	})
	if err != nil {
		return err
	}
	return c.OK(nitro.Map{"items": items, "total": len(items), "page": page, "per_page": per})
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
