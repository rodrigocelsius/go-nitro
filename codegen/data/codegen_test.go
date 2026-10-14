package datagen

import (
	"go/format"
	"os"
	"testing"
)

func TestGenerate_ProducesValidGoCode(t *testing.T) {
	models, err := ParseModels("testdata/models.go")
	if err != nil {
		t.Fatalf("ParseModels failed: %v", err)
	}

	code, err := Generate("testdata", "github.com/rodrigocelsius/go-nitro/blinq/v2/field", models)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	formatted, err := format.Source([]byte(code))
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n\n--- generated code ---\n%s", err, code)
	}

	if err := os.WriteFile("testdata/models_gen.go", formatted, 0644); err != nil {
		t.Fatalf("failed to write generated file: %v", err)
	}
}
