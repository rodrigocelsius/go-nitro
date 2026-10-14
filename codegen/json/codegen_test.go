package gen

import (
	"go/format"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate_ProducesValidGoCode(t *testing.T) {
	defs, err := ParseFile("testdata/order.go")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	code, err := Generate("testdata", defs)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// go/format.Source fails if the generated code is not syntactically
	// valid — cheapest, most direct check we have before a real compile.
	formatted, err := format.Source([]byte(code))
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n\n--- generated code ---\n%s", err, code)
	}

	// Write the formatted file into testdata/ so the real execution
	// test (order_gen_test.go) can import and run it.
	outPath := filepath.Join("testdata", "order_gen.go")
	if err := os.WriteFile(outPath, formatted, 0644); err != nil {
		t.Fatalf("failed to write generated file: %v", err)
	}
}
