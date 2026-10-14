package nitro

import "testing"

func TestListResult(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	r := ListResult(items, 2, 2)
	if r.Total != 5 || r.Page != 2 || r.PerPage != 2 || r.TotalPages != 3 {
		t.Fatalf("meta = %+v", r)
	}
	if len(r.Items) != 2 {
		t.Fatalf("items len = %d", len(r.Items))
	}
	if r.Items[0] != "c" || r.Items[1] != "d" {
		t.Fatalf("items = %v", r.Items)
	}
}
