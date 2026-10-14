package fasthttp

import (
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

// HealthCheck registers a health checker.
// On the first checker, /health, /ready, /live routes are created automatically.
func (a *App) HealthCheck(checker domain.HealthChecker) *App {
	a.healthCheckers = append(a.healthCheckers, checker)
	if len(a.healthCheckers) == 1 {
		a.RegisterHealthRoutes()
	}
	return a
}

// RegisterHealthRoutes registers /health, /ready, /live.
func (a *App) RegisterHealthRoutes() {
	a.Get("/health", func(c *domain.Ctx) error {
		status := usecases.RunHealthChecks(a.healthCheckers)
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		if status.Status == "unhealthy" {
			return fc.JSON(503, status)
		}
		return fc.JSON(200, status)
	})

	a.Get("/ready", func(c *domain.Ctx) error {
		ready, status := usecases.ReadinessCheck(a.healthCheckers)
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		if !ready {
			return fc.JSON(503, status)
		}
		return fc.JSON(200, status)
	})

	a.Get("/live", func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		return fc.JSON(200, map[string]string{"status": "alive"})
	})
}
