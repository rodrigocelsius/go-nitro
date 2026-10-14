package field_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
)

func TestFieldOps(t *testing.T) {
	age := field.Col[int]("users", "age")
	p := age.Gte(18).And(age.Lt(65))
	if _, ok := p.Node().(expr.And); !ok {
		t.Fatalf("%T", p.Node())
	}
	outer := age.EqOuter(field.Col[int]("orders", "user_id").Ref())
	b, ok := outer.Node().(expr.Binary)
	if !ok {
		t.Fatal(outer.Node())
	}
	if _, ok := b.Value.(expr.OuterRef); !ok {
		t.Fatalf("%#v", b.Value)
	}
}

// TestContains_EscapesLikeWildcards covers V-34: user-supplied text
// passed to Contains/IContains/StartsWith/EndsWith must have LIKE's
// special characters ('\', '%', '_') escaped, so e.g. searching for the
// literal string "50%_off" does not behave as a wildcard pattern.
func TestContains_EscapesLikeWildcards(t *testing.T) {
	name := field.Col[string]("products", "name")

	cases := []struct {
		build func(text string) field.Pred
		text  string
		want  string
	}{
		{name.Contains, "50%_off", `%50\%\_off%`},
		{name.IContains, "50%_off", `%50\%\_off%`},
		{name.StartsWith, `a\b`, `a\\b%`},
		{name.EndsWith, "100%", `%100\%`},
	}
	for _, tc := range cases {
		p := tc.build(tc.text)
		b, ok := p.Node().(expr.Binary)
		if !ok {
			t.Fatalf("%#v", p.Node())
		}
		if got := b.Value.(string); got != tc.want {
			t.Errorf("text=%q: got pattern %q, want %q", tc.text, got, tc.want)
		}
	}
}
