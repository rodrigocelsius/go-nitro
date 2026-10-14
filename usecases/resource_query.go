package usecases

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
)

// acceptCamelQueryEnabled is true unless AcceptCamelQuery is an explicit false pointer.
func acceptCamelQueryEnabled(opts ResourceOpts) bool {
	return opts.AcceptCamelQuery == nil || *opts.AcceptCamelQuery
}

// queryParam tries the literal name, then camel/snake aliases when enabled.
func queryParam(c *domain.Ctx, name string, camelAliases bool) string {
	if v := c.QueryParam(name); v != "" {
		return v
	}
	if !camelAliases || name == "" {
		return ""
	}
	snake, camel := nameVariants(name)
	if snake != name {
		if v := c.QueryParam(snake); v != "" {
			return v
		}
	}
	if camel != name {
		if v := c.QueryParam(camel); v != "" {
			return v
		}
	}
	return ""
}

// Query-param names come from ResourceOpts (a bounded set), so their
// snake/camel variants and "<field>_<op>" names are computed once instead of
// on every list request. Never cache request-supplied strings here.
var (
	variantCache sync.Map // name → [2]string{snake, camel}
	opNameCache  sync.Map // [2]string{field, op} → field_op
)

func nameVariants(name string) (snake, camel string) {
	if v, ok := variantCache.Load(name); ok {
		p := v.([2]string)
		return p[0], p[1]
	}
	p := [2]string{transform.SnakeCase(name), transform.CamelCase(name)}
	variantCache.Store(name, p)
	return p[0], p[1]
}

func opParamName(field, op string) string {
	key := [2]string{field, op}
	if v, ok := opNameCache.Load(key); ok {
		return v.(string)
	}
	n := field + "_" + op
	opNameCache.Store(key, n)
	return n
}

// buildListQuery builds ListQuery from query params + ResourceOpts.
// Only Filterable / FilterRules fields are accepted; values and count are clamped (anti-DoS).
func buildListQuery(c *domain.Ctx, page, perPage int, opts ResourceOpts) domain.ListQuery {
	lq := domain.ListQuery{Page: page, PerPage: perPage}
	maxFilters := opts.MaxFilters
	if maxFilters <= 0 {
		maxFilters = DefaultListMaxFilters
	}
	maxLen := opts.MaxFilterValueLen
	if maxLen <= 0 {
		maxLen = DefaultListMaxFilterLen
	}
	camel := acceptCamelQueryEnabled(opts)

	add := func(field, value, op string) {
		if len(lq.Filters) >= maxFilters {
			return
		}
		value = clampFilterValue(value, maxLen)
		if value == "" {
			return
		}
		if op == "" {
			op = "eq"
		}
		lq.Filters = append(lq.Filters, domain.ListFilter{Field: field, Value: value, Op: op})
	}

	for _, name := range opts.Filterable {
		if v := queryParam(c, name, camel); v != "" {
			add(name, v, "eq")
		}
	}
	// Operator suffixes on any Filterable field: ?price_gte=10&price_lt=50&name_contains=ab
	for _, name := range opts.Filterable {
		for _, op := range filterOpSuffixes {
			if v := queryParam(c, opParamName(name, op), camel); v != "" {
				add(name, v, op)
			}
		}
	}
	for _, rule := range opts.FilterRules {
		param := rule.Param
		if param == "" {
			param = rule.Field
		}
		if v := queryParam(c, param, camel); v != "" {
			field := rule.Field
			if field == "" {
				field = param
			}
			add(field, v, rule.Op)
		}
	}
	if sortKey := queryParam(c, "sort", false); sortKey != "" {
		desc := false
		if strings.HasPrefix(sortKey, "-") {
			desc = true
			sortKey = strings.TrimPrefix(sortKey, "-")
		}
		canonical := sortKey
		ok := false
		for _, a := range opts.Sortable {
			if a == sortKey {
				ok, canonical = true, a
				break
			}
			if camel {
				if aSnake, aCamel := nameVariants(a); aSnake == sortKey || aCamel == sortKey {
					ok, canonical = true, a
					break
				}
			}
		}
		if !ok && camel {
			// Rare spellings (e.g. "UnitPrice"): convert the request value once.
			snake, cam := transform.SnakeCase(sortKey), transform.CamelCase(sortKey)
			for _, a := range opts.Sortable {
				if a == snake || a == cam {
					ok, canonical = true, a
					break
				}
			}
		}
		if ok {
			lq.Sort = append(lq.Sort, domain.ListSort{Field: canonical, Desc: desc})
		}
	}
	return lq
}

// filterOpSuffixes are the operators accepted as `<field>_<op>` query params.
// They are the ops every store understands (see ResourceConfig.FilterRules).
var filterOpSuffixes = []string{"gt", "gte", "lt", "lte", "contains", "icontains"}

func clampFilterValue(v string, maxLen int) string {
	v = strings.TrimSpace(v)
	if maxLen > 0 && len(v) > maxLen {
		return v[:maxLen]
	}
	return v
}

func effectiveDefaultPerPage(opts ResourceOpts) int {
	if opts.DefaultPerPage > 0 {
		return opts.DefaultPerPage
	}
	return DefaultListPerPage
}

func effectiveMaxPerPage(opts ResourceOpts) int {
	if opts.MaxPerPage > 0 {
		return opts.MaxPerPage
	}
	return DefaultListMaxPerPage
}

func effectiveMaxPage(opts ResourceOpts) int {
	if opts.MaxPage > 0 {
		return opts.MaxPage
	}
	return DefaultListMaxPage
}

