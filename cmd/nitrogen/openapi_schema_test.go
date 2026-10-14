package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAPIResourceSchemas(t *testing.T) {
	dir := t.TempDir()
	model := `package model

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Address struct {
	Street string
}

type Customer struct {
	nitro.Model
	FullName     string ` + "`validate:\"required\"`" + `
	Address2Line string
	Email        string  ` + "`json:\"mail,omitempty\" validate:\"required,email\"`" + `
	Secret       string  ` + "`json:\"-\"`" + `
	Score        *int64
	Tags         []string
	Home         Address
	IsAdmin      bool ` + "`readonly:\"true\"`" + `
	BirthDate    nitro.Date
}
`
	routes := `package handler

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

func Register(app *nitro.App) {
	repo := nitro.NewMemoryRepo[model.Customer]()
	nitro.Resource(app, "/customers", repo, &nitro.ResourceConfig{Public: true})
}
`
	for path, src := range map[string]string{"internal/model/customer.go": model, "internal/handler/routes.go": routes} {
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, "openapi.json")
	cmdOpenAPI([]string{"--dir", dir, "-o", out})
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths      map[string]map[string]map[string]any `json:"paths"`
		Components struct {
			Schemas map[string]struct {
				Properties map[string]map[string]any `json:"properties"`
				Required   []string                  `json:"required"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	full := spec.Components.Schemas["Customer"]
	for _, k := range []string{"id", "created_at", "full_name", "address2_line", "mail", "score", "tags", "home", "is_admin", "birth_date"} {
		if full.Properties[k] == nil {
			t.Errorf("Customer missing property %q: %v", k, full.Properties)
		}
	}
	if full.Properties["secret"] != nil {
		t.Error(`json:"-" field must not be documented`)
	}
	if full.Properties["id"]["readOnly"] != true || full.Properties["is_admin"]["readOnly"] != true {
		t.Errorf("server-owned fields must be readOnly: id=%v is_admin=%v", full.Properties["id"], full.Properties["is_admin"])
	}
	if full.Properties["home"]["$ref"] != "#/components/schemas/Address" || spec.Components.Schemas["Address"].Properties["street"] == nil {
		t.Errorf("nested struct must be a $ref component: %v", full.Properties["home"])
	}
	if full.Properties["score"]["nullable"] != true || full.Properties["birth_date"]["format"] != "date" {
		t.Errorf("score=%v birth_date=%v", full.Properties["score"], full.Properties["birth_date"])
	}
	if got := full.Required; len(got) != 2 || got[0] != "full_name" || got[1] != "mail" {
		t.Errorf("required=%v", got)
	}

	input := spec.Components.Schemas["CustomerInput"]
	for _, k := range []string{"id", "created_at", "updated_at", "deleted_at", "is_admin"} {
		if input.Properties[k] != nil {
			t.Errorf("CustomerInput must not expose server-owned %q", k)
		}
	}
	if len(spec.Components.Schemas["CustomerPatch"].Required) != 0 {
		t.Error("CustomerPatch must not require fields")
	}

	post := spec.Paths["/customers"]["post"]
	resp := post["responses"].(map[string]any)
	if _, ok := resp["201"]; !ok || resp["200"] != nil {
		t.Errorf("POST responses=%v", resp)
	}
	if post["requestBody"] == nil {
		t.Error("POST has no requestBody")
	}
	if del := spec.Paths["/customers/{id}"]["delete"]["responses"].(map[string]any); del["204"] == nil || del["200"] != nil {
		t.Errorf("DELETE responses=%v", del)
	}
	list := spec.Paths["/customers"]["get"]["responses"].(map[string]any)["200"].(map[string]any)
	if ref := list["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"]; ref != "#/components/schemas/CustomerList" {
		t.Errorf("list schema=%v", ref)
	}
}
