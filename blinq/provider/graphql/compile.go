// Package graphql compiles a BLinq AST subset to GraphQL (query string + variables)
// and executes via HTTP Poster — no GraphQL client dependency.
package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

// Compiled is GraphQL query + variables.
type Compiled struct {
	Query     string
	Variables map[string]any
}

// CompileQuery translates a SELECT subset → selection set like:
//
//	query($where: ..., $limit: Int) { users(where: $where, limit: $limit) { ...fields } }
func CompileQuery(q expr.Query) (Compiled, error) {
	if q.Kind != expr.KindSelect {
		return Compiled{}, fmt.Errorf("blinq/graphql: only Select (got %d)", q.Kind)
	}
	if len(q.Joins) > 0 || len(q.GroupBy) > 0 || q.Having != nil {
		return Compiled{}, fmt.Errorf("blinq/graphql: Join/GroupBy/Having fora do subset")
	}

	entity := q.From.Name
	vars := map[string]any{}
	var args []string
	var defs []string

	if q.Where != nil {
		where, err := predToInput(q.Where)
		if err != nil {
			return Compiled{}, err
		}
		vars["where"] = where
		defs = append(defs, "$where: JSON")
		args = append(args, "where: $where")
	}
	if q.Limit != nil {
		vars["limit"] = int(*q.Limit)
		defs = append(defs, "$limit: Int")
		args = append(args, "limit: $limit")
	}
	if q.Offset != nil {
		vars["offset"] = int(*q.Offset)
		defs = append(defs, "$offset: Int")
		args = append(args, "offset: $offset")
	}
	if len(q.OrderBy) > 0 {
		order := make([]map[string]any, 0, len(q.OrderBy))
		for _, o := range q.OrderBy {
			dir := "ASC"
			if o.Dir == expr.OrderDesc {
				dir = "DESC"
			}
			order = append(order, map[string]any{"field": o.Field.Column, "dir": dir})
		}
		vars["order"] = order
		defs = append(defs, "$order: JSON")
		args = append(args, "order: $order")
	}

	fields := selectionFields(q)
	var sb strings.Builder
	sb.WriteString("query")
	if len(defs) > 0 {
		sb.WriteString("(")
		sb.WriteString(strings.Join(defs, ", "))
		sb.WriteString(")")
	}
	sb.WriteString(" { ")
	sb.WriteString(entity)
	if len(args) > 0 {
		sb.WriteString("(")
		sb.WriteString(strings.Join(args, ", "))
		sb.WriteString(")")
	}
	sb.WriteString(" { ")
	sb.WriteString(strings.Join(fields, " "))
	sb.WriteString(" } }")

	return Compiled{Query: sb.String(), Variables: vars}, nil
}

func selectionFields(q expr.Query) []string {
	seen := map[string]bool{}
	var out []string
	add := func(c string) {
		if c == "" || seen[c] {
			return
		}
		seen[c] = true
		out = append(out, c)
	}
	for _, s := range q.Selects {
		add(s.Field.Column)
	}
	if q.Where != nil {
		collectPredCols(q.Where, add)
	}
	for _, o := range q.OrderBy {
		add(o.Field.Column)
	}
	for _, inc := range q.Includes {
		name := inc.Name
		if name == "" {
			name = inc.Target.Name
		}
		add(name + " { id }")
	}
	if len(out) == 0 {
		return []string{"id"}
	}
	sort.Strings(out)
	return out
}

func collectPredCols(p expr.Predicate, add func(string)) {
	switch n := p.(type) {
	case expr.Binary:
		add(n.Field.Column)
	case expr.And:
		for _, it := range n.Items {
			collectPredCols(it, add)
		}
	case expr.Or:
		for _, it := range n.Items {
			collectPredCols(it, add)
		}
	case expr.Not:
		collectPredCols(n.Inner, add)
	}
}

func predToInput(p expr.Predicate) (map[string]any, error) {
	switch n := p.(type) {
	case expr.Binary:
		return binaryToInput(n)
	case expr.And:
		items := make([]any, 0, len(n.Items))
		for _, it := range n.Items {
			m, err := predToInput(it)
			if err != nil {
				return nil, err
			}
			items = append(items, m)
		}
		return map[string]any{"and": items}, nil
	case expr.Or:
		items := make([]any, 0, len(n.Items))
		for _, it := range n.Items {
			m, err := predToInput(it)
			if err != nil {
				return nil, err
			}
			items = append(items, m)
		}
		return map[string]any{"or": items}, nil
	case expr.Not:
		inner, err := predToInput(n.Inner)
		if err != nil {
			return nil, err
		}
		return map[string]any{"not": inner}, nil
	default:
		return nil, fmt.Errorf("blinq/graphql: pred %T", p)
	}
}

