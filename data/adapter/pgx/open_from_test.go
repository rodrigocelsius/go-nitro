package pgx

import "testing"

func TestSearchPathForSchema(t *testing.T) {
	if got := searchPathForSchema(""); got != "public" {
		t.Fatalf("empty=%q", got)
	}
	if got := searchPathForSchema("public"); got != "public" {
		t.Fatalf("public=%q", got)
	}
	if got := searchPathForSchema("transporte"); got != "transporte,public" {
		t.Fatalf("schema=%q", got)
	}
}
