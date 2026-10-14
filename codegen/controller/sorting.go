package controller

import "strings"

// GenerateSortingCode returns a comment for ResourceOpts.Sortable (no stubs).
func GenerateSortingCode(fields []FieldConfig, defaultSort string) string {
	names := CollectSortableJSONNames(fields)
	if len(names) == 0 {
		if defaultSort == "" {
			defaultSort = "created_at"
		}
		return "// Sortable default: " + defaultSort
	}
	return "// Sortable: " + strings.Join(names, ", ")
}

// IsSortable determines if a field can be sorted on
func IsSortable(field FieldConfig) bool {
	if field.GoName == "ID" || field.GoName == "CreatedAt" || field.GoName == "UpdatedAt" {
		return true
	}
	switch field.GoType {
	case "string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return true
	default:
		return false
	}
}