func binaryToInput(b expr.Binary) (map[string]any, error) {
	col := b.Field.Column
	switch b.Op {
	case expr.OpEq:
		return map[string]any{col: map[string]any{"eq": b.Value}}, nil
	case expr.OpNeq:
		return map[string]any{col: map[string]any{"neq": b.Value}}, nil
	case expr.OpGt:
		return map[string]any{col: map[string]any{"gt": b.Value}}, nil
	case expr.OpGte:
		return map[string]any{col: map[string]any{"gte": b.Value}}, nil
	case expr.OpLt:
		return map[string]any{col: map[string]any{"lt": b.Value}}, nil
	case expr.OpLte:
		return map[string]any{col: map[string]any{"lte": b.Value}}, nil
	case expr.OpIn:
		vals, ok := b.Value.([]any)
		if !ok {
			return nil, fmt.Errorf("blinq/graphql: IN subquery not in this subset")
		}
		return map[string]any{col: map[string]any{"in": vals}}, nil
	case expr.OpBetween:
		vals, _ := b.Value.([]any)
		return map[string]any{col: map[string]any{"between": vals}}, nil
	case expr.OpIsNull:
		return map[string]any{col: map[string]any{"is_null": true}}, nil
	case expr.OpIsNotNull:
		return map[string]any{col: map[string]any{"is_null": false}}, nil
	case expr.OpLike, expr.OpILike:
		return map[string]any{col: map[string]any{"like": b.Value}}, nil
	default:
		return nil, fmt.Errorf("blinq/graphql: op %s", b.Op)
	}
}

// Poster sends GraphQL (query + variables) and returns entity rows.
type Poster interface {
	PostGraphQL(ctx context.Context, query string, variables map[string]any) ([]map[string]any, error)
}

// Provider runs SELECT via HTTP GraphQL.
type Provider struct {
	Client Poster
}

var _ provider.Provider = (*Provider)(nil)

func (p *Provider) Explain(_ context.Context, q expr.Query) (provider.Plan, error) {
	c, err := CompileQuery(q)
	if err != nil {
		return provider.Plan{}, err
	}
	b, err := json.Marshal(map[string]any{"query": c.Query, "variables": c.Variables})
	if err != nil {
		return provider.Plan{}, err
	}
	return provider.Plan{Text: string(b)}, nil
}

func (p *Provider) Execute(ctx context.Context, q expr.Query, _ ...provider.Option) (provider.Result, error) {
	if p.Client == nil {
		return nil, fmt.Errorf("blinq/graphql: Client nil")
	}
	c, err := CompileQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := p.Client.PostGraphQL(ctx, c.Query, c.Variables)
	if err != nil {
		return nil, err
	}
	return &mapResult{rows: rows}, nil
}

type mapResult struct {
	rows []map[string]any
}

func (r *mapResult) Err() error            { return nil }
func (r *mapResult) Count() (int64, error) { return int64(len(r.rows)), nil }
func (r *mapResult) ScanSlice(dest any) error {
	return scanMaps(dest, r.rows)
}
func (r *mapResult) ScanOne(dest any) error {
	if len(r.rows) == 0 {
		return provider.ErrNotFound
	}
	return scan.MapToStruct(r.rows[0], dest)
}

func scanMaps(dest any, rows []map[string]any) error {
	if dest == nil {
		return fmt.Errorf("blinq/graphql: ScanSlice dest nil")
	}
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("blinq/graphql: ScanSlice expects *[]T")
	}
	slice := rv.Elem()
	et := slice.Type().Elem()
	if et.Kind() == reflect.Map {
		out := reflect.MakeSlice(slice.Type(), len(rows), len(rows))
		for i, row := range rows {
			out.Index(i).Set(reflect.ValueOf(row))
		}
		slice.Set(out)
		return nil
	}
	out := reflect.MakeSlice(slice.Type(), 0, len(rows))
	for _, row := range rows {
		elem := reflect.New(et)
		if err := scan.MapToStruct(row, elem.Interface()); err != nil {
			return err
		}
		out = reflect.Append(out, elem.Elem())
	}
	slice.Set(out)
	return nil
}
