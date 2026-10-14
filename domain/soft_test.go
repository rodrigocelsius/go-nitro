package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestSoftDeleteColumn_Embed(t *testing.T) {
	type base struct {
		ID        string     `db:"id"`
		DeletedAt *time.Time `db:"deleted_at"`
	}
	type row struct {
		base
		Name string `db:"name"`
	}
	if got := domain.SoftDeleteColumn(reflect.TypeOf(row{})); got != "deleted_at" {
		t.Fatalf("got %q", got)
	}
}

func TestSoftDeleteColumn_Hard(t *testing.T) {
	type row struct {
		ID   string `db:"id"`
		Name string `db:"name"`
	}
	if got := domain.SoftDeleteColumn(reflect.TypeOf(row{})); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestModelIDIsInteger(t *testing.T) {
	type serial struct {
		ID int64 `db:"id"`
	}
	type uuid struct {
		ID string `db:"id"`
	}
	if !domain.ModelIDIsInteger(reflect.TypeOf(serial{})) {
		t.Fatal("expected int ID")
	}
	if domain.ModelIDIsInteger(reflect.TypeOf(uuid{})) {
		t.Fatal("expected string ID not integer")
	}
}
