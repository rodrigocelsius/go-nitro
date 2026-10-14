package controller

import "strings"

// FilterType represents the type of filter to apply
type FilterType int

const (
	FilterText FilterType = iota
	FilterExact
	FilterBool
	FilterNumber
	FilterIgnore
)

// GetFilterType determines the filter type based on the Go type
func GetFilterType(goType string, fieldName string) FilterType {
	if fieldName == "ID" || strings.HasSuffix(fieldName, "ID") {
		return FilterIgnore
	}
	if fieldName == "CreatedAt" || fieldName == "UpdatedAt" || fieldName == "DeletedAt" {
		return FilterIgnore
	}
	switch goType {
	case "string":
		return FilterText
	case "bool":
		return FilterBool
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return FilterNumber
	default:
		return FilterIgnore
	}
}

// CollectFilterableJSONNames returns JSON names for ResourceOpts.Filterable.
func CollectFilterableJSONNames(fields []FieldConfig) []string {
	var out []string
	for _, f := range fields {
		if f.IsID {
			continue
		}
		if GetFilterType(f.GoType, f.GoName) == FilterIgnore && !f.IsFilterable {
			continue
		}
		name := f.JSONName
		if name == "" {
			name = strings.ToLower(f.GoName)
		}
		out = append(out, name)
	}
	return out
}

// CollectSortableJSONNames returns JSON names for ResourceOpts.Sortable.
func CollectSortableJSONNames(fields []FieldConfig) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range fields {
		if f.IsID {
			continue
		}
		name := f.JSONName
		if name == "" {
			name = strings.ToLower(f.GoName)
		}
		if f.IsSortable || f.GoName == "CreatedAt" || f.GoName == "UpdatedAt" ||
			f.GoType == "string" || strings.HasPrefix(f.GoType, "int") || strings.HasPrefix(f.GoType, "float") {
			if !seen[name] {
				out = append(out, name)
				seen[name] = true
			}
		}
	}
	return out
}

// GenerateFilterCode is kept for tests; Resource path uses CollectFilterableJSONNames.
func GenerateFilterCode(field FieldConfig) string {
	if GetFilterType(field.GoType, field.GoName) == FilterIgnore {
		return ""
	}
	return "// filter ?" + field.JSONName + " via ResourceOpts.Filterable"
}

// GenerateAllFilters joins filter hints (legacy).
func GenerateAllFilters(fields []FieldConfig) string {
	names := CollectFilterableJSONNames(fields)
	if len(names) == 0 {
		return ""
	}
	return "// Filterable: " + strings.Join(names, ", ")
}
