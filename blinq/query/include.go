package query

import (
	"context"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
)

// Include marks a relation for batch load after the parent SELECT (anti-N+1).
func (q Query) Include(rels ...field.Relation) Query {
	q = q.clone()
	for _, r := range rels {
		q.ast.Includes = append(q.ast.Includes, r.ToInclude())
	}
	return q
}

// CollectMaps runs SELECT and hydrates Includes into []map[string]any.
func (q Query) CollectMaps(ctx context.Context, p provider.Provider, opts ...provider.Option) ([]map[string]any, error) {
	ast := q.AST()
	incs := ast.Includes
	ast.Includes = nil // provider does not need includes on SELECT

	res, err := p.Execute(ctx, ast, opts...)
	if err != nil {
		return nil, err
	}
	var parents []map[string]any
	if err := res.ScanSlice(&parents); err != nil {
		return nil, err
	}
	if len(incs) == 0 {
		return parents, nil
	}
	if err := hydrateIncludes(ctx, p, parents, incs, opts...); err != nil {
		return nil, err
	}
	return parents, nil
}

func hydrateIncludes(ctx context.Context, p provider.Provider, parents []map[string]any, incs []expr.Include, opts ...provider.Option) error {
	for _, inc := range incs {
		if err := hydrateOne(ctx, p, parents, inc, opts...); err != nil {
			return err
		}
	}
	return nil
}

func hydrateOne(ctx context.Context, p provider.Provider, parents []map[string]any, inc expr.Include, opts ...provider.Option) error {
	ids := make([]any, 0, len(parents))
	seen := make(map[string]struct{}, len(parents))
	for _, row := range parents {
		v, ok := lookupCol(row, inc.LocalKey.Column)
		if !ok || v == nil {
			continue
		}
		key := fmt.Sprint(v)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, v)
	}
	name := inc.Name
	if name == "" {
		name = inc.Target.Name
	}
	if len(ids) == 0 {
		for _, row := range parents {
			row[name] = []map[string]any{}
		}
		return nil
	}

	childAST := expr.NewSelect(inc.Target)
	childAST.Where = expr.Binary{Field: inc.ForeignKey, Op: expr.OpIn, Value: ids}
	res, err := p.Execute(ctx, childAST, opts...)
	if err != nil {
		return fmt.Errorf("blinq: include %q: %w", name, err)
	}
	var children []map[string]any
	if err := res.ScanSlice(&children); err != nil {
		return err
	}

	byFK := make(map[string][]map[string]any, len(ids))
	fkCol := inc.ForeignKey.Column
	for _, ch := range children {
		v, ok := lookupCol(ch, fkCol)
		if !ok {
			continue
		}
		k := fmt.Sprint(v)
		byFK[k] = append(byFK[k], ch)
	}
	localCol := inc.LocalKey.Column
	for _, row := range parents {
		v, ok := lookupCol(row, localCol)
		if !ok {
			row[name] = []map[string]any{}
			continue
		}
		if list, ok := byFK[fmt.Sprint(v)]; ok {
			row[name] = list
		} else {
			row[name] = []map[string]any{}
		}
	}
	return nil
}

func lookupCol(row map[string]any, col string) (any, bool) {
	if v, ok := row[col]; ok {
		return v, true
	}
	for k, v := range row {
		if equalFoldASCII(k, col) {
			return v, true
		}
	}
	return nil, false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
