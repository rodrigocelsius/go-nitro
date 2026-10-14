package nitro

import "testing"

func TestPluralizeSnake_Irregulars(t *testing.T) {
	cases := map[string]string{
		"person": "people", "child": "children", "user_person": "user_people",
		"product": "products", "category": "categories", "address": "addresses",
		"box": "boxes", "status": "statuses", "key": "keys",
	}
	for in, want := range cases {
		if got := pluralizeSnake(in); got != want {
			t.Errorf("pluralizeSnake(%q)=%q want %q", in, got, want)
		}
	}
	RegisterPlural("cactus", "cacti")
	if got := pluralizeSnake("cactus"); got != "cacti" {
		t.Errorf("RegisterPlural: %q", got)
	}
}
