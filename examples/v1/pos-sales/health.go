package main

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

// Health is a plain liveness check — GET /health.
func Health(c *nitro.Ctx) error {
	return c.OK(nitro.Map{"status": "ok"})
}
