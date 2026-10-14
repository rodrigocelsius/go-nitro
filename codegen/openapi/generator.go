package openapi

import "encoding/json"

// Default security scheme names.
const (
	SchemeBearerAuth = "bearerAuth"
	SchemeAPIKeyAuth = "apiKeyAuth"
)

// Generator builds OpenAPI specs from route information.
type Generator struct {
	spec *Spec
}

// NewGenerator creates a new Generator.
func NewGenerator(title, description, version string) *Generator {
	return &Generator{
		spec: &Spec{
			OpenAPI: "3.0.3",
			Info: Info{
				Title:       title,
				Description: description,
				Version:     version,
			},
			Paths: make(map[string]*PathItem),
		},
	}
}

// AddBearerAuth registers components.securitySchemes.bearerAuth (JWT).
// Enables the Authorize button in Swagger UI.
func (g *Generator) AddBearerAuth(description string) *Generator {
	if description == "" {
		description = "JWT access token (Authorization: Bearer <token>). Obtenha via POST /auth/login."
	}
	if g.spec.Components == nil {
		g.spec.Components = &Components{}
	}
	if g.spec.Components.SecuritySchemes == nil {
		g.spec.Components.SecuritySchemes = map[string]*SecurityScheme{}
	}
	g.spec.Components.SecuritySchemes[SchemeBearerAuth] = &SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  description,
	}
	return g
}

// BearerAuth returns the security requirement for bearerAuth.
func BearerAuth() []SecurityRequirement {
	return []SecurityRequirement{{SchemeBearerAuth: {}}}
}

// AddAPIKeyAuth registers components.securitySchemes.apiKeyAuth (header).
// In Swagger UI: Authorize → apiKeyAuth → value (e.g. "dev").
func (g *Generator) AddAPIKeyAuth(headerName, description string) *Generator {
	if headerName == "" {
		headerName = "X-API-Key"
	}
	if description == "" {
		description = "API key no header " + headerName + " (ex.: dev)."
	}
	if g.spec.Components == nil {
		g.spec.Components = &Components{}
	}
	if g.spec.Components.SecuritySchemes == nil {
		g.spec.Components.SecuritySchemes = map[string]*SecurityScheme{}
	}
	g.spec.Components.SecuritySchemes[SchemeAPIKeyAuth] = &SecurityScheme{
		Type:        "apiKey",
		In:          "header",
		Name:        headerName,
		Description: description,
	}
	return g
}

// APIKeyAuth returns the security requirement for apiKeyAuth.
func APIKeyAuth() []SecurityRequirement {
	return []SecurityRequirement{{SchemeAPIKeyAuth: {}}}
}

// APIKeyOrBearerAuth allows either API key or Bearer JWT (OR in OpenAPI).
func APIKeyOrBearerAuth() []SecurityRequirement {
	return []SecurityRequirement{
		{SchemeAPIKeyAuth: {}},
		{SchemeBearerAuth: {}},
	}
}

// AddRoute adds a route to the spec.
func (g *Generator) AddRoute(info RouteInfo) {
	pathKey := ToOpenAPIPath(info.Path)
	path, exists := g.spec.Paths[pathKey]
	if !exists {
		path = &PathItem{}
		g.spec.Paths[pathKey] = path
	}

	responses := map[string]Response{
		"200": {Description: "Success"},
	}
	switch info.Method {
	case "POST":
		responses = map[string]Response{
			"200": {Description: "Success"},
			"201": {Description: "Created"},
			"400": {Description: "Bad Request"},
			"401": {Description: "Unauthorized"},
			"422": {Description: "Validation Error"},
		}
	case "PUT", "PATCH":
		responses = map[string]Response{
			"200": {Description: "Success"},
			"400": {Description: "Bad Request"},
			"401": {Description: "Unauthorized"},
			"404": {Description: "Not Found"},
			"422": {Description: "Validation Error"},
		}
	case "DELETE":
		responses = map[string]Response{
			"200": {Description: "Success"},
			"204": {Description: "No Content"},
			"401": {Description: "Unauthorized"},
			"404": {Description: "Not Found"},
		}
	case "GET":
		if len(info.Security) > 0 {
			responses["401"] = Response{Description: "Unauthorized"}
		}
	}

	operation := &Operation{
		Summary:     info.Summary,
		Tags:        info.Tags,
		Parameters:  info.Params,
		RequestBody: info.RequestBody,
		Responses:   responses,
	}
	if info.Security != nil {
		operation.Security = make([]map[string][]string, len(info.Security))
		for i, req := range info.Security {
			operation.Security[i] = map[string][]string(req)
		}
	}

	switch info.Method {
	case "GET":
		path.Get = operation
	case "POST":
		path.Post = operation
	case "PUT":
		path.Put = operation
	case "PATCH":
		path.Patch = operation
	case "DELETE":
		path.Delete = operation
	}
}

// AddServer adds a server to the spec.
func (g *Generator) AddServer(url, description string) {
	g.spec.Servers = append(g.spec.Servers, Server{
		URL:         url,
		Description: description,
	})
}

// Generate returns the OpenAPI spec as JSON.
func (g *Generator) Generate() ([]byte, error) {
	return json.MarshalIndent(g.spec, "", "  ")
}

// Spec returns the Spec object.
func (g *Generator) Spec() *Spec {
	return g.spec
}

// ToOpenAPIPath converts :id / *path to {id} / {path}.
func ToOpenAPIPath(path string) string {
	out := make([]byte, 0, len(path)+8)
	i := 0
	for i < len(path) {
		if path[i] == ':' || path[i] == '*' {
			out = append(out, '{')
			i++
			start := i
			for i < len(path) && path[i] != '/' {
				i++
			}
			out = append(out, path[start:i]...)
			out = append(out, '}')
			continue
		}
		out = append(out, path[i])
		i++
	}
	return string(out)
}
