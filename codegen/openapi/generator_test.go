package openapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToOpenAPIPath(t *testing.T) {
	if got := ToOpenAPIPath("/api/drivers/:id"); got != "/api/drivers/{id}" {
		t.Fatalf("got %q", got)
	}
}

func TestAddRoute_LoginRequestBody(t *testing.T) {
	g := NewGenerator("t", "d", "1")
	g.AddRoute(RouteInfo{
		Method: "POST",
		Path:   "/auth/login",
		RequestBody: JSONBody(
			ObjectSchema(map[string]*Schema{
				"username": {Type: "string"},
				"password": {Type: "string"},
			}, "username", "password"),
			true,
			map[string]string{"username": "admin", "password": "admin"},
		),
	})
	raw, err := g.Generate()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	paths := m["paths"].(map[string]any)
	login := paths["/auth/login"].(map[string]any)
	post := login["post"].(map[string]any)
	rb := post["requestBody"].(map[string]any)
	content := rb["content"].(map[string]any)
	appJSON := content["application/json"].(map[string]any)
	schema := appJSON["schema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	if props["username"] == nil || props["password"] == nil {
		t.Fatalf("missing props: %s", raw)
	}
	if !strings.Contains(string(raw), `"required"`) {
		t.Fatalf("expected required fields: %s", raw)
	}
}

func TestAddBearerAuth(t *testing.T) {
	g := NewGenerator("t", "d", "1")
	g.AddBearerAuth("")
	g.AddRoute(RouteInfo{
		Method:   "GET",
		Path:     "/api/me",
		Security: BearerAuth(),
	})
	raw, err := g.Generate()
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"bearerAuth"`) || !strings.Contains(s, `"bearer"`) {
		t.Fatalf("missing bearer scheme: %s", s)
	}
	if !strings.Contains(s, `"security"`) {
		t.Fatalf("missing operation security: %s", s)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	comp := m["components"].(map[string]any)
	schemes := comp["securitySchemes"].(map[string]any)
	bearer := schemes["bearerAuth"].(map[string]any)
	if bearer["type"] != "http" || bearer["scheme"] != "bearer" {
		t.Fatalf("bad scheme: %#v", bearer)
	}
}

func TestAddAPIKeyAuth(t *testing.T) {
	g := NewGenerator("t", "d", "1")
	g.AddAPIKeyAuth("X-API-Key", "")
	g.AddRoute(RouteInfo{
		Method:   "GET",
		Path:     "/cities",
		Security: APIKeyAuth(),
	})
	raw, err := g.Generate()
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"apiKeyAuth"`) || !strings.Contains(s, `"X-API-Key"`) {
		t.Fatalf("missing apiKey scheme: %s", s)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	comp := m["components"].(map[string]any)
	schemes := comp["securitySchemes"].(map[string]any)
	apiKey := schemes["apiKeyAuth"].(map[string]any)
	if apiKey["type"] != "apiKey" || apiKey["in"] != "header" || apiKey["name"] != "X-API-Key" {
		t.Fatalf("bad scheme: %#v", apiKey)
	}
}
