package fasthttp

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/codegen/openapi/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// OpenAPIHandler returns the OpenAPI spec as JSON.
func OpenAPIHandler(generator *openapi.Generator) domain.Handler {
	return func(c *domain.Ctx) error {
		data, err := generator.Generate()
		if err != nil {
			return err
		}
		if fc, ok := c.AdapterData.(*FastCtx); ok {
			return fc.Write(200, data)
		}
		c.SetBody(data)
		c.SetHeader("Content-Type", "application/json")
		return nil
	}
}

// RegisterOpenAPIRoute registers the /openapi.json route.
func RegisterOpenAPIRoute(a *App, generator *openapi.Generator) {
	a.Get("/openapi.json", OpenAPIHandler(generator))
}

// RegisterOpenAPIFromRoutes auto-generates the OpenAPI spec from
// routes registered on App and registers /openapi.json.
// Call AFTER registering all routes (Resource, auth, etc.).
func RegisterOpenAPIFromRoutes(a *App, title, description, version string) {
	gen := openapi.NewGenerator(title, description, version)
	gen.AddServer("/", "current host")
	gen.AddBearerAuth("")
	gen.AddAPIKeyAuth("X-API-Key", "API key in the X-API-Key header (e.g. local value \"dev\"). No JWT needed.")
	for _, r := range a.Routes() {
		// Skip docs/spec themselves
		if r.Path == "/openapi.json" || r.Path == "/docs" || r.Path == "/docs/" {
			continue
		}
		params := extractPathParams(r.Path)
		openapiParams := make([]openapi.Parameter, 0, len(params))
		for _, p := range params {
			openapiParams = append(openapiParams, openapi.Parameter{
				Name:     p,
				In:       "path",
				Required: true,
				Schema:   &openapi.Schema{Type: "string"},
			})
		}
		info := openapi.RouteInfo{
			Method:      r.Method,
			Path:        r.Path,
			Summary:     r.Method + " " + openapi.ToOpenAPIPath(r.Path),
			Tags:        extractTags(r.Path),
			Params:      openapiParams,
			RequestBody: inferRequestBody(r.Method, r.Path),
		}
		if requiresAuth(r.Path) {
			// OR: API key OU Bearer JWT
			info.Security = openapi.APIKeyOrBearerAuth()
		}
		gen.AddRoute(info)
	}
	a.Get("/openapi.json", OpenAPIHandler(gen))
}

// requiresAuth marks routes that need X-API-Key or Bearer (not public/docs/auth/health).
func requiresAuth(path string) bool {
	clean := strings.TrimSuffix(path, "/")
	switch clean {
	case "", "/health", "/ready", "/live", "/openapi.json", "/docs":
		return false
	}
	if strings.HasPrefix(clean, "/auth/") || clean == "/login" || clean == "/register" || clean == "/refresh" {
		return false
	}
	if clean == "/api/me" || clean == "/me" {
		return true
	}
	if strings.HasPrefix(clean, "/api/") {
		return true
	}
	// CRUD resources at root (e.g. /cities, /holidays) — data-api style
	return true
}

// inferRequestBody adds known schemas (auth) and a generic JSON body on writes.
func inferRequestBody(method, path string) *openapi.RequestBody {
	switch method {
	case "POST", "PUT", "PATCH":
	default:
		return nil
	}

	clean := strings.TrimSuffix(path, "/")
	switch {
	case strings.HasSuffix(clean, "/auth/login") || clean == "/login":
		return openapi.JSONBody(
			openapi.ObjectSchema(map[string]*openapi.Schema{
				"username": {Type: "string", Example: "admin"},
				"password": {Type: "string", Format: "password", Example: "admin"},
			}, "username", "password"),
			true,
			map[string]string{"username": "admin", "password": "admin"},
		)
	case strings.HasSuffix(clean, "/auth/refresh") || clean == "/refresh":
		return openapi.JSONBody(
			openapi.ObjectSchema(map[string]*openapi.Schema{
				"refresh_token": {
					Type:        "string",
					Description: "Use the refresh_token returned by POST /auth/login (not the access_token).",
				},
			}, "refresh_token"),
			true,
			nil,
		)
	case strings.HasSuffix(clean, "/auth/register") || clean == "/register":
		return openapi.JSONBody(
			openapi.ObjectSchema(map[string]*openapi.Schema{
				"email":    {Type: "string", Format: "email"},
				"password": {Type: "string", Format: "password"},
				"name":     {Type: "string"},
			}, "email", "password"),
			true,
			nil,
		)
	default:
		// CRUD / writes: body JSON (Swagger mostra editor)
		return openapi.JSONBody(
			&openapi.Schema{Type: "object"},
			true,
			map[string]any{},
		)
	}
}

// RegisterSwagger generates /openapi.json from routes and serves Swagger UI at /docs.
//
//	app.Swagger("My API", "Description", "1.0.0")
//	// GET /openapi.json — spec
//	// GET /docs         — Swagger UI
func RegisterSwagger(a *App, title, description, version string) {
	RegisterOpenAPIFromRoutes(a, title, description, version)
	a.Get("/docs", SwaggerUIHandler("/openapi.json", title))
	a.Get("/docs/", SwaggerUIHandler("/openapi.json", title))
}

// SwaggerUIHandler serves Swagger UI (CDN) pointing at specJSONPath.
func SwaggerUIHandler(specJSONPath, title string) domain.Handler {
	if title == "" {
		title = "API Docs"
	}
	if specJSONPath == "" {
		specJSONPath = "/openapi.json"
	}
	html := swaggerUIHTML(title, specJSONPath)
	return func(c *domain.Ctx) error {
		// Defaults() applies CSP default-src 'self', which blocks the Swagger UI CDN.
		c.SetHeader("Content-Security-Policy", swaggerDocsCSP)
		c.SetHeader("Content-Type", "text/html; charset=utf-8")
		c.SetBody([]byte(html))
		return nil
	}
}

// swaggerDocsCSP allows Swagger UI assets (unpkg) plus inline script/style on the page.
const swaggerDocsCSP = "default-src 'self'; " +
	"script-src 'self' 'unsafe-inline' https://unpkg.com; " +
	"style-src 'self' 'unsafe-inline' https://unpkg.com; " +
	"img-src 'self' data: https:; " +
	"font-src 'self' https://unpkg.com data:; " +
	"connect-src 'self'"

func swaggerUIHTML(title, specURL string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>` + title + ` — Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" crossorigin>
  <style>html{box-sizing:border-box}*,*:before,*:after{box-sizing:inherit}body{margin:0;background:#fafafa}.topbar{display:none!important}</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js" crossorigin></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-standalone-preset.js" crossorigin></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: '` + specURL + `',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: 'StandaloneLayout',
        tryItOutEnabled: true,
        persistAuthorization: true
      });
    };
  </script>
</body>
</html>`
}

// extractPathParams extracts route parameter names (e.g. ":id" → "id").
func extractPathParams(path string) []string {
	var params []string
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if strings.HasPrefix(p, ":") {
			params = append(params, p[1:])
		}
		if strings.HasPrefix(p, "*") {
			params = append(params, p[1:])
		}
	}
	return params
}

// extractTags extracts a tag from the path root.
func extractTags(path string) []string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		return []string{parts[0]}
	}
	return []string{"default"}
}
