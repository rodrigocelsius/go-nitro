// Package api serializes BLinq AST to JSON wire (POST /query) — REST subset.
package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
)

// Wire is the public JSON format.
type Wire struct {
	Entity  string         `json:"entity"`
	Where   map[string]any `json:"where,omitempty"`
	Order   []WireOrder    `json:"order,omitempty"`
	Limit   *uint64        `json:"limit,omitempty"`
	Offset  *uint64        `json:"offset,omitempty"`
	Include []string       `json:"include,omitempty"`
}

// WireOrder is ORDER BY on the wire.
type WireOrder struct {
	Field string `json:"field"`
	Dir   string `json:"dir"` // asc|desc
}

// Encode serializes a SELECT subset → JSON.
func Encode(q expr.Query) ([]byte, error) {
	w, err := ToWire(q)
	if err != nil {
		return nil, err
	}
	return json.Marshal(w)
}

// ToWire converts AST → Wire.
func ToWire(q expr.Query) (Wire, error) {
	if q.Kind != expr.KindSelect {
		return Wire{}, fmt.Errorf("blinq/api: only Select on wire v1")
	}
	if len(q.Joins) > 0 || len(q.GroupBy) > 0 {
		return Wire{}, fmt.Errorf("blinq/api: Join/GroupBy fora do subset")
	}
	w := Wire{Entity: q.From.Name, Limit: q.Limit, Offset: q.Offset}
	if q.Where != nil {
		m, err := predToMap(q.Where)
		if err != nil {
			return Wire{}, err
		}
		w.Where = m
	}
	for _, o := range q.OrderBy {
		dir := "asc"
		if o.Dir == expr.OrderDesc {
			dir = "desc"
		}
		w.Order = append(w.Order, WireOrder{Field: o.Field.Column, Dir: dir})
	}
	for _, inc := range q.Includes {
		name := inc.Name
		if name == "" {
			name = inc.Target.Name
		}
		w.Include = append(w.Include, name)
	}
	return w, nil
}

func predToMap(p expr.Predicate) (map[string]any, error) {
	switch n := p.(type) {
	case expr.Binary:
		return binaryToMap(n)
	case expr.And:
		items := make([]any, 0, len(n.Items))
		for _, it := range n.Items {
			m, err := predToMap(it)
			if err != nil {
				return nil, err
			}
			items = append(items, m)
		}
		return map[string]any{"and": items}, nil
	case expr.Or:
		items := make([]any, 0, len(n.Items))
		for _, it := range n.Items {
			m, err := predToMap(it)
			if err != nil {
				return nil, err
			}
			items = append(items, m)
		}
		return map[string]any{"or": items}, nil
	case expr.Not:
		inner, err := predToMap(n.Inner)
		if err != nil {
			return nil, err
		}
		return map[string]any{"not": inner}, nil
	default:
		return nil, fmt.Errorf("blinq/api: pred %T", p)
	}
}

func binaryToMap(b expr.Binary) (map[string]any, error) {
	col := b.Field.Column
	op := string(b.Op)
	switch b.Op {
	case expr.OpEq:
		return map[string]any{col: map[string]any{"eq": b.Value}}, nil
	case expr.OpNeq:
		return map[string]any{col: map[string]any{"neq": b.Value}}, nil
	case expr.OpGt, expr.OpGte, expr.OpLt, expr.OpLte:
		key := map[expr.Op]string{
			expr.OpGt: "gt", expr.OpGte: "gte", expr.OpLt: "lt", expr.OpLte: "lte",
		}[b.Op]
		return map[string]any{col: map[string]any{key: b.Value}}, nil
	case expr.OpIn:
		vals, ok := b.Value.([]any)
		if !ok {
			return nil, fmt.Errorf("blinq/api: IN subquery not supported on wire v1")
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
		return nil, fmt.Errorf("blinq/api: op %s", op)
	}
}

// Poster sends the JSON body and returns rows.
type Poster interface {
	PostQuery(ctx context.Context, body []byte) ([]map[string]any, error)
}

// Provider runs SELECT via HTTP JSON.
type Provider struct {
	Client Poster
}

var _ provider.Provider = (*Provider)(nil)

func (p *Provider) Explain(_ context.Context, q expr.Query) (provider.Plan, error) {
	b, err := Encode(q)
	if err != nil {
		return provider.Plan{}, err
	}
	return provider.Plan{Text: string(b)}, nil
}

func (p *Provider) Execute(ctx context.Context, q expr.Query, _ ...provider.Option) (provider.Result, error) {
	if p.Client == nil {
		return nil, fmt.Errorf("blinq/api: Client nil")
	}
	body, err := Encode(q)
	if err != nil {
		return nil, err
	}
	rows, err := p.Client.PostQuery(ctx, body)
	if err != nil {
		return nil, err
	}
	return &mapResult{rows: rows}, nil
}

// ExecuteConcurrent is a parallelism no-op: API wire does not parallelize MultiValues;
// delegates to Execute (ConcurrentCapable for Wrap compatibility).
func (p *Provider) ExecuteConcurrent(ctx context.Context, q expr.Query, _ int, opts ...provider.Option) (provider.Result, error) {
	return p.Execute(ctx, q, opts...)
}

var _ provider.ConcurrentCapable = (*Provider)(nil)

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
	return scanOne(dest, r.rows[0])
}
