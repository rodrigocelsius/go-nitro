package domain

// HealthChecker checks a component's health.
type HealthChecker interface {
	Name() string
	Check() error
}

// HealthStatus represents overall health status.
type HealthStatus struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks,omitempty"`
	Timestamp int64             `json:"timestamp"`
}
