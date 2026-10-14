package controller

// ControllerConfig holds configuration for generating a controller
type ControllerConfig struct {
	// StructName is the name of the model struct (e.g., "User")
	StructName string
	// TableName is the database table name (e.g., "users")
	TableName string
	// PackageName is the Go package name
	PackageName string
	// Fields are the model fields
	Fields []FieldConfig
	// HasSoftDelete indicates if the model has soft delete
	HasSoftDelete bool
	// HasTimestamps indicates if the model has created_at/updated_at
	HasTimestamps bool
}

// FieldConfig describes a single field in the model
type FieldConfig struct {
	// GoName is the Go field name (e.g., "Name")
	GoName string
	// JSONName is the JSON field name (e.g., "name")
	JSONName string
	// ColumnName is the database column name (e.g., "name")
	ColumnName string
	// GoType is the Go type (e.g., "string", "int", "time.Time")
	GoType string
	// IsID indicates if this is the primary key field
	IsID bool
	// HasDefault indicates if the field has a default value
	HasDefault bool
	// IsRequired indicates if the field is required (validate:"required")
	IsRequired bool
	// IsFilterable indicates if this field can be filtered
	IsFilterable bool
	// IsSortable indicates if this field can be sorted
	IsSortable bool
}

// HandlerType represents the type of CRUD handler
type HandlerType int

const (
	HandlerList HandlerType = iota
	HandlerGet
	HandlerCreate
	HandlerUpdate
	HandlerDelete
)

// HandlerInfo contains metadata about a generated handler
type HandlerInfo struct {
	Type       HandlerType
	HTTPMethod string
	Path       string
	MethodName string
}
