package datagen

import (
	nitroast "github.com/rodrigocelsius/go-nitro/codegen/ast/v2"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoInjection_BasicBehavior(t *testing.T) {
	models, err := ParseModels("testdata/models.go")
	if err != nil {
		t.Fatalf("ParseModels failed: %v", err)
	}

	byName := make(map[string]Model)
	for _, m := range models {
		byName[m.StructName] = m
	}

	// User should have standard fields injected (CreatedAt, UpdatedAt, DeletedAt)
	// but ID already exists in the struct
	user := byName["User"]
	cols := make(map[string]Column)
	for _, c := range user.Columns {
		cols[c.GoName] = c
	}

	// ID should come from user struct (int, not string)
	if cols["ID"].Kind != nitroast.KindInt {
		t.Errorf("expected ID to be int (from user struct), got %v", cols["ID"].Kind)
	}

	// CreatedAt should be injected
	if _, ok := cols["CreatedAt"]; !ok {
		t.Error("expected CreatedAt to be injected")
	}
	if cols["CreatedAt"].ColumnName != "created_at" {
		t.Errorf("expected CreatedAt column name 'created_at', got %q", cols["CreatedAt"].ColumnName)
	}

	// UpdatedAt should be injected
	if _, ok := cols["UpdatedAt"]; !ok {
		t.Error("expected UpdatedAt to be injected")
	}

	// DeletedAt should be injected
	if _, ok := cols["DeletedAt"]; !ok {
		t.Error("expected DeletedAt to be injected")
	}
}

func TestAutoInjection_EmptyStruct(t *testing.T) {
	models, err := ParseModels("testdata/models.go")
	if err != nil {
		t.Fatalf("ParseModels failed: %v", err)
	}

	byName := make(map[string]Model)
	for _, m := range models {
		byName[m.StructName] = m
	}

	// NoColumns should have all standard fields injected
	noColumns := byName["NoColumns"]
	if len(noColumns.Columns) != 4 {
		t.Errorf("expected 4 columns for NoColumns, got %d", len(noColumns.Columns))
	}

	cols := make(map[string]Column)
	for _, c := range noColumns.Columns {
		cols[c.GoName] = c
	}

	expectedCols := []string{"ID", "CreatedAt", "UpdatedAt", "DeletedAt"}
	for _, name := range expectedCols {
		if _, ok := cols[name]; !ok {
			t.Errorf("expected %s to be injected for NoColumns", name)
		}
	}
}

func TestAutoInjection_ColumnOrder(t *testing.T) {
	models, err := ParseModels("testdata/models.go")
	if err != nil {
		t.Fatalf("ParseModels failed: %v", err)
	}

	byName := make(map[string]Model)
	for _, m := range models {
		byName[m.StructName] = m
	}

	// User columns should be in order: ID, Name, Email, Age, CreatedAt, UpdatedAt, DeletedAt
	user := byName["User"]
	expectedOrder := []string{"ID", "Name", "Email", "Age", "CreatedAt", "UpdatedAt", "DeletedAt"}
	if len(user.Columns) != len(expectedOrder) {
		t.Fatalf("expected %d columns, got %d", len(expectedOrder), len(user.Columns))
	}
	for i, name := range expectedOrder {
		if user.Columns[i].GoName != name {
			t.Errorf("column %d: expected %s, got %s", i, name, user.Columns[i].GoName)
		}
	}
}

func TestAutoInjection_WithAnnotations(t *testing.T) {
	// Create a temporary test file with annotations
	content := `package testdata

//nitro:notimestamps
type Product struct {
	Name  string
	Price float64
}

//nitro:nosoftdelete
type Category struct {
	Name string
}

//nitro:noid
type Tag struct {
	Name string
}

//nitro:notimestamps
//nitro:nosoftdelete
type Simple struct {
	Value string
}
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "annotations.go")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	models, err := ParseModels(tmpFile)
	if err != nil {
		t.Fatalf("ParseModels failed: %v", err)
	}

	byName := make(map[string]Model)
	for _, m := range models {
		byName[m.StructName] = m
	}

	// Product should have ID and DeletedAt but no timestamps (CreatedAt/UpdatedAt)
	product := byName["Product"]
	cols := make(map[string]Column)
	for _, c := range product.Columns {
		cols[c.GoName] = c
	}
	if _, ok := cols["ID"]; !ok {
		t.Error("expected ID to be present for Product")
	}
	if _, ok := cols["CreatedAt"]; ok {
		t.Error("expected CreatedAt to NOT be present for Product (nitro:notimestamps)")
	}
	if _, ok := cols["UpdatedAt"]; ok {
		t.Error("expected UpdatedAt to NOT be present for Product (nitro:notimestamps)")
	}
	if _, ok := cols["DeletedAt"]; !ok {
		t.Error("expected DeletedAt to be present for Product (nitro:nosoftdelete not set)")
	}

	// Category should have ID and timestamps but no DeletedAt
	category := byName["Category"]
	cols = make(map[string]Column)
	for _, c := range category.Columns {
		cols[c.GoName] = c
	}
	if _, ok := cols["ID"]; !ok {
		t.Error("expected ID to be present for Category")
	}
	if _, ok := cols["CreatedAt"]; !ok {
		t.Error("expected CreatedAt to be present for Category")
	}
	if _, ok := cols["UpdatedAt"]; !ok {
		t.Error("expected UpdatedAt to be present for Category")
	}
	if _, ok := cols["DeletedAt"]; ok {
		t.Error("expected DeletedAt to NOT be present for Category (nitro:nosoftdelete)")
	}

	// Tag should have timestamps and DeletedAt but no ID
	tag := byName["Tag"]
	cols = make(map[string]Column)
	for _, c := range tag.Columns {
		cols[c.GoName] = c
	}
	if _, ok := cols["ID"]; ok {
		t.Error("expected ID to NOT be present for Tag (nitro:noid)")
	}
	if _, ok := cols["CreatedAt"]; !ok {
		t.Error("expected CreatedAt to be present for Tag")
	}
	if _, ok := cols["UpdatedAt"]; !ok {
		t.Error("expected UpdatedAt to be present for Tag")
	}
	if _, ok := cols["DeletedAt"]; !ok {
		t.Error("expected DeletedAt to be present for Tag")
	}

	// Simple should have only ID (no timestamps, no soft delete)
	simple := byName["Simple"]
	cols = make(map[string]Column)
	for _, c := range simple.Columns {
		cols[c.GoName] = c
	}
	if _, ok := cols["ID"]; !ok {
		t.Error("expected ID to be present for Simple")
	}
	if _, ok := cols["CreatedAt"]; ok {
		t.Error("expected CreatedAt to NOT be present for Simple (nitro:notimestamps)")
	}
	if _, ok := cols["UpdatedAt"]; ok {
		t.Error("expected UpdatedAt to NOT be present for Simple (nitro:notimestamps)")
	}
	if _, ok := cols["DeletedAt"]; ok {
		t.Error("expected DeletedAt to NOT be present for Simple (nitro:nosoftdelete)")
	}
}

func TestAutoInjection_UserOverrideIDType(t *testing.T) {
	// Test that user-defined ID type is preserved
	content := `package testdata

type CustomID struct {
	ID   int64  ` + "`" + `db:"custom_id"` + "`" + `
	Name string
}
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "custom_id.go")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	models, err := ParseModels(tmpFile)
	if err != nil {
		t.Fatalf("ParseModels failed: %v", err)
	}

	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}

	model := models[0]
	if model.StructName != "CustomID" {
		t.Errorf("expected struct name 'CustomID', got %q", model.StructName)
	}

	cols := make(map[string]Column)
	for _, c := range model.Columns {
		cols[c.GoName] = c
	}

	// ID should be int64 (user-defined), not string (default)
	if cols["ID"].TypeName != "int64" {
		t.Errorf("expected ID type 'int64', got %q", cols["ID"].TypeName)
	}
	if cols["ID"].ColumnName != "custom_id" {
		t.Errorf("expected ID column name 'custom_id', got %q", cols["ID"].ColumnName)
	}
}

func TestAutoInjection_GenerateCode(t *testing.T) {
	models, err := ParseModels("testdata/models.go")
	if err != nil {
		t.Fatalf("ParseModels failed: %v", err)
	}

	code, err := Generate("testdata", "github.com/rodrigocelsius/go-nitro/blinq/v2/field", models)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify generated code contains standard fields
	expectedFields := []string{
		"CreatedAt blinqfield.Field[any]",
		"UpdatedAt blinqfield.Field[any]",
		"DeletedAt blinqfield.Field[any]",
	}
	for _, field := range expectedFields {
		if !containsString(code, field) {
			t.Errorf("generated code missing field: %s", field)
		}
	}

	// Verify generated code contains Field initialization
	expectedInit := []string{
		`CreatedAt: blinqfield.Field[any]{Table:`,
		`UpdatedAt: blinqfield.Field[any]{Table:`,
		`DeletedAt: blinqfield.Field[any]{Table:`,
	}
	for _, init := range expectedInit {
		if !containsString(code, init) {
			t.Errorf("generated code missing initialization: %s", init)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
