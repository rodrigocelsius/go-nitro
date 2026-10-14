package money_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2/money"
)

func TestFormatParse_ptBR(t *testing.T) {
	m := money.FromMajor(1234.56, "BRL")
	s := money.Format(m, "pt-BR")
	if s != "1.234,56 BRL" {
		t.Fatalf("got %q", s)
	}
	parsed, err := money.Parse("1.234,56 BRL", "", "pt-BR")
	if err != nil || !parsed.Equal(m) {
		t.Fatalf("parse: %+v %v", parsed, err)
	}
}
