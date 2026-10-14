package controller

import (
	"strings"
	"testing"
)

func TestGenerateController_ResourceMount(t *testing.T) {
	cfg := ControllerConfig{
		StructName:  "User",
		TableName:   "users",
		PackageName: "handlers",
		Fields: []FieldConfig{
			{GoName: "ID", JSONName: "id", GoType: "string", IsID: true},
			{GoName: "Name", JSONName: "name", GoType: "string"},
			{GoName: "Email", JSONName: "email", GoType: "string"},
		},
	}

	code := GenerateController(cfg)

	if !strings.Contains(code, "package handlers") {
		t.Error("package")
	}
	if !strings.Contains(code, `nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"`) {
		t.Error("import")
	}
	if !strings.Contains(code, "func MountUser(app *nitro.App, repo nitro.ResourceRepository)") {
		t.Error("MountUser")
	}
	if !strings.Contains(code, "app.Resource(") {
		t.Error("Resource")
	}
	if !strings.Contains(code, `&User{}`) {
		t.Error("model pointer")
	}
	if !strings.Contains(code, `Filterable`) {
		t.Error("Filterable opts")
	}
	if strings.Contains(code, "TODO") {
		t.Error("should not contain TODO stubs")
	}
}

func TestGenerateControllerFilename(t *testing.T) {
	if got := GenerateControllerFilename("User"); got != "user_resource_gen.go" {
		t.Fatalf("got %q", got)
	}
}

func TestGetHandlerInfo(t *testing.T) {
	cfg := ControllerConfig{StructName: "User", TableName: "users"}
	info := GetHandlerInfo(cfg)
	if len(info) != 5 {
		t.Fatalf("want 5 handlers, got %d", len(info))
	}
}
