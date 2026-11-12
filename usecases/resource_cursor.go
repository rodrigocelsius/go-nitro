package usecases

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// CursorStart is the ?cursor= value that starts cursor paging from the
// first row (a list answered with offset paging also carries next_cursor).
const CursorStart = "start"

// maxCursorLen bounds the ?cursor= token (it is client-controlled input).
const maxCursorLen = 4096

// cursorPage is the response of a cursor-paged list: no total and no page
// number — computing them is the cost cursor paging avoids.
type cursorPage struct {
	Items      []any  `json:"items"`
	PerPage    int    `json:"per_page"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// cursorToken is the decoded ?cursor= value. It is opaque to clients but
// not secret or trusted: its values are only ever used as bound comparison
// parameters next to the same filters (tenant included) as any other page.
type cursorToken struct {
	// Sort is the ordering the cursor was issued for; a cursor is only
	// valid with the same ?sort.
	Sort   string   `json:"s"`
	Values []string `json:"v"`
	ID     string   `json:"id"`
}

var errBadCursor = &domain.CodedError{Status: 400, Code: "INVALID_CURSOR", Message: "invalid or expired cursor"}

func sortSignature(sorts []domain.ListSort) string {
	var b strings.Builder
	for _, s := range sorts {
		if s.Desc {
			b.WriteByte('-')
		}
		b.WriteString(s.Field)
		b.WriteByte(',')
	}
	return b.String()
}

func encodeCursor(sorts []domain.ListSort, values []string, id string) string {
	raw, err := json.Marshal(cursorToken{Sort: sortSignature(sorts), Values: values, ID: id})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// decodeCursor parses ?cursor=. CursorStart yields a nil position.
func decodeCursor(token string, sorts []domain.ListSort) (*domain.ListCursor, error) {
	if token == CursorStart {
		return nil, nil
	}
	if len(token) > maxCursorLen {
		return nil, errBadCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, errBadCursor
	}
	var t cursorToken
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, errBadCursor
	}
	if t.Sort != sortSignature(sorts) || len(t.Values) != len(sorts) || t.ID == "" {
		return nil, errBadCursor
	}
	return &domain.ListCursor{Values: t.Values, ID: t.ID}, nil
}

// cursorFieldValue renders one field of an item for a cursor. wire selects
// the JSON form (RFC 3339 times, plain numbers), which is what the SQL
// stores parse back into column values; otherwise the form the in-memory
// comparison uses.
func cursorFieldValue(item any, jsonName string, wire bool) string {
	if !wire {
		return jsonFieldString(item, jsonName)
	}
	rv := derefStruct(item)
	if !rv.IsValid() {
		return jsonFieldString(item, jsonName)
	}
	v, ok := structFieldByJSONName(rv, jsonName)
	if !ok {
		return ""
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return jsonFieldString(item, jsonName)
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	if string(raw) == "null" {
		return ""
	}
	return string(raw)
}

// nextCursor builds the cursor that continues after item.
func nextCursor(item any, sorts []domain.ListSort, idField string, wire bool) string {
	id := cursorFieldValue(item, idField, wire)
	if id == "" {
		return ""
	}
	values := make([]string, len(sorts))
	for i, s := range sorts {
		values[i] = cursorFieldValue(item, s.Field, wire)
	}
	return encodeCursor(sorts, values, id)
}

// compareKeyset orders item against a keyset position (sort values, then
// id): negative when item comes before it in the listing, positive after.
func compareKeyset(item any, sorts []domain.ListSort, idField string, values []string, id string) int {
	for i, s := range sorts {
		cmp := compareValues(jsonFieldString(item, s.Field), values[i])
		if cmp != 0 {
			if s.Desc {
				return -cmp
			}
			return cmp
		}
	}
	return compareValues(jsonFieldString(item, idField), id)
}

// sortItemsKeyset sorts in-memory items by the list ordering with the
// primary key as the final tiebreaker — the total order cursor paging needs
// (and the one the SQL stores apply).
func sortItemsKeyset(items []any, sorts []domain.ListSort, idField string) {
	sort.SliceStable(items, func(i, j int) bool {
		for _, s := range sorts {
			cmp := compareValues(jsonFieldString(items[i], s.Field), jsonFieldString(items[j], s.Field))
			if cmp != 0 {
				if s.Desc {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return compareValues(jsonFieldString(items[i], idField), jsonFieldString(items[j], idField)) < 0
	})
}

// pageAfter returns the perPage items that follow the keyset position in
// items (already in keyset order) and whether more remain.
func pageAfter(items []any, after *domain.ListCursor, sorts []domain.ListSort, idField string, perPage int) (page []any, more bool) {
	start := 0
	if after != nil {
		start = sort.Search(len(items), func(i int) bool {
			return compareKeyset(items[i], sorts, idField, after.Values, after.ID) > 0
		})
	}
	end := start + perPage
	if end >= len(items) {
		return items[start:], false
	}
	return items[start:end], true
}

// derefStruct returns the struct value behind item (a struct or a pointer
// to one), or the zero Value.
func derefStruct(item any) reflect.Value {
	rv := reflect.ValueOf(item)
	for rv.IsValid() && rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return reflect.Value{}
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	return rv
}
