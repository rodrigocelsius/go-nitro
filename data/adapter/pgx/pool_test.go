package pgx_test

import (
	"context"
	"testing"

	"github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
)

func TestPool_NilSafe(t *testing.T) {
	var p *pgx.Pool
	p.Close() // must not panic
	if err := p.Ping(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if p.DB() != nil {
		t.Fatal("nil pool DB")
	}
	if p.Stat() != nil {
		t.Fatal("nil stat")
	}
}

func TestPool_NewFromPoolNil(t *testing.T) {
	p := pgx.NewFromPool(nil)
	p.Close()
	if err := p.Ping(context.Background()); err == nil {
		t.Fatal("expected ping error")
	}
}
