package nitro_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type pagedItem struct {
	nitro.Model
	Name      string
	UnitPrice float64
}

func seedPaged(t *testing.T, app *nitro.App, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		body := fmt.Sprintf(`{"name":"item-%02d","unit_price":%d}`, i, i%4)
		r, _ := app.TestWithHeaders("POST", "/paged_items", strings.NewReader(body), map[string]string{"Content-Type": "application/json"})
		if r.StatusCode() != 201 {
			t.Fatalf("create %d %s", r.StatusCode(), r.Body())
		}
	}
}

type listBody struct {
	Items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"items"`
	Total      *int   `json:"total"`
	Page       int    `json:"page"`
	NextCursor string `json:"next_cursor"`
	HasMore    *bool  `json:"has_more"`
}

func getList(t *testing.T, app *nitro.App, q string) (int, listBody, string) {
	t.Helper()
	r, err := app.Test("GET", "/paged_items"+q, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out listBody
	_ = json.Unmarshal(r.Body(), &out)
	return r.StatusCode(), out, string(r.Body())
}

// StrictQuery turns an unknown parameter or sort field into a 400 instead
// of silently answering the unfiltered list.
func TestResource_StrictQuery(t *testing.T) {
	newApp := func(strict bool) *nitro.App {
		app := nitro.New(nitro.Config{DisableDefaults: true, Env: nitro.EnvDevelopment})
		app.Resource(&pagedItem{}, nitro.NewMemoryRepo[pagedItem](), &nitro.ResourceConfig{
			Public:      true,
			Filterable:  []string{"name", "unit_price"},
			Sortable:    []string{"unit_price"},
			FilterRules: []nitro.FilterRule{{Param: "q", Field: "name", Op: "icontains"}},
			StrictQuery: strict,
			AllowQuery:  []string{"_ts"},
		})
		seedPaged(t, app, 3)
		return app
	}

	lenient := newApp(false)
	if code, body, _ := getList(t, lenient, "?nmae=item-00&sort=name"); code != 200 || len(body.Items) != 3 {
		t.Fatalf("lenient: %d with %d items (a typo is ignored by default)", code, len(body.Items))
	}

	strict := newApp(true)
	for q, wantCode := range map[string]string{
		"?nmae=item-00":    "UNKNOWN_QUERY_PARAM",
		"?unit_price_ge=1": "UNKNOWN_QUERY_PARAM",
		"?sort=name":       "INVALID_SORT",
		"?cursor=start":    "UNKNOWN_QUERY_PARAM", // cursor paging is off here
	} {
		code, _, raw := getList(t, strict, q)
		if code != 400 || !strings.Contains(raw, wantCode) {
			t.Errorf("%s: %d %s, want 400 %s", q, code, raw, wantCode)
		}
	}
	for _, q := range []string{
		"", "?name=item-00", "?unit_price_gte=1&unit_price_lt=3", "?unitPrice=1", "?unitPriceGte=1",
		"?q=item&sort=-unit_price&page=1&per_page=2", "?sort=unitPrice", "?_ts=123",
	} {
		if code, _, raw := getList(t, strict, q); code != 200 {
			t.Errorf("%s: %d %s, want 200", q, code, raw)
		}
	}
}

// Every accepted spelling of a filter still reaches the same filter, with
// the documented precedence (literal, then snake_case, then camelCase).
func TestResource_FilterSpellings(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true, Env: nitro.EnvDevelopment})
	app.Resource(&pagedItem{}, nitro.NewMemoryRepo[pagedItem](), &nitro.ResourceConfig{
		Public: true, Filterable: []string{"name", "unit_price"},
	})
	seedPaged(t, app, 8)
	for q, want := range map[string]int{
		"?unit_price=1":                 2,
		"?unitPrice=1":                  2,
		"?unit_price_gte=2":             4,
		"?unitPriceGte=2":               4,
		"?unit_price=1&unitPrice=3":     2, // literal wins over the camel alias
		"?unitPrice=3&unit_price=1":     2,
		"?unit_price=&unitPrice=3":      2, // an empty literal falls back to the alias
		"?unit_price=1&unit_price=3":    2, // first occurrence wins
		"?name=item-03&unit_price_lt=9": 1,
	} {
		if _, body, raw := getList(t, app, q); len(body.Items) != want {
			t.Errorf("%s: %d items, want %d (%s)", q, len(body.Items), want, raw)
		}
	}
}

// Cursor paging walks the whole list, in the same order as offset paging,
// without repeating or skipping rows — also when the sort column has
// duplicates and with the per-process cache on.
func TestResource_CursorPaging(t *testing.T) {
	for _, cached := range []bool{false, true} {
		cfg := &nitro.ResourceConfig{
			Public: true, Sortable: []string{"unit_price", "name"}, Filterable: []string{"unit_price"},
			CursorPaging: true,
		}
		if cached {
			cfg.CacheTTL = 60_000_000_000
		}
		app := nitro.New(nitro.Config{DisableDefaults: true, Env: nitro.EnvDevelopment})
		app.Resource(&pagedItem{}, nitro.NewMemoryRepo[pagedItem](), cfg)
		seedPaged(t, app, 23)

		for _, sortQ := range []string{"", "&sort=unit_price", "&sort=-unit_price", "&sort=-name", "&unit_price_gte=2&sort=unit_price"} {
			_, all, _ := getList(t, app, "?per_page=100"+sortQ)
			var want []string
			for _, it := range all.Items {
				want = append(want, it.ID)
			}
			if len(want) == 0 || all.NextCursor != "" {
				t.Fatalf("%q: full page: %d items, next_cursor %q", sortQ, len(want), all.NextCursor)
			}

			// Offset page 1 hands out the cursor; cursor pages follow.
			code, first, raw := getList(t, app, "?per_page=5"+sortQ)
			if code != 200 || first.NextCursor == "" || first.Total == nil {
				t.Fatalf("%q: first page %d %s", sortQ, code, raw)
			}
			var got []string
			for _, it := range first.Items {
				got = append(got, it.ID)
			}
			cursor := first.NextCursor
			for pages := 0; cursor != "" && pages < 20; pages++ {
				code, page, raw := getList(t, app, "?per_page=5&cursor="+cursor+sortQ)
				if code != 200 || page.HasMore == nil || page.Total != nil {
					t.Fatalf("%q: cursor page %d %s", sortQ, code, raw)
				}
				for _, it := range page.Items {
					got = append(got, it.ID)
				}
				if *page.HasMore != (page.NextCursor != "") {
					t.Fatalf("%q: has_more=%v next_cursor=%q", sortQ, *page.HasMore, page.NextCursor)
				}
				cursor = page.NextCursor
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("cached=%v %q:\ncursor = %v\noffset = %v", cached, sortQ, got, want)
			}

			// ?cursor=start begins cursor paging without an offset page.
			_, start, _ := getList(t, app, "?per_page=5&cursor=start"+sortQ)
			if len(start.Items) == 0 || start.Items[0].ID != want[0] {
				t.Errorf("%q: cursor=start does not begin at the first row", sortQ)
			}
		}

		// A cursor issued for one sort is refused with another, and garbage is a 400.
		_, first, _ := getList(t, app, "?per_page=5&sort=unit_price")
		for _, q := range []string{"?per_page=5&sort=-name&cursor=" + first.NextCursor, "?cursor=not-a-cursor", "?cursor=" + strings.Repeat("A", 5000)} {
			if code, _, raw := getList(t, app, q); code != 400 || !strings.Contains(raw, "INVALID_CURSOR") {
				t.Errorf("%.40s: %d %s, want 400 INVALID_CURSOR", q, code, raw)
			}
		}
	}
}

// Without CursorPaging the list response is unchanged and ?cursor is ignored.
func TestResource_CursorPagingIsOptIn(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true, Env: nitro.EnvDevelopment})
	app.Resource(&pagedItem{}, nitro.NewMemoryRepo[pagedItem](), &nitro.ResourceConfig{Public: true})
	seedPaged(t, app, 7)
	code, body, raw := getList(t, app, "?per_page=3&cursor=start")
	if code != 200 || strings.Contains(raw, "next_cursor") || strings.Contains(raw, "has_more") || body.Total == nil || *body.Total != 7 {
		t.Fatalf("%d %s", code, raw)
	}
}
