package repository

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ID", "id"},
		{"Name", "name"},
		{"Email", "email"},
		{"CreatedAt", "created_at"},
		{"UserName", "user_name"},
		{"isActive", "is_active"},
		{"HTTPResponse", "http_response"},
	}
	for _, tt := range tests {
		got := scan.ToSnakeCase(tt.input)
		if got != tt.want {
			t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapToStruct(t *testing.T) {
	type User struct {
		ID    int
		Name  string
		Email string
	}

	row := map[string]any{
		"id":    1,
		"name":  "Rodrigo",
		"email": "rodrigo@test.com",
	}

	var user User
	if err := scan.MapToStruct(row, &user); err != nil {
		t.Fatalf("MapToStruct error: %v", err)
	}

	if user.ID != 1 {
		t.Errorf("ID = %d, want 1", user.ID)
	}
	if user.Name != "Rodrigo" {
		t.Errorf("Name = %q, want %q", user.Name, "Rodrigo")
	}
	if user.Email != "rodrigo@test.com" {
		t.Errorf("Email = %q, want %q", user.Email, "rodrigo@test.com")
	}
}

func TestMapToStruct_MissingField(t *testing.T) {
	type User struct {
		ID    int
		Name  string
		Email string
	}

	row := map[string]any{
		"id":   1,
		"name": "Rodrigo",
	}

	var user User
	if err := scan.MapToStruct(row, &user); err != nil {
		t.Fatalf("MapToStruct error: %v", err)
	}

	if user.ID != 1 {
		t.Errorf("ID = %d, want 1", user.ID)
	}
	if user.Name != "Rodrigo" {
		t.Errorf("Name = %q, want %q", user.Name, "Rodrigo")
	}
	if user.Email != "" {
		t.Errorf("Email should be empty, got %q", user.Email)
	}
}

func TestNewBaseRepository(t *testing.T) {
	type User struct {
		ID   int
		Name string
	}

	var _ = NewBaseRepository[User](nil, "users", "id")
}
