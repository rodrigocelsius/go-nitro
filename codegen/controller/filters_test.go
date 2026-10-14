package controller

import (
	"strings"
	"testing"
)

func TestGetFilterType(t *testing.T) {
	tests := []struct {
		name      string
		goType    string
		fieldName string
		expected  FilterType
	}{
		{"string field", "string", "Name", FilterText},
		{"bool field", "bool", "Active", FilterBool},
		{"int field", "int", "Age", FilterNumber},
		{"ID field", "string", "ID", FilterIgnore},
		{"foreign key", "string", "UserID", FilterIgnore},
		{"created_at", "time.Time", "CreatedAt", FilterIgnore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFilterType(tt.goType, tt.fieldName)
			if result != tt.expected {
				t.Errorf("GetFilterType(%q, %q) = %d, want %d", tt.goType, tt.fieldName, result, tt.expected)
			}
		})
	}
}

func TestCollectFilterableJSONNames(t *testing.T) {
	fields := []FieldConfig{
		{GoName: "ID", JSONName: "id", GoType: "string", IsID: true},
		{GoName: "Name", JSONName: "name", GoType: "string"},
		{GoName: "Active", JSONName: "active", GoType: "bool"},
		{GoName: "CreatedAt", JSONName: "created_at", GoType: "time.Time"},
	}
	names := CollectFilterableJSONNames(fields)
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "name") || !strings.Contains(joined, "active") {
		t.Fatalf("unexpected filterable: %v", names)
	}
	if strings.Contains(joined, "id") || strings.Contains(joined, "created_at") {
		t.Fatalf("should skip id/timestamps: %v", names)
	}
}

func TestGenerateFilterCode(t *testing.T) {
	code := GenerateFilterCode(FieldConfig{GoName: "Name", JSONName: "name", GoType: "string"})
	if !strings.Contains(code, "name") {
		t.Fatalf("got %q", code)
	}
}
