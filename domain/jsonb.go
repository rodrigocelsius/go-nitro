package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// JSONSlice is a []T persisted as JSON (driver.Valuer / sql.Scanner).
// encoding/json treats it like a normal slice. SQL column type depends on dialect
// (Postgres JSONB, MySQL JSON, SQLite TEXT, SQL Server NVARCHAR).
//
//	Missions nitro.JSONSlice[string] `db:"missions"`
//	// or: nitro.StringSlice
type JSONSlice[T any] []T

// Value implements driver.Valuer.
func (s JSONSlice[T]) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]T(s))
}

// Scan implements sql.Scanner.
func (s *JSONSlice[T]) Scan(value any) error {
	if s == nil {
		return errors.New("jsonb: Scan on nil *JSONSlice")
	}
	b, err := jsonbBytes(value, true)
	if err != nil {
		return err
	}
	var out []T
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*s = JSONSlice[T](out)
	return nil
}

// StringSlice is JSONB []string.
type StringSlice = JSONSlice[string]

// JSON wraps any value as a JSON column (objects, maps, nested structs).
// Prefer JSONSlice for arrays when you want []T field semantics.
//
//	Meta nitro.JSON[map[string]any] `db:"meta"`
type JSON[T any] struct {
	V T `json:"-"`
}

// MarshalJSON serializes the inner value (not a {"V":…} wrapper).
func (j JSON[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.V)
}

// UnmarshalJSON populates the inner value.
func (j *JSON[T]) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &j.V)
}

// Value implements driver.Valuer.
func (j JSON[T]) Value() (driver.Value, error) {
	return json.Marshal(j.V)
}

// Scan implements sql.Scanner.
func (j *JSON[T]) Scan(value any) error {
	if j == nil {
		return errors.New("jsonb: Scan on nil *JSON")
	}
	b, err := jsonbBytes(value, false)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &j.V)
}

func jsonbBytes(value any, emptyArray bool) ([]byte, error) {
	if value == nil {
		if emptyArray {
			return []byte("[]"), nil
		}
		return []byte("null"), nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			if emptyArray {
				return []byte("[]"), nil
			}
			return []byte("null"), nil
		}
		return v, nil
	case string:
		if v == "" {
			if emptyArray {
				return []byte("[]"), nil
			}
			return []byte("null"), nil
		}
		return []byte(v), nil
	case []any, map[string]any:
		return json.Marshal(v)
	default:
		// pgx / drivers may already decode JSON into Go values.
		if b, err := json.Marshal(v); err == nil {
			return b, nil
		}
		return nil, fmt.Errorf("jsonb: unsupported Scan type %T", value)
	}
}
