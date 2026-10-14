package usecases

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// HandleError converts an error into an HTTP response.
// If the error implements HTTPError, it uses its StatusCode and Body.
// Otherwise it returns 500 with a generic message.
// In development (Config.Env or NITRO_ENV), the 500 JSON includes request_id
// and a short hint (correlate logs; quiet OTEL with NITRO_OTEL=0).
func HandleError(c *domain.Ctx, err error) {
	HandleErrorEnv(c, err, envFromProcess())
}

// HandleErrorEnv is like HandleError but uses an explicit runtime Env
// (preferred when the App knows Config.Env).
func HandleErrorEnv(c *domain.Ctx, err error, env domain.Env) {
	if httpErr, ok := err.(domain.HTTPError); ok {
		c.SetStatus(httpErr.StatusCode())
		c.SetHeader("Content-Type", "application/json")
		data, marshalErr := json.Marshal(httpErr.Body())
		if marshalErr != nil {
			c.SetBody([]byte(`{"error":"internal server error"}`))
			return
		}
		c.SetBody(data)
		return
	}
	c.SetStatus(500)
	c.SetHeader("Content-Type", "application/json")
	if env == domain.EnvDevelopment {
		payload := map[string]string{
			"error": "internal server error",
			"hint":  "dev: check server logs; set NITRO_OTEL=0 to quiet telemetry noise",
		}
		if rid := c.RequestID(); rid != "" {
			payload["request_id"] = rid
		}
		data, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			c.SetBody([]byte(`{"error":"internal server error"}`))
			return
		}
		c.SetBody(data)
		return
	}
	c.SetBody([]byte(`{"error":"internal server error"}`))
}

func envFromProcess() domain.Env {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("NITRO_ENV")))
	switch v {
	case "development", "dev", "local":
		return domain.EnvDevelopment
	default:
		return domain.EnvProduction
	}
}
