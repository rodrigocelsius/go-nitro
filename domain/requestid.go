package domain

import "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"

const RequestIDHeader = "X-Request-ID"

// GenerateRequestID returns a UUIDv7 (time-ordered, RFC 9562) request id, so
// logs and traces sort chronologically by id.
func GenerateRequestID() string {
	return uuid.NewString()
}
