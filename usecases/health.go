package usecases

import (
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// RunHealthChecks runs all registered checkers.
// Returns HealthStatus with the result of each checker.
func RunHealthChecks(checkers []domain.HealthChecker) domain.HealthStatus {
	status := domain.HealthStatus{
		Status:    "healthy",
		Checks:    make(map[string]string, len(checkers)),
		Timestamp: time.Now().UnixMilli(),
	}

	for _, checker := range checkers {
		if err := checker.Check(); err != nil {
			status.Status = "unhealthy"
			status.Checks[checker.Name()] = err.Error()
		} else {
			status.Checks[checker.Name()] = "ok"
		}
	}

	return status
}

// ReadinessCheck reports whether the service is ready to receive traffic.
// Returns true if all checkers passed.
func ReadinessCheck(checkers []domain.HealthChecker) (bool, domain.HealthStatus) {
	status := RunHealthChecks(checkers)
	return status.Status == "healthy", status
}
