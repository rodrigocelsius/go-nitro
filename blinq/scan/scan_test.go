package scan_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

type row struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

func TestMapToStruct(t *testing.T) {
	var u row
	err := scan.MapToStruct(map[string]any{"id": int64(7), "name": "Ada"}, &u)
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 7 || u.Name != "Ada" {
		t.Fatalf("%#v", u)
	}
}

func TestMapsToSlice(t *testing.T) {
	out, err := scan.MapsToSlice[row]([]map[string]any{
		{"id": int64(1), "name": "A"},
		{"id": int64(2), "name": "B"},
	})
	if err != nil || len(out) != 2 {
		t.Fatalf("%v %#v", err, out)
	}
}
