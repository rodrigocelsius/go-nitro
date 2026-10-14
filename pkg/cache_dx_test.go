package nitro_test

import (
	"errors"
	"testing"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type memCache struct {
	data map[string]any
}

func (m *memCache) Get(key string, dest any) error {
	v, ok := m.data[key]
	if !ok {
		return errors.New("miss")
	}
	switch d := dest.(type) {
	case *[]string:
		*d = append([]string(nil), v.([]string)...)
		return nil
	case *string:
		*d = v.(string)
		return nil
	default:
		return errors.New("unsupported dest")
	}
}

func (m *memCache) Set(key string, value any, ttl time.Duration) error {
	if m.data == nil {
		m.data = map[string]any{}
	}
	m.data[key] = value
	return nil
}

func (m *memCache) Delete(key string) error {
	delete(m.data, key)
	return nil
}

func (m *memCache) Invalidate(pattern string) error {
	// simplistic: clear all for tests
	m.data = map[string]any{}
	return nil
}

func (m *memCache) Clear() error {
	m.data = map[string]any{}
	return nil
}

func TestRemember_HitAndMiss(t *testing.T) {
	c := &memCache{data: map[string]any{}}
	calls := 0
	fn := func() ([]string, error) {
		calls++
		return []string{"a"}, nil
	}
	got, err := nitro.Remember(c, "k", time.Minute, fn)
	if err != nil || len(got) != 1 || calls != 1 {
		t.Fatalf("miss: got=%v calls=%d err=%v", got, calls, err)
	}
	got, err = nitro.Remember(c, "k", time.Minute, fn)
	if err != nil || len(got) != 1 || calls != 1 {
		t.Fatalf("hit: got=%v calls=%d err=%v", got, calls, err)
	}
}

func TestBust_NilSafe(t *testing.T) {
	nitro.Bust(nil, "x:*")
	c := &memCache{data: map[string]any{"a": "1"}}
	nitro.Bust(c, "drivers:*")
	if len(c.data) != 0 {
		t.Fatalf("%#v", c.data)
	}
}

func TestMustValidate(t *testing.T) {
	type row struct {
		Name string `json:"name" validate:"required,min=2"`
	}
	v := nitro.NewValidator()
	if err := nitro.MustValidate(v, &row{Name: "ab"}); err != nil {
		t.Fatal(err)
	}
	if err := nitro.MustValidate(v, &row{Name: "x"}); err == nil {
		t.Fatal("expected validation error")
	}
}
