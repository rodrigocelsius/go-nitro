package openapi

// Spec represents the OpenAPI 3.0 structure.
type Spec struct {
	OpenAPI    string               `json:"openapi"`
	Info       Info                 `json:"info"`
	Servers    []Server             `json:"servers,omitempty"`
	Paths      map[string]*PathItem `json:"paths"`
	Components *Components          `json:"components,omitempty"`
}

// Info is API metadata.
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// Server is an API server.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

// PathItem represents an endpoint.
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
}

// Operation represents an HTTP operation.
type Operation struct {
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
}

// Parameter is an operation parameter.
type Parameter struct {
	Name     string  `json:"name"`
	In       string  `json:"in"`
	Required bool    `json:"required"`
	Schema   *Schema `json:"schema,omitempty"`
}

// RequestBody is the request body (OpenAPI 3 content → media type → schema).
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required"`
	Content     map[string]MediaType `json:"content"`
}

// MediaType is content keyed by Content-Type.
type MediaType struct {
	Schema  *Schema `json:"schema,omitempty"`
	Example any     `json:"example,omitempty"`
}

// Response is an operation response.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// Schema is a field schema.
type Schema struct {
	Type        string             `json:"type,omitempty"`
	Format      string             `json:"format,omitempty"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Ref         string             `json:"$ref,omitempty"`
	Example     any                `json:"example,omitempty"`
	Nullable    bool               `json:"nullable,omitempty"`
	ReadOnly    bool               `json:"readOnly,omitempty"`
}

// RefSchema returns a $ref to components.schemas[name].
func RefSchema(name string) *Schema {
	return &Schema{Ref: "#/components/schemas/" + name}
}

// Components are reusable OpenAPI components.
type Components struct {
	Schemas         map[string]*Schema         `json:"schemas,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme (OpenAPI 3) — e.g. HTTP Bearer JWT.
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
	Name         string `json:"name,omitempty"` // apiKey
	In           string `json:"in,omitempty"`   // apiKey
}

// SecurityRequirement is a security item (scheme name → scopes).
type SecurityRequirement map[string][]string

// RouteInfo is route information for spec generation.
type RouteInfo struct {
	Method      string
	Path        string
	Summary     string
	Tags        []string
	Params      []Parameter
	RequestBody *RequestBody
	Response    interface{}
	// ResponseSchema, if set, documents the success response body.
	ResponseSchema *Schema
	// SuccessStatus, if set (e.g. 201), is the only 2xx code documented.
	SuccessStatus int
	// Security, if non-nil, applies to the operation (e.g. BearerAuth()).
	// An empty slice [] marks the route as public (no auth).
	Security []SecurityRequirement
}

// JSONBody builds an application/json requestBody.
func JSONBody(schema *Schema, required bool, example any) *RequestBody {
	mt := MediaType{Schema: schema, Example: example}
	return &RequestBody{
		Required: required,
		Content:  map[string]MediaType{"application/json": mt},
	}
}

// ObjectSchema creates an object schema with properties and required.
func ObjectSchema(props map[string]*Schema, required ...string) *Schema {
	return &Schema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}
