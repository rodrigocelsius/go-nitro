package routergen

import (
	"go/format"
	"os"
	"testing"
)

func TestGenerate_ProducesValidGoCode(t *testing.T) {
	routes := []Route{
		{Method: "GET", Path: "/health", HandlerName: "healthHandler"},
		{Method: "GET", Path: "/users", HandlerName: "usersListHandler"},
		{Method: "GET", Path: "/users/:id", HandlerName: "getUserHandler"},
		{Method: "GET", Path: "/users/:id/posts", HandlerName: "getUserPostsHandler"},
		{Method: "POST", Path: "/users", HandlerName: "createUserHandler"},
		{Method: "GET", Path: "/orders/:id/items/:itemId", HandlerName: "getOrderItemHandler"},
	}

	code, err := Generate("testpkg", "github.com/rodrigocelsius/go-nitro/v2", routes)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = format.Source([]byte(code))
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n\n--- generated code ---\n%s", err, code)
	}

	formatted, _ := format.Source([]byte(code))
	os.WriteFile("testdata_output.go.txt", formatted, 0644)
}
