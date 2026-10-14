package nitro_test

import (
	"context"
	"errors"
	"testing"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func TestPing_OK(t *testing.T) {
	var saw context.Context
	h := nitro.Ping("db", func(ctx context.Context) error {
		saw = ctx
		return nil
	})
	if h.Name() != "db" {
		t.Fatalf("name=%q", h.Name())
	}
	if err := h.Check(); err != nil {
		t.Fatal(err)
	}
	if _, ok := saw.Deadline(); !ok {
		t.Fatal("expected timeout deadline")
	}
}

func TestPing_Error(t *testing.T) {
	h := nitro.Ping("redis", func(context.Context) error {
		return errors.New("down")
	})
	if err := h.Check(); err == nil || err.Error() != "down" {
		t.Fatalf("err=%v", err)
	}
}

func TestPingTimeout_Custom(t *testing.T) {
	h := nitro.PingTimeout("x", 50*time.Millisecond, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return nil
		}
	})
	if err := h.Check(); err == nil {
		t.Fatal("expected timeout")
	}
}
