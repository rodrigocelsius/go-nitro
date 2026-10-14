package domain

import (
	"testing"
)

func TestCtx_Param(t *testing.T) {
	c := &Ctx{}
	c.SetParam("id", "123")
	c.SetParam("name", "nitro")

	if c.ParamCount != 2 {
		t.Fatalf("expected 2 params, got %d", c.ParamCount)
	}
	if got := c.Param("id"); got != "123" {
		t.Fatalf("expected '123', got %q", got)
	}
	if got := c.Param("name"); got != "nitro" {
		t.Fatalf("expected 'nitro', got %q", got)
	}
	if got := c.Param("missing"); got != "" {
		t.Fatalf("expected empty string for missing param, got %q", got)
	}
}

func TestCtx_Reset(t *testing.T) {
	c := &Ctx{}
	c.SetParam("id", "123")
	c.Reset()

	if c.ParamCount != 0 {
		t.Fatalf("expected 0 after reset, got %d", c.ParamCount)
	}
	if c.Param("id") != "" {
		t.Fatalf("expected empty string after reset")
	}
}

func TestErrInvalidJSON(t *testing.T) {
	err := ErrInvalidJSON{}
	if err.Error() != "nitro: invalid JSON body" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}
