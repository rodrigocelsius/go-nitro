package gen

import (
	"go/format"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateRoutes_ProducesValidGoCode(t *testing.T) {
	routes, err := ParseRoutes("routetestdata/handlers.go")
	if err != nil {
		t.Fatalf("ParseRoutes failed: %v", err)
	}

	code, err := GenerateRoutes("routetestdata", "github.com/rodrigocelsius/go-nitro/v2", routes)
	if err != nil {
		t.Fatalf("GenerateRoutes failed: %v", err)
	}

	formatted, err := format.Source([]byte(code))
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n\n--- generated code ---\n%s", err, code)
	}

	outPath := filepath.Join("routetestdata", "handlers_gen.go")
	if err := os.WriteFile(outPath, formatted, 0644); err != nil {
		t.Fatalf("failed to write generated file: %v", err)
	}
}
