//go:build nitro_nosqlite

package sqladapter

import (
	"context"
	"strings"
	"testing"
)

func TestExcludedDriverExplainsBuildTag(t *testing.T) {
	_, err := Open(context.Background(), "sqlite", "file:x?mode=memory")
	if err == nil || !strings.Contains(err.Error(), "nitro_nosqlite") {
		t.Fatalf("want build-tag hint, got %v", err)
	}
}