func effectiveMaxInMemory(opts ResourceOpts) int {
	if opts.MaxInMemoryFetch > 0 {
		return opts.MaxInMemoryFetch
	}
	return DefaultListMaxInMemory
}

// clampListPagination reads ?page / ?per_page and applies anti-DoS caps.
func clampListPagination(c *domain.Ctx, opts ResourceOpts) (page, perPage int) {
	defPer := effectiveDefaultPerPage(opts)
	maxPer := effectiveMaxPerPage(opts)
	maxPage := effectiveMaxPage(opts)
	page = queryIntDefault(c, "page", 1)
	perPage = queryIntDefault(c, "per_page", defPer)
	if page < 1 {
		page = 1
	}
	if maxPage > 0 && page > maxPage {
		page = maxPage
	}
	if perPage < 1 {
		perPage = defPer
	}
	if maxPer > 0 && perPage > maxPer {
		perPage = maxPer
	}
	return page, perPage
}

func filterItems(c *domain.Ctx, items []any, allow []string) []any {
	return filterItemsOpts(c, items, ResourceOpts{Filterable: allow})
}

// filterItemsOpts applies the same filters the SQL stores push down
// (Filterable equality, `<field>_<op>` suffixes and FilterRules) to an
// in-memory slice, so MemoryRepo behaves like Postgres.
func filterItemsOpts(c *domain.Ctx, items []any, opts ResourceOpts) []any {
	if len(items) == 0 || (len(opts.Filterable) == 0 && len(opts.FilterRules) == 0) {
		return items
	}
	lq := buildListQuery(c, 1, 1, opts)
	if len(lq.Filters) == 0 {
		return items
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		ok := true
		for _, f := range lq.Filters {
			if !matchFilter(item, f) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, item)
		}
	}
	return out
}

func matchFilter(item any, f domain.ListFilter) bool {
	got := jsonFieldString(item, f.Field)
	want := f.Value
	switch strings.ToLower(f.Op) {
	case "contains":
		return strings.Contains(got, want)
	case "icontains":
		return strings.Contains(strings.ToLower(got), strings.ToLower(want))
	case "gt":
		return compareValues(got, want) > 0
	case "gte":
		return compareValues(got, want) >= 0
	case "lt":
		return compareValues(got, want) < 0
	case "lte":
		return compareValues(got, want) <= 0
	default: // eq
		if a, b, ok := bothNumbers(got, want); ok {
			return a == b
		}
		return strings.EqualFold(got, want)
	}
}

func bothNumbers(a, b string) (float64, float64, bool) {
	x, err1 := strconv.ParseFloat(a, 64)
	y, err2 := strconv.ParseFloat(b, 64)
	return x, y, err1 == nil && err2 == nil
}

// compareValues compares numerically when both sides are numbers, else as strings.
func compareValues(a, b string) int {
	if x, y, ok := bothNumbers(a, b); ok {
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	return strings.Compare(a, b)
}

func sortItems(c *domain.Ctx, items []any, allow []string) []any {
	if len(items) == 0 {
		return items
	}
	sortKey := c.QueryParam("sort")
	if sortKey == "" {
		return items
	}
	desc := false
	if strings.HasPrefix(sortKey, "-") {
		desc = true
		sortKey = strings.TrimPrefix(sortKey, "-")
	}
	if len(allow) > 0 {
		ok := false
		for _, a := range allow {
			if a == sortKey {
				ok = true
				break
			}
		}
		if !ok {
			return items
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		cmp := compareValues(jsonFieldString(items[i], sortKey), jsonFieldString(items[j], sortKey))
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
	return items
}

func jsonFieldString(item any, jsonName string) string {
	rv := reflect.ValueOf(item)
	if !rv.IsValid() {
		return ""
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Map {
		for _, k := range rv.MapKeys() {
			if fmt.Sprint(k.Interface()) == jsonName {
				return fmt.Sprint(rv.MapIndex(k).Interface())
			}
		}
		return ""
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Sprint(item)
	}
	if v, ok := structFieldByJSONName(rv, jsonName); ok {
		return fmt.Sprint(v)
	}
	return ""
}

// structFieldByJSONName finds a field by json tag, Go name (case-insensitive)
// or the snake_case default, descending into embedded structs (nitro.Model).
func structFieldByJSONName(rv reflect.Value, jsonName string) (any, bool) {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() && !f.Anonymous {
			continue
		}
		name := f.Name
		if tag := f.Tag.Get("json"); tag != "" {
			if n := strings.Split(tag, ",")[0]; n != "" && n != "-" {
				name = n
			}
		}
		if f.Anonymous && f.Tag.Get("json") == "" {
			ev := rv.Field(i)
			if ev.Kind() == reflect.Ptr {
				if ev.IsNil() {
					continue
				}
				ev = ev.Elem()
			}
			if ev.Kind() == reflect.Struct {
				if v, ok := structFieldByJSONName(ev, jsonName); ok {
					return v, true
				}
			}
			continue
		}
		if name == jsonName || strings.EqualFold(f.Name, jsonName) || transform.SnakeCase(f.Name) == jsonName {
			return rv.Field(i).Interface(), true
		}
	}
	return nil, false
}

func paginateAnyItems(items []any, page, perPage int) *domain.ResourceListResult {
	if perPage <= 0 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}
	total := len(items)
	totalPages := (total + perPage - 1) / perPage
	offset := (page - 1) * perPage
	if offset > total {
		offset = total
	}
	end := offset + perPage
	if end > total {
		end = total
	}
	pageItems := items[offset:end]
	return &domain.ResourceListResult{
		Items:      pageItems,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}
}
