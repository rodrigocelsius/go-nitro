package nitro

import (
	"encoding/json"
	"testing"
)

type sortItem struct {
	Model
	Rank int `json:"rank"`
}

func TestResourceDefaultSort(t *testing.T) {
	repo := NewMemoryRepo[sortItem]()
	app := New(Config{DisableDefaults: true})
	app.Resource("/sort_items", &sortItem{}, AdaptRepo(repo), &ResourceConfig{
		NoCache: true, Public: true, Sortable: []string{"rank"}, DefaultSort: "-rank",
	})
	tc := NewTestClient(app)
	for _, r := range []string{`{"rank":1}`, `{"rank":3}`, `{"rank":2}`} {
		tc.AssertStatus(t, tc.Post(t, "/sort_items", r), 201)
	}
	ranks := func(path string) []int {
		var out struct {
			Items []sortItem `json:"items"`
		}
		res := tc.Get(t, path)
		if err := json.Unmarshal(res.Body(), &out); err != nil {
			t.Fatal(err)
		}
		r := make([]int, len(out.Items))
		for i, it := range out.Items {
			r[i] = it.Rank
		}
		return r
	}
	if got := ranks("/sort_items"); len(got) != 3 || got[0] != 3 || got[2] != 1 {
		t.Fatalf("default sort -rank: %v", got)
	}
	if got := ranks("/sort_items?sort=rank"); got[0] != 1 || got[2] != 3 {
		t.Fatalf("explicit sort must win: %v", got)
	}
}
