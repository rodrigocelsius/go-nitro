package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanProjectRoutes_ResourceAndAuth(t *testing.T) {
	dir := t.TempDir()
	src := `package main
import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
func main() {
	app := nitro.New()
	app.Resource("/products", nil)
	app.Resource(&Order{}, nil)
	app.AuthRoutes("/auth", nil)
	app.Get("/health", nil)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	routes := scanProjectRoutes(dir)
	want := map[string]bool{
		"GET /products":       true,
		"POST /products":      true,
		"GET /products/:id":   true,
		"GET /orders":         true,
		"POST /auth/register": true,
		"POST /auth/login":    true,
		"GET /health":         true,
	}
	got := map[string]bool{}
	for _, r := range routes {
		got[r.method+" "+r.path] = true
	}
	for k := range want {
		if !got[k] {
			t.Fatalf("missing %s in %#v", k, got)
		}
	}
}

func TestCmdOpenAPIWritesFile(t *testing.T) {
	dir := t.TempDir()
	src := `package main
import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
func main() {
	app := nitro.New()
	app.Get("/ping", nil)
	app.Resource("/items", nil)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "openapi.json")
	old := os.Args
	defer func() { os.Args = old }()
	// call cmdOpenAPI directly
	cmdOpenAPI([]string{"--dir", dir, "-o", out, "--title", "Demo"})
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	if spec["openapi"] != "3.0.3" {
		t.Fatalf("openapi = %v", spec["openapi"])
	}
	paths, _ := spec["paths"].(map[string]any)
	if paths["/ping"] == nil {
		t.Fatalf("paths missing /ping: %v", paths)
	}
	if !strings.Contains(string(b), `"/items"`) {
		t.Fatalf("expected /items in %s", b)
	}
}
