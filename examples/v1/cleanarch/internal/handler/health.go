package handler

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

func Health(c *nitro.Ctx) error {
	return c.OK(nitro.Map{"status": "ok"})
}
