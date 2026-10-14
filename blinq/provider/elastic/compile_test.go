package elastic_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/elastic"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestCompileSearch(t *testing.T) {
	age := field.Col[int]("users", "age")
	q := query.FromName("users").
		Where(age.Gte(18).And(age.Lt(65))).
		OrderBy(age.OrderDesc()).
		Offset(5).
		Limit(10)

	s, err := elastic.CompileSearch(q.AST())
	if err != nil {
		t.Fatal(err)
	}
	if s.Index != "users" {
		t.Fatal(s.Index)
	}
	queryObj, ok := s.Body["query"].(map[string]any)
	if !ok {
		t.Fatalf("%#v", s.Body)
	}
	boolQ, ok := queryObj["bool"].(map[string]any)
	if !ok || boolQ["filter"] == nil {
		t.Fatalf("%#v", queryObj)
	}
	if s.Body["from"] != 5 || s.Body["size"] != 10 {
		t.Fatalf("from/size %#v %#v", s.Body["from"], s.Body["size"])
	}
	sort, ok := s.Body["sort"].([]map[string]any)
	if !ok || len(sort) != 1 {
		t.Fatalf("%#v", s.Body["sort"])
	}
	ageSort, ok := sort[0]["age"].(map[string]any)
	if !ok || ageSort["order"] != "desc" {
		t.Fatalf("%#v", sort[0])
	}
}

func TestCompileSearchMatchAll(t *testing.T) {
	s, err := elastic.CompileSearch(query.FromName("users").AST())
	if err != nil {
		t.Fatal(err)
	}
	q, ok := s.Body["query"].(map[string]any)
	if !ok || q["match_all"] == nil {
		t.Fatalf("%#v", s.Body)
	}
}
