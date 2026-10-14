package pgx

import (
	"context"
	"testing"
)

func TestEnsureSchema_NoopAndInvalid(t *testing.T) {
	db := &DB{}
	if err := db.EnsureSchema(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureSchema(context.Background(), "public"); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureSchema(context.Background(), "bad-name!"); err == nil {
		t.Fatal("expected invalid schema error")
	}
}
