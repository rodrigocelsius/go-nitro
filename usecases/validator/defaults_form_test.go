package validator_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

func TestApplyDefaults_FormTagPresence(t *testing.T) {
	type in struct {
		Name   string `form:"name"`
		Active bool   `form:"active" default:"true"`
	}
	var v in
	// Key present under form name (not json) — Active omitted → default true.
	validator.ApplyDefaults(&v, map[string]any{"name": "Ada"})
	if !v.Active {
		t.Fatal("expected default active=true when form key absent")
	}
	var v2 in
	validator.ApplyDefaults(&v2, map[string]any{"name": "Ada", "active": false})
	if v2.Active {
		t.Fatal("explicit false must win via form key")
	}
}
