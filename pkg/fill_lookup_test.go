package nitro

import (
	"context"
	"errors"
	"testing"
)

type lookupRow struct{ Name string }

type lookupStore struct{ calls int }

func (s *lookupStore) Get(_ context.Context, id any) (*lookupRow, error) {
	s.calls++
	if id == "missing" {
		return nil, errors.New("not found")
	}
	return &lookupRow{Name: "found"}, nil
}

func TestFillStringFrom(t *testing.T) {
	ctx := context.Background()
	s := &lookupStore{}
	pick := func(r *lookupRow) string { return r.Name }
	var nilUUID *UUID
	for _, id := range []any{nil, "", nilUUID, UUID{}} {
		dest := "keep"
		if err := FillStringFrom(ctx, s, id, &dest, pick); err != nil || dest != "keep" {
			t.Fatalf("empty id %#v: dest=%q err=%v", id, dest, err)
		}
	}
	if s.calls != 0 {
		t.Fatal("empty ids must not hit the store")
	}
	dest := ""
	u := NewUUID()
	if err := FillStringFrom(ctx, s, &u, &dest, pick); err != nil || dest != "found" {
		t.Fatalf("dest=%q err=%v", dest, err)
	}
	if err := FillStringFrom(ctx, s, "missing", &dest, pick); err == nil {
		t.Fatal("store error must propagate")
	}
}
