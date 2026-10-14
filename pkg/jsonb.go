package nitro

import "github.com/rodrigocelsius/go-nitro/domain/v2"

// JSONSlice is a []T stored as PostgreSQL JSONB.
type JSONSlice[T any] = domain.JSONSlice[T]

// StringSlice is JSONB []string.
type StringSlice = domain.StringSlice

// JSON wraps any value as a JSONB column (see domain.JSON).
type JSON[T any] = domain.JSON[T]
