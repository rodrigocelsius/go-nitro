package nitro

import (
	"reflect"
	"strings"
	"sync"
)

// ResourcePath returns the URL path / table name segment (no leading slash),
// e.g. "products" — the same convention app.Resource uses (TableName()
// method, else snake_case plural of the struct name). Exported for adapters
// that derive a table from a model outside app.Resource (e.g. sqladapter.RepositoryFor).
func ResourcePath(model any) string {
	return resourcePathFromModel(model)
}

// resourcePathFromModel returns the URL path segment (no leading slash), e.g. "products".
// Prefer TableName() string method; else snake_case plural of the struct name.
func resourcePathFromModel(model any) string {
	if model == nil {
		return "resources"
	}
	if tn, ok := tableNameOf(model); ok && tn != "" {
		return strings.Trim(tn, "/")
	}
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return "resources"
	}
	return pluralizeSnake(SnakeCase(t.Name()))
}

func tableNameOf(model any) (string, bool) {
	rv := reflect.ValueOf(model)
	if !rv.IsValid() {
		return "", false
	}
	// Prefer value receiver TableName on T or *T.
	for _, v := range []reflect.Value{rv, reflect.Indirect(rv)} {
		if !v.IsValid() {
			continue
		}
		m := v.MethodByName("TableName")
		if !m.IsValid() && v.CanAddr() {
			m = v.Addr().MethodByName("TableName")
		}
		if !m.IsValid() {
			continue
		}
		if m.Type().NumIn() != 0 || m.Type().NumOut() != 1 || m.Type().Out(0).Kind() != reflect.String {
			continue
		}
		out := m.Call(nil)
		if s, ok := out[0].Interface().(string); ok && s != "" {
			return s, true
		}
	}
	return "", false
}

// irregularPlurals maps the last snake_case word of a model name to its plural.
var (
	pluralMu         sync.RWMutex
	irregularPlurals = map[string]string{
		"person": "people", "child": "children", "man": "men", "woman": "women",
		"foot": "feet", "tooth": "teeth", "mouse": "mice", "goose": "geese",
		"ox": "oxen", "datum": "data", "criterion": "criteria", "analysis": "analyses",
		"status": "statuses", "bus": "buses", "series": "series", "species": "species",
		"news": "news", "media": "media", "sheep": "sheep", "fish": "fish",
	}
)

// RegisterPlural teaches the convention an irregular plural (call at startup,
// before registering resources): nitro.RegisterPlural("cactus", "cacti").
// For a one-off, prefer app.Resource("/cacti", &Cactus{}) or a TableName() method.
func RegisterPlural(singular, plural string) {
	pluralMu.Lock()
	irregularPlurals[strings.ToLower(singular)] = plural
	pluralMu.Unlock()
}

func pluralizeSnake(word string) string {
	if word == "" {
		return word
	}
	lower := strings.ToLower(word)
	// Irregulars match on the last "_"-separated word (user_person -> user_people).
	head, last := "", lower
	if i := strings.LastIndex(lower, "_"); i >= 0 {
		head, last = word[:i+1], lower[i+1:]
	}
	pluralMu.RLock()
	irr, ok := irregularPlurals[last]
	pluralMu.RUnlock()
	if ok {
		return head + irr
	}
	switch {
	case strings.HasSuffix(lower, "y") && len(lower) > 1 && !isVowel(rune(lower[len(lower)-2])):
		return word[:len(word)-1] + "ies"
	case strings.HasSuffix(lower, "s"),
		strings.HasSuffix(lower, "x"),
		strings.HasSuffix(lower, "ch"),
		strings.HasSuffix(lower, "sh"):
		return word + "es"
	default:
		return word + "s"
	}
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}

// inferResourceFields walks model tags: blinq:"index" → filterable + sortable.
// Also sorts by id / created_at when those columns exist.
func inferResourceFields(model any) (filterable, sortable []string) {
	t := reflect.TypeOf(model)
	if t == nil {
		return nil, nil
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, nil
	}
	seenF, seenS := map[string]bool{}, map[string]bool{}
	add := func(dst *[]string, seen map[string]bool, name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		*dst = append(*dst, name)
	}
	var walk func(reflect.Type)
	walk = func(rt reflect.Type) {
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if f.Anonymous {
				ft := f.Type
				if ft.Kind() == reflect.Ptr {
					ft = ft.Elem()
				}
				if ft.Kind() == reflect.Struct {
					walk(ft)
				}
				continue
			}
			if f.PkgPath != "" {
				continue
			}
			col := columnJSONName(f)
			blinq := f.Tag.Get("blinq")
			if hasBlinqToken(blinq, "index") {
				add(&filterable, seenF, col)
				add(&sortable, seenS, col)
			}
			switch col {
			case "id", "created_at", "updated_at":
				add(&sortable, seenS, col)
			}
		}
	}
	walk(t)
	return filterable, sortable
}

// columnJSONName resolves the filter/sort key for a field: explicit `json`
// tag → explicit `db` tag → snake_case of the Go field name (same default
// convention used when both tags are omitted; see FieldNames / dbColumnName).
func columnJSONName(f reflect.StructField) string {
	if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
		if n := strings.Split(tag, ",")[0]; n != "" {
			return n
		}
	}
	if tag := f.Tag.Get("db"); tag != "" && tag != "-" {
		if n := strings.Split(tag, ",")[0]; n != "" {
			return n
		}
	}
	return SnakeCase(f.Name)
}

func hasBlinqToken(tag, want string) bool {
	for _, p := range strings.Split(tag, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if i := strings.IndexByte(p, '='); i >= 0 {
			p = p[:i]
		}
		if p == want {
			return true
		}
	}
	return false
}

func applyInferredFilters(model any, cfg *ResourceConfig, opts *resourceOpts) {
	if cfg == nil && opts == nil {
		return
	}
	needFilter := (cfg != nil && cfg.Filterable == nil && cfg.FilterRules == nil) ||
		(opts != nil && opts.Filterable == nil && len(opts.FilterRules) == 0)
	needSort := (cfg != nil && cfg.Sortable == nil) || (opts != nil && opts.Sortable == nil)
	if !needFilter && !needSort {
		return
	}
	f, s := inferResourceFields(model)
	if needFilter && len(f) > 0 {
		if cfg != nil {
			cfg.Filterable = append([]string(nil), f...)
		}
		if opts != nil {
			opts.Filterable = append([]string(nil), f...)
		}
	}
	if needSort && len(s) > 0 {
		if cfg != nil {
			cfg.Sortable = append([]string(nil), s...)
		}
		if opts != nil {
			opts.Sortable = append([]string(nil), s...)
		}
	}
}
