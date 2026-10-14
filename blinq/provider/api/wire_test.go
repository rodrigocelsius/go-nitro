package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/api"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestEncodeWire(t *testing.T) {
	age := field.Col[int]("users", "age")
	q := query.FromName("users").Where(age.Gte(18)).Limit(5)
	b, err := api.Encode(q.AST())
	if err != nil {
		t.Fatal(err)
	}
	var w api.Wire
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	if w.Entity != "users" || w.Limit == nil || *w.Limit != 5 {
		t.Fatalf("%#v", w)
	}
}

type fakePoster struct {
	last []byte
}

func (f *fakePoster) PostQuery(_ context.Context, body []byte) ([]map[string]any, error) {
	f.last = append([]byte(nil), body...)
	return []map[string]any{{"id": 1}}, nil
}

func TestAPIProvider(t *testing.T) {
	ctx := context.Background()
	client := &fakePoster{}
	p := &api.Provider{Client: client}
	var got []map[string]any
	err := query.FromName("users").
		Where(field.Col[int]("users", "age").Gte(18)).
		Collect(ctx, p, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(client.last) == 0 {
		t.Fatalf("%#v body=%s", got, client.last)
	}
}
