package usecases

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
)

// listIndex is the query-string vocabulary of one Resource list, computed
// once at registration: every accepted parameter spelling points at the
// filter it feeds. A request is then read in a single pass over its
// arguments instead of one lookup per (field × operator × spelling).
type listIndex struct {
	// slots are the possible filters in emission order: Filterable
	// equality, then `<field>_<op>` suffixes, then FilterRules — the order
	// buildListQuery always produced (it decides which filters survive
	// MaxFilters and the shape of the cache key).
	slots []listSlot
	// params maps a query-parameter spelling to the slots it feeds.
	params map[string][]listParam
	// reserved are the non-filter parameters the list handler reads.
	reserved map[string]bool
}

type listSlot struct {
	field string
	op    string
}

// listParam is one spelling of a slot. rank orders the spellings as
// queryParam tries them: 0 literal, 1 snake_case, 2 camelCase.
type listParam struct {
	slot int
	rank int8
}

func newListIndex(opts ResourceOpts) *listIndex {
	idx := &listIndex{
		params:   make(map[string][]listParam),
		reserved: map[string]bool{"page": true, "per_page": true, "sort": true},
	}
	if opts.CursorPaging {
		idx.reserved["cursor"] = true
	}
	for _, name := range opts.AllowQuery {
		idx.reserved[name] = true
	}
	camel := acceptCamelQueryEnabled(opts)

	add := func(param, field, op string) {
		slot := len(idx.slots)
		idx.slots = append(idx.slots, listSlot{field: field, op: op})
		idx.params[param] = append(idx.params[param], listParam{slot: slot})
		if !camel || param == "" {
			return
		}
		snake, cam := nameVariants(param)
		if snake != param {
			idx.params[snake] = append(idx.params[snake], listParam{slot: slot, rank: 1})
		}
		if cam != param && cam != snake {
			idx.params[cam] = append(idx.params[cam], listParam{slot: slot, rank: 2})
		}
	}

	for _, name := range opts.Filterable {
		add(name, name, "eq")
	}
	for _, name := range opts.Filterable {
		for _, op := range filterOpSuffixes {
			add(opParamName(name, op), name, op)
		}
	}
	for _, rule := range opts.FilterRules {
		param := rule.Param
		if param == "" {
			param = rule.Field
		}
		field := rule.Field
		if field == "" {
			field = param
		}
		add(param, field, rule.Op)
	}
	return idx
}

// collect reads the filter values of a request in one pass. ok is false
// when the adapter cannot enumerate the query string (the caller falls back
// to per-parameter lookups).
func (idx *listIndex) collect(c *domain.Ctx) (vals []string, ok bool) {
	if len(idx.slots) == 0 {
		return nil, true
	}
	var ranks []int8
	ok = c.VisitQuery(func(key, value string) {
		if value == "" {
			return
		}
		for _, p := range idx.params[key] {
			if vals == nil {
				vals = make([]string, len(idx.slots))
				ranks = make([]int8, len(idx.slots))
			}
			// First occurrence of the best-ranked spelling wins, as with
			// queryParam (literal, then snake_case, then camelCase).
			if vals[p.slot] == "" || p.rank < ranks[p.slot] {
				vals[p.slot], ranks[p.slot] = value, p.rank
			}
		}
	})
	return vals, ok
}

// validate is the strict mode (ResourceOpts.StrictQuery): a parameter the
// Resource does not know, or a sort field it does not allow, is a client
// mistake answered with 400 — not silently dropped, which returns the
// unfiltered list to someone who believes it is filtered.
func (idx *listIndex) validate(c *domain.Ctx, opts ResourceOpts) error {
	var unknown string
	enumerated := c.VisitQuery(func(key, _ string) {
		if unknown != "" || idx.reserved[key] {
			return
		}
		if _, ok := idx.params[key]; !ok {
			unknown = key
		}
	})
	if !enumerated {
		return nil
	}
	if unknown != "" {
		return &domain.CodedError{
			Status:  400,
			Code:    "UNKNOWN_QUERY_PARAM",
			Message: "unknown query parameter: " + clampFilterValue(unknown, 64),
		}
	}
	if sortKey := c.QueryParam("sort"); sortKey != "" {
		if _, _, ok := resolveSort(sortKey, opts); !ok {
			return &domain.CodedError{
				Status:  400,
				Code:    "INVALID_SORT",
				Message: "cannot sort by: " + clampFilterValue(strings.TrimPrefix(sortKey, "-"), 64),
			}
		}
	}
	return nil
}

// resolveSort maps ?sort=[-]field to a Sortable field (canonical spelling).
func resolveSort(sortKey string, opts ResourceOpts) (field string, desc, ok bool) {
	if strings.HasPrefix(sortKey, "-") {
		desc = true
		sortKey = strings.TrimPrefix(sortKey, "-")
	}
	camel := acceptCamelQueryEnabled(opts)
	for _, a := range opts.Sortable {
		if a == sortKey {
			return a, desc, true
		}
		if camel {
			if aSnake, aCamel := nameVariants(a); aSnake == sortKey || aCamel == sortKey {
				return a, desc, true
			}
		}
	}
	if camel {
		// Rare spellings (e.g. "UnitPrice"): convert the request value once.
		snake, cam := transform.SnakeCase(sortKey), transform.CamelCase(sortKey)
		for _, a := range opts.Sortable {
			if a == snake || a == cam {
				return a, desc, true
			}
		}
	}
	return "", desc, false
}
