package pgx_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type item struct {
	ID     int64   `json:"id" db:"id"`
	Name   string  `json:"name" db:"name"`
	Active bool    `json:"active" db:"active"`
	Price  float64 `json:"price" db:"price"`
}

func TestAdaptStore_ImplementsQueryableLister(t *testing.T) {
	repo := pgx.AdaptStore[item](nil)
	if _, ok := repo.(domain.ResourceRepository); !ok {
		t.Fatal("ResourceRepository")
	}
	if _, ok := repo.(domain.QueryableLister); !ok {
		t.Fatal("QueryableLister")
	}
}

func TestStore_AsResourceRepository(t *testing.T) {
	var store *pgx.Store[item]
	repo := store.AsResourceRepository()
	if repo == nil {
		t.Fatal("nil repo")
	}
	if _, ok := repo.(domain.QueryableLister); !ok {
		t.Fatal("QueryableLister")
	}
}
