package nitro

import (
	"fmt"
	"strconv"
	"sync/atomic"

	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

var globalSeq atomic.Int64

// NextID returns a global atomic sequential ID (useful for in-memory repos).
func NextID() int64 {
	return globalSeq.Add(1)
}

// NextIDString returns NextID() as a string.
func NextIDString() string {
	return strconv.FormatInt(NextID(), 10)
}

// IDKey normalizes any id to a map/cache key.
func IDKey(id any) string {
	return fmt.Sprint(id)
}

// NewUUID generates a UUID v7.
func NewUUID() nitrouuid.UUID {
	return nitrouuid.New()
}

// NewUUIDString generates a UUID v7 as a string.
func NewUUIDString() string {
	return nitrouuid.NewString()
}

// ParseUUID parses a canonical UUID string.
func ParseUUID(s string) (nitrouuid.UUID, error) {
	return nitrouuid.Parse(s)
}

// MustParseUUID panics if s is invalid.
func MustParseUUID(s string) nitrouuid.UUID {
	u, err := nitrouuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
