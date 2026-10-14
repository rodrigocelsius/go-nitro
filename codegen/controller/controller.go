package controller

import (
	"fmt"
	"strings"

	nitroast "github.com/rodrigocelsius/go-nitro/codegen/ast/v2"
	datagen "github.com/rodrigocelsius/go-nitro/codegen/data/v2"
)

// GenerateControllerFromModel generates a controller from a Go model file
func GenerateControllerFromModel(filename string, packageName string) (string, error) {
	models, err := datagen.ParseModels(filename)
	if err != nil {
		return "", fmt.Errorf("failed to parse models: %w", err)
	}

	if len(models) == 0 {
		return "", fmt.Errorf("no models found in %s", filename)
	}

	// Generate controller for the first model
	model := models[0]
	cfg := modelToControllerConfig(model, packageName)

	return GenerateController(cfg), nil
}

// modelToControllerConfig converts a datagen.Model to ControllerConfig
func modelToControllerConfig(model datagen.Model, packageName string) ControllerConfig {
	cfg := ControllerConfig{
		StructName:    model.StructName,
		TableName:     model.TableName,
		PackageName:   packageName,
		HasSoftDelete: hasSoftDeleteField(model),
		HasTimestamps: hasTimestampFields(model),
	}

	for _, col := range model.Columns {
		field := FieldConfig{
			GoName:     col.GoName,
			JSONName:   col.ColumnName,
			ColumnName: col.ColumnName,
			GoType:     goTypeToString(col.Kind, col.TypeName),
			IsID:       col.ColumnName == "id",
		}

		// Check for tags
		if col.Tags != nil {
			if primary, ok := col.Tags["primary"]; ok && primary == "true" {
				field.IsID = true
			}
			if _, ok := col.Tags["default"]; ok {
				field.HasDefault = true
			}
			if validate, ok := col.Tags["validate"]; ok && strings.Contains(validate, "required") {
				field.IsRequired = true
			}
		}

		// Set filterable and sortable flags
		field.IsFilterable = GetFilterType(field.GoType, field.GoName) != FilterIgnore
		field.IsSortable = IsSortable(field)

		cfg.Fields = append(cfg.Fields, field)
	}

	return cfg
}

// hasSoftDeleteField checks if the model has a deleted_at field
func hasSoftDeleteField(model datagen.Model) bool {
	for _, col := range model.Columns {
		if col.ColumnName == "deleted_at" {
			return true
		}
	}
	return false
}

// hasTimestampFields checks if the model has created_at/updated_at fields
func hasTimestampFields(model datagen.Model) bool {
	hasCreatedAt := false
	hasUpdatedAt := false

	for _, col := range model.Columns {
		if col.ColumnName == "created_at" {
			hasCreatedAt = true
		}
		if col.ColumnName == "updated_at" {
			hasUpdatedAt = true
		}
	}

	return hasCreatedAt && hasUpdatedAt
}

// goTypeToString converts a FieldKind to a Go type string
func goTypeToString(kind nitroast.FieldKind, typeName string) string {
	switch kind {
	case nitroast.KindString:
		return "string"
	case nitroast.KindInt:
		return typeName
	case nitroast.KindFloat:
		return typeName
	case nitroast.KindBool:
		return "bool"
	case nitroast.KindTime:
		return "time.Time"
	default:
		return "interface{}"
	}
}

// GetHandlerInfo returns metadata about the generated handlers
func GetHandlerInfo(cfg ControllerConfig) []HandlerInfo {
	return []HandlerInfo{
		{Type: HandlerList, HTTPMethod: "GET", Path: "/", MethodName: "List"},
		{Type: HandlerGet, HTTPMethod: "GET", Path: "/:id", MethodName: "Get"},
		{Type: HandlerCreate, HTTPMethod: "POST", Path: "/", MethodName: "Create"},
		{Type: HandlerUpdate, HTTPMethod: "PUT", Path: "/:id", MethodName: "Update"},
		{Type: HandlerDelete, HTTPMethod: "DELETE", Path: "/:id", MethodName: "Delete"},
	}
}
