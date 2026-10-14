package validator_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

func TestIsEmail(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"a@b.co", true},
		{"  john@example.com  ", true},
		{"", false},
		{"@domain.com", false},
		{"a@b", false},
		{"a @b.co", false},
		{"a@b.", false},
	}
	for _, tt := range tests {
		if got := validator.IsEmail(tt.in); got != tt.want {
			t.Errorf("IsEmail(%q)=%v want %v", tt.in, got, tt.want)
		}
	}
}
