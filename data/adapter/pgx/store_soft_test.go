package pgx

import (
	"testing"
	"time"
)

type softModel struct {
	ID        string     `json:"id" db:"id"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

type hardModel struct {
	ID        string    `json:"id" db:"id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

func TestSoftColumnOf_ModelEmbed(t *testing.T) {
	type row struct {
		softModel
		Name string `db:"name"`
	}
	if got := softColumnOf[row](); got != "deleted_at" {
		t.Fatalf("softColumnOf = %q, want deleted_at", got)
	}
}

func TestSoftColumnOf_HardModel(t *testing.T) {
	type row struct {
		hardModel
		Name string `db:"name"`
	}
	if got := softColumnOf[row](); got != "" {
		t.Fatalf("softColumnOf HardModel = %q, want empty", got)
	}
}

func TestSoftColumnOf_ExplicitDeletedAt(t *testing.T) {
	type row struct {
		ID        string     `db:"id"`
		DeletedAt *time.Time `db:"deleted_at"`
	}
	if got := softColumnOf[row](); got != "deleted_at" {
		t.Fatalf("softColumnOf = %q, want deleted_at", got)
	}
}

func TestStructDBValues_FlattenEmbed(t *testing.T) {
	type row struct {
		softModel
		Name string `db:"name"`
	}
	now := time.Now()
	r := row{softModel: softModel{ID: "abc", CreatedAt: now, UpdatedAt: now}, Name: "x"}
	cols, vals := structDBValues(&r, true)
	by := map[string]any{}
	for i, c := range cols {
		by[c] = vals[i]
	}
	if by["id"] != "abc" || by["name"] != "x" {
		t.Fatalf("unexpected cols=%v vals=%v", cols, vals)
	}
}
