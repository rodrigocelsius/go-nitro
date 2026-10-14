package migration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/codegen/migration/v2"
)

func TestGenerateMigration_JSONSlice(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "models.go")
	content := `package models

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

//nitro:table=items
type Item struct {
	nitro.Model
	Name     string             ` + "`json:\"name\" db:\"name\"`" + `
	Tags     nitro.StringSlice  ` + "`json:\"tags\" db:\"tags\"`" + `
	Payload  []string           ` + "`json:\"payload\" db:\"payload\"`" + `
	Meta     map[string]string  ` + "`json:\"meta\" db:\"meta\"`" + `
	Custom   string             ` + "`json:\"custom\" db:\"custom,jsonb\"`" + `
	Missions nitro.JSONSlice[string] ` + "`json:\"missions\" db:\"missions\" blinq:\"gin\"`" + `
}
`
	if err := os.WriteFile(src, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	mig, tables, err := migration.GenerateMigrationWithSchema(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 {
		t.Fatalf("tables=%d", len(tables))
	}
	up := mig.Up
	for _, col := range []string{"tags", "payload", "missions"} {
		if !strings.Contains(up, col) || !strings.Contains(up, "JSONB") {
			t.Fatalf("missing jsonb col %s in:\n%s", col, up)
		}
	}
	if !strings.Contains(up, "DEFAULT '[]'::jsonb") {
		t.Fatalf("missing array default:\n%s", up)
	}
	if !strings.Contains(up, "USING GIN") {
		t.Fatalf("missing gin:\n%s", up)
	}
}
