package datagen

import "testing"

func TestParseModels_ConventionAndOverride(t *testing.T) {
	models, err := ParseModels("testdata/models.go")
	if err != nil {
		t.Fatalf("ParseModels failed: %v", err)
	}

	byName := make(map[string]Model)
	for _, m := range models {
		byName[m.StructName] = m
	}

	// NoColumns now gets standard fields injected (ID, CreatedAt, UpdatedAt, DeletedAt)
	noColumns, ok := byName["NoColumns"]
	if !ok {
		t.Fatal("NoColumns model not found (should have standard fields injected)")
	}
	if len(noColumns.Columns) != 4 {
		t.Errorf("expected 4 columns for NoColumns (injected standard fields), got %d", len(noColumns.Columns))
	}

	user, ok := byName["User"]
	if !ok {
		t.Fatal("User model not found")
	}
	if user.TableName != "users" {
		t.Fatalf("expected table name 'users' (convention), got %q", user.TableName)
	}

	colsByGoName := make(map[string]Column)
	for _, c := range user.Columns {
		colsByGoName[c.GoName] = c
	}
	if colsByGoName["ID"].ColumnName != "id" {
		t.Errorf("expected column 'id' for ID (from db tag), got %q", colsByGoName["ID"].ColumnName)
	}
	if colsByGoName["Age"].ColumnName != "age" {
		t.Errorf("expected column 'age' for Age (from convention, no db tag), got %q", colsByGoName["Age"].ColumnName)
	}

	// User should have standard fields injected (CreatedAt, UpdatedAt, DeletedAt)
	if _, ok := colsByGoName["CreatedAt"]; !ok {
		t.Error("expected CreatedAt to be injected for User")
	}
	if _, ok := colsByGoName["UpdatedAt"]; !ok {
		t.Error("expected UpdatedAt to be injected for User")
	}
	if _, ok := colsByGoName["DeletedAt"]; !ok {
		t.Error("expected DeletedAt to be injected for User")
	}

	orderItem, ok := byName["OrderItem"]
	if !ok {
		t.Fatal("OrderItem model not found")
	}
	if orderItem.TableName != "order_items_custom" {
		t.Fatalf("expected overridden table name 'order_items_custom', got %q", orderItem.TableName)
	}
}
