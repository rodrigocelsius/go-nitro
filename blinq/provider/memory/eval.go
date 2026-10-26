package memory

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

func evalBinary(b expr.Binary, row map[string]any) (bool, error) {
	col := b.Field.Column
	val, ok := row[col]
	if !ok {
		for k, v := range row {
			if strings.EqualFold(k, col) {
				val, ok = v, true
				break
			}
		}
	}
	switch b.Op {
	case expr.OpIsNull:
		return !ok || val == nil, nil
	case expr.OpIsNotNull:
		return ok && val != nil, nil
	case expr.OpIn:
		list, _ := b.Value.([]any)
		for _, item := range list {
			if cmpEqual(val, item) {
				return true, nil
			}
		}
		return false, nil
	case expr.OpBetween:
		pair, _ := b.Value.([]any)
		if len(pair) != 2 {
			return false, fmt.Errorf("BETWEEN needs 2 values")
		}
		return cmpLessEqual(pair[0], val) && cmpLessEqual(val, pair[1]), nil
	case expr.OpEq:
		return cmpEqual(val, b.Value), nil
	case expr.OpNeq:
		return !cmpEqual(val, b.Value), nil
	case expr.OpGt:
		return cmpLess(b.Value, val), nil // val > b.Value
	case expr.OpGte:
		return cmpLessEqual(b.Value, val), nil
	case expr.OpLt:
		return cmpLess(val, b.Value), nil
	case expr.OpLte:
		return cmpLessEqual(val, b.Value), nil
	case expr.OpLike, expr.OpILike:
		pattern, _ := b.Value.(string)
		s := fmt.Sprint(val)
		if b.Op == expr.OpILike {
			return matchLike(strings.ToLower(s), strings.ToLower(pattern)), nil
		}
		return matchLike(s, pattern), nil
	default:
		return false, fmt.Errorf("blinq/memory: unsupported op %s", b.Op)
	}
}

// likeToken is a single unit of a parsed LIKE pattern: either a literal
// byte (possibly produced by unescaping "\%"/"\_"/"\\") or a "%" wildcard.
type likeToken struct {
	wildcard bool
	ch       byte
}

// parseLikePattern tokenizes a LIKE pattern, honoring backslash escaping
// of '%'/'_'/'\' (matching field.Contains/StartsWith/EndsWith and the SQL
// providers' "ESCAPE '\'" clause — see vuln.md V-34). Only "%" acts as a
// wildcard here (consistent with the pre-existing "% wildcard only"
// behavior of this in-memory matcher); "_" and any other character are
// always literal.
func parseLikePattern(pattern string) []likeToken {
	toks := make([]likeToken, 0, len(pattern))
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' && i+1 < len(pattern) {
			i++
			toks = append(toks, likeToken{ch: pattern[i]})
			continue
		}
		if c == '%' {
			toks = append(toks, likeToken{wildcard: true})
			continue
		}
		toks = append(toks, likeToken{ch: c})
	}
	return toks
}

// matchLike reports whether s matches the LIKE pattern (see
// parseLikePattern). Uses the classic greedy-with-backtracking algorithm
// for a single wildcard type ("%" = any run of characters, including
// empty).
func matchLike(s, pattern string) bool {
	toks := parseLikePattern(pattern)
	sb := []byte(s)
	si, ti := 0, 0
	starTok, starS := -1, 0
	for si < len(sb) {
		switch {
		case ti < len(toks) && !toks[ti].wildcard && toks[ti].ch == sb[si]:
			si++
			ti++
		case ti < len(toks) && toks[ti].wildcard:
			starTok = ti
			starS = si
			ti++
		case starTok != -1:
			ti = starTok + 1
			starS++
			si = starS
		default:
			return false
		}
	}
	for ti < len(toks) && toks[ti].wildcard {
		ti++
	}
	return ti == len(toks)
}

func cmpEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	return reflect.DeepEqual(normalize(a), normalize(b))
}

func normalize(v any) any {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(rv.Uint())
	case reflect.Float32, reflect.Float64:
		return rv.Float()
	default:
		return v
	}
}

func asFloat(v any) (float64, bool) {
	switch x := normalize(v).(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	default:
		return 0, false
	}
}

func cmpLess(a, b any) bool {
	fa, oka := asFloat(a)
	fb, okb := asFloat(b)
	if oka && okb {
		return fa < fb
	}
	return fmt.Sprint(a) < fmt.Sprint(b)
}

func cmpLessEqual(a, b any) bool {
	return cmpEqual(a, b) || cmpLess(a, b)
}

func lessRows(a, b map[string]any, orders []expr.Order) bool {
	for _, o := range orders {
		av, bv := a[o.Field.Column], b[o.Field.Column]
		if cmpEqual(av, bv) {
			continue
		}
		less := cmpLess(av, bv)
		if o.Dir == expr.OrderDesc {
			return !less
		}
		return less
	}
	return false
}

func project(rows []map[string]any, selects []expr.SelectItem) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		m := make(map[string]any, len(selects))
		for _, s := range selects {
			if s.All {
				out[i] = row
				break
			}
			m[s.Field.Column] = row[s.Field.Column]
		}
		if out[i] == nil {
			out[i] = m
		}
	}
	return out
}

func distinctRows(rows []map[string]any, selects []expr.SelectItem) []map[string]any {
	seen := map[string]struct{}{}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		key := rowKey(row, selects)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}
	return out
}

func rowKey(row map[string]any, selects []expr.SelectItem) string {
	var b strings.Builder
	if len(selects) == 0 || selects[0].All {
		keys := make([]string, 0, len(row))
		for k := range row {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s=%v;", k, row[k])
		}
		return b.String()
	}
	for _, s := range selects {
		fmt.Fprintf(&b, "%s=%v;", s.Field.Column, row[s.Field.Column])
	}
	return b.String()
}
