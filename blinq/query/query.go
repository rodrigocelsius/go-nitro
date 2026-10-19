package query

import (
	"context"
	"fmt"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
)

// Query is an immutable SELECT/DML builder.
type Query struct {
	ast       expr.Query
	page      *uint64
	perPage   *uint64
	withCount bool
	soft      bool
	softCol   string
	hasTenant bool
	tenantID  any
	tenantCol string
}

// PageResult (also called Page in the DX) is the generic paginated result.
type PageResult[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	TotalPages int   `json:"total_pages"`
}

// Page is an ergonomic alias (same layout as PageResult).
type Page[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	TotalPages int   `json:"total_pages"`
}

// From starts a SELECT on the entity.
// If SoftColumn is set, soft-delete is on (filter IS NULL); use Hard() to include deleted rows.
func From(e field.Entity) Query {
	q := Query{ast: expr.NewSelect(e.Ref())}
	q.softCol = e.SoftColumn
	q.tenantCol = e.TenantColumn
	if e.SoftColumn != "" {
		q.soft = true
	}
	return q
}

// FromName starts SELECT with only a table name (prototypes).
func FromName(table string) Query {
	return From(field.Entity{Name: table})
}

// Soft enables soft-delete: SELECT/UPDATE filter deleted; DELETE → UPDATE SET deleted_at.
// If Entity already has SoftColumn, Soft() is redundant (already on in From/Update/DeleteFrom).
func (q Query) Soft() Query {
	q = q.clone()
	q.soft = true
	if q.softCol == "" {
		q.softCol = "deleted_at"
	}
	return q
}

// Hard turns off soft-delete on this query (SELECT includes deleted; physical DELETE).
func (q Query) Hard() Query {
	q = q.clone()
	q.soft = false
	return q
}

// SoftColumn sets the soft-delete column (before or with Soft).
func (q Query) SoftColumn(col string) Query {
	q = q.clone()
	q.softCol = col
	return q
}

// Tenant applies AND tenant_col = id on SELECT/UPDATE/DELETE (and INSERT if Values).
func (q Query) Tenant(id any) Query {
	q = q.clone()
	q.hasTenant = true
	q.tenantID = id
	if q.tenantCol == "" {
		q.tenantCol = "tenant_id"
	}
	return q
}

// TenantColumn sets the tenant column.
func (q Query) TenantColumn(col string) Query {
	q = q.clone()
	q.tenantCol = col
	return q
}

// Cache marks the query with a TTL hint (honored by provider.Wrap + Cache).
func (q Query) Cache(ttl time.Duration) Query {
	q = q.clone()
	if q.ast.Cache == nil {
		q.ast.Cache = &expr.CacheHint{TTL: ttl}
	} else {
		q.ast.Cache.TTL = ttl
	}
	return q
}

// CacheTag attaches a tag for invalidation (provider.Cache.InvalidateTag).
func (q Query) CacheTag(tag string) Query {
	q = q.clone()
	if q.ast.Cache == nil {
		q.ast.Cache = &expr.CacheHint{Tag: tag}
	} else {
		q.ast.Cache.Tag = tag
	}
	return q
}

// AST returns the tree (Page/PerPage + Soft/Tenant resolved).
func (q Query) AST() expr.Query {
	out := q.ast
	if q.page != nil && q.perPage != nil {
		off := (*q.page - 1) * *q.perPage
		lim := *q.perPage
		out.Limit = &lim
		out.Offset = &off
	}
	return resolveHints(out, q)
}

// WithCountEnabled reports whether List/Paginate should count the total.
func (q Query) WithCountEnabled() bool { return q.withCount }

func (q Query) clone() Query {
	out := q
	if q.ast.Selects != nil {
		out.ast.Selects = append([]expr.SelectItem(nil), q.ast.Selects...)
	}
	if q.ast.OrderBy != nil {
		out.ast.OrderBy = append([]expr.Order(nil), q.ast.OrderBy...)
	}
	if q.ast.Joins != nil {
		out.ast.Joins = append([]expr.Join(nil), q.ast.Joins...)
	}
	if q.ast.GroupBy != nil {
		out.ast.GroupBy = append([]expr.FieldRef(nil), q.ast.GroupBy...)
	}
	if q.ast.Sets != nil {
		out.ast.Sets = append([]expr.Assignment(nil), q.ast.Sets...)
	}
	if q.ast.Columns != nil {
		out.ast.Columns = append([]string(nil), q.ast.Columns...)
	}
	if q.ast.Values != nil {
		out.ast.Values = append([]any(nil), q.ast.Values...)
	}
	if q.ast.MultiValues != nil {
		out.ast.MultiValues = make([][]any, len(q.ast.MultiValues))
		for i, row := range q.ast.MultiValues {
			out.ast.MultiValues[i] = append([]any(nil), row...)
		}
	}
	if q.ast.Returning != nil {
		out.ast.Returning = append([]expr.FieldRef(nil), q.ast.Returning...)
	}
	if q.ast.Conflict != nil {
		c := *q.ast.Conflict
		c.Columns = append([]expr.FieldRef(nil), q.ast.Conflict.Columns...)
		c.Updates = append([]expr.Assignment(nil), q.ast.Conflict.Updates...)
		out.ast.Conflict = &c
	}
	if q.ast.Cache != nil {
		c := *q.ast.Cache
		out.ast.Cache = &c
	}
	if q.ast.Includes != nil {
		out.ast.Includes = append([]expr.Include(nil), q.ast.Includes...)
	}
	return out
}

func andWhere(where expr.Predicate, extra expr.Predicate) expr.Predicate {
	if extra == nil {
		return where
	}
	if where == nil {
		return extra
	}
	return expr.And{Items: []expr.Predicate{where, extra}}
}

func resolveHints(out expr.Query, q Query) expr.Query {
	if q.soft {
		out.SoftDelete = true
		out.SoftColumn = q.softCol
		col := expr.FieldRef{Table: out.From.Name, Column: q.softCol}
		switch out.Kind {
		case expr.KindDelete:
			out.Kind = expr.KindUpdate
			out.Sets = []expr.Assignment{{Field: col, Value: expr.SoftNow{}}}
			out.SoftDelete = false // already rewritten; avoids a compile loop
		case expr.KindSelect, expr.KindUpdate, expr.KindAggregate:
			out.Where = andWhere(out.Where, expr.Binary{Field: col, Op: expr.OpIsNull})
		}
	}
	if q.hasTenant {
		out.TenantID = q.tenantID
		out.TenantColumn = q.tenantCol
		col := expr.FieldRef{Table: out.From.Name, Column: q.tenantCol}
		pred := expr.Binary{Field: col, Op: expr.OpEq, Value: q.tenantID}
		switch out.Kind {
		case expr.KindSelect, expr.KindUpdate, expr.KindDelete, expr.KindAggregate:
			out.Where = andWhere(out.Where, pred)
		case expr.KindInsert, expr.KindUpsert:
			// injeta coluna se ausente
			found := false
			for _, c := range out.Columns {
				if c == q.tenantCol {
					found = true
					break
				}
			}
			if !found {
				out.Columns = append(append([]string(nil), out.Columns...), q.tenantCol)
				out.Values = append(append([]any(nil), out.Values...), q.tenantID)
				// Bulk insert: the tenant goes into ALL MultiValues rows,
				// otherwise compile fails with "row length mismatch".
				if len(out.MultiValues) > 0 {
					rows := make([][]any, len(out.MultiValues))
					for i, row := range out.MultiValues {
						grown := make([]any, len(row)+1)
						copy(grown, row)
						grown[len(row)] = q.tenantID
						rows[i] = grown
					}
					out.MultiValues = rows
				}
			}
		}
	}
	return out
}

// Where adds a predicate (AND with the existing one).
func (q Query) Where(p field.Pred) Query {
	q = q.clone()
	node := p.Node()
	if q.ast.Where == nil {
		q.ast.Where = node
		return q
	}
	q.ast.Where = expr.And{Items: []expr.Predicate{q.ast.Where, node}}
	return q
}

// WhereAll adds several AND conditions.
func (q Query) WhereAll(preds ...field.Pred) Query {
	for _, p := range preds {
		q = q.Where(p)
	}
	return q
}

// WhereIf adds a predicate only if ok.
func (q Query) WhereIf(ok bool, p field.Pred) Query {
	if !ok {
		return q
	}
	return q.Where(p)
}

// OrderByIf adds ORDER BY only if ok.
func (q Query) OrderByIf(ok bool, orders ...expr.Order) Query {
	if !ok {
		return q
	}
	return q.OrderBy(orders...)
}

// Select sets columns (empty = *).
func (q Query) Select(fields ...expr.FieldRef) Query {
	q = q.clone()
	if len(fields) == 0 {
		q.ast.Selects = []expr.SelectItem{{All: true}}
		return q
	}
	items := make([]expr.SelectItem, len(fields))
	for i, f := range fields {
		items[i] = expr.SelectItem{Field: f}
	}
	q.ast.Selects = items
	return q
}

// SelectFields accepts Field[T] via Ref.
func SelectFields[T any](q Query, fields ...field.Field[T]) Query {
	refs := make([]expr.FieldRef, len(fields))
	for i, f := range fields {
		refs[i] = f.Ref()
	}
	return q.Select(refs...)
}

// OrderBy appends ORDER BY ASC/DESC.
func (q Query) OrderBy(orders ...expr.Order) Query {
	q = q.clone()
	q.ast.OrderBy = append(append([]expr.Order(nil), q.ast.OrderBy...), orders...)
	return q
}

// Distinct marks SELECT DISTINCT.
func (q Query) Distinct() Query {
	q = q.clone()
	q.ast.Distinct = true
	return q
}

// Limit sets LIMIT.
func (q Query) Limit(n uint64) Query {
	q = q.clone()
	q.ast.Limit = &n
	return q
}

// ForUpdate locks the selected rows until the transaction ends
// (SELECT … FOR UPDATE). Use inside a transaction.
//
//	jobs, err := store.Find(ctx, store.Query().Where(pending).Limit(100).ForUpdate().SkipLocked())
func (q Query) ForUpdate() Query {
	q = q.clone()
	l := q.lock()
	l.Share = false
	return q
}

// ForShare takes a shared row lock (SELECT … FOR SHARE).
func (q Query) ForShare() Query {
	q = q.clone()
	q.lock().Share = true
	return q
}

// SkipLocked skips rows locked by other transactions — the building block of
// multi-consumer job queues. Implies ForUpdate when no lock mode was set.
func (q Query) SkipLocked() Query {
	q = q.clone()
	l := q.lock()
	l.SkipLocked, l.NoWait = true, false
	return q
}

// NoWait fails immediately instead of waiting for locked rows.
func (q Query) NoWait() Query {
	q = q.clone()
	l := q.lock()
	l.NoWait, l.SkipLocked = true, false
	return q
}

// lock returns the (copied) lock clause, creating FOR UPDATE by default.
func (q *Query) lock() *expr.LockClause {
	if q.ast.Lock == nil {
		q.ast.Lock = &expr.LockClause{}
	} else {
		c := *q.ast.Lock
		q.ast.Lock = &c
	}
	return q.ast.Lock
}

// Offset sets OFFSET.
func (q Query) Offset(n uint64) Query {
	q = q.clone()
	q.ast.Offset = &n
	return q
}

// Page sets the page (with PerPage → offset).
func (q Query) Page(n uint64) Query {
	q = q.clone()
	q.page = &n
	return q
}

// PerPage sets records per page.
func (q Query) PerPage(n uint64) Query {
	q = q.clone()
	q.perPage = &n
	return q
}

// Paginate is a shortcut for Page + PerPage + WithCount.
func (q Query) Paginate(page, size uint64) Query {
	return q.Page(page).PerPage(size).WithCount()
}

// WithCount enables the total count (List/pagination).
func (q Query) WithCount() Query {
	q = q.clone()
	q.withCount = true
	return q
}

// WithoutCount disables counting.
func (q Query) WithoutCount() Query {
	q = q.clone()
	q.withCount = false
	return q
}

// Skip is an alias of Offset (LINQ DX). Accepts int to avoid uint64 casts.
func (q Query) Skip(n int) Query {
	if n < 0 {
		n = 0
	}
	return q.Offset(uint64(n))
}

// Take is an alias of Limit. Accepts int to avoid uint64 casts.
func (q Query) Take(n int) Query {
	if n < 0 {
		n = 0
	}
	return q.Limit(uint64(n))
}

// Execute runs on the provider (with Chunk on bulk insert/upsert).
func (q Query) Execute(ctx context.Context, p provider.Provider, opts ...provider.Option) (provider.Result, error) {
	ast := q.AST()
	if ast.ChunkSize > 0 && len(ast.MultiValues) > ast.ChunkSize {
		return executeChunked(ctx, p, ast, opts...)
	}
	return p.Execute(ctx, ast, opts...)
}

func executeChunked(ctx context.Context, p provider.Provider, ast expr.Query, opts ...provider.Option) (provider.Result, error) {
	size := ast.ChunkSize
	rows := ast.MultiValues
	var total int64
	for i := 0; i < len(rows); i += size {
		end := i + size
		if end > len(rows) {
			end = len(rows)
		}
		chunk := ast
		chunk.MultiValues = rows[i:end]
		chunk.ChunkSize = 0
		res, err := p.Execute(ctx, chunk, opts...)
		if err != nil {
			return nil, err
		}
		n, _ := res.Count()
		total += n
	}
	return &chunkResult{n: total}, nil
}

type chunkResult struct{ n int64 }

func (r *chunkResult) Err() error               { return nil }
func (r *chunkResult) Count() (int64, error)    { return r.n, nil }
func (r *chunkResult) ScanSlice(dest any) error { return fmt.Errorf("blinq: chunked exec has no rows") }
func (r *chunkResult) ScanOne(dest any) error   { return provider.ErrNotFound }

// Explain returns a plan without executing.
func (q Query) Explain(ctx context.Context, p provider.Provider) (provider.Plan, error) {
	return p.Explain(ctx, q.AST())
}

// Collect scans into a slice.
func (q Query) Collect(ctx context.Context, p provider.Provider, dest any, opts ...provider.Option) error {
	res, err := q.Execute(ctx, p, opts...)
	if err != nil {
		return err
	}
	return res.ScanSlice(dest)
}

// First scans a single record.
func (q Query) First(ctx context.Context, p provider.Provider, dest any, opts ...provider.Option) error {
	q2 := q.Limit(1)
	res, err := q2.Execute(ctx, p, opts...)
	if err != nil {
		return err
	}
	return res.ScanOne(dest)
}

// Counter is a provider that can COUNT(*) without materializing rows.
type Counter interface {
	CountQuery(ctx context.Context, q expr.Query) (int64, error)
}

// Aggregator runs typed SUM/AVG/MIN/MAX/COUNT.
type Aggregator interface {
	AggregateQuery(ctx context.Context, q expr.Query) (float64, error)
}

// Count counts records (COUNT(*) in SQL; filtered len in memory).
func (q Query) Count(ctx context.Context, p provider.Provider, opts ...provider.Option) (int64, error) {
	ast := q.AST()
	if c, ok := p.(Counter); ok {
		return c.CountQuery(ctx, ast)
	}
	res, err := p.Execute(ctx, ast, opts...)
	if err != nil {
		return 0, err
	}
	return res.Count()
}

// Sum aggregates SUM(col).
func (q Query) Sum(ctx context.Context, p provider.Provider, col expr.FieldRef) (float64, error) {
	return q.aggregate(ctx, p, expr.Aggregate{Func: expr.AggSum, Field: col})
}

// Avg aggregates AVG(col).
func (q Query) Avg(ctx context.Context, p provider.Provider, col expr.FieldRef) (float64, error) {
	return q.aggregate(ctx, p, expr.Aggregate{Func: expr.AggAvg, Field: col})
}

// Min aggregates MIN(col).
func (q Query) Min(ctx context.Context, p provider.Provider, col expr.FieldRef) (float64, error) {
	return q.aggregate(ctx, p, expr.Aggregate{Func: expr.AggMin, Field: col})
}

// Max aggregates MAX(col).
func (q Query) Max(ctx context.Context, p provider.Provider, col expr.FieldRef) (float64, error) {
	return q.aggregate(ctx, p, expr.Aggregate{Func: expr.AggMax, Field: col})
}

func (q Query) aggregate(ctx context.Context, p provider.Provider, agg expr.Aggregate) (float64, error) {
	ast := q.AST()
	ast.Kind = expr.KindAggregate
	ast.Aggregate = &agg
	ast.Selects = nil
	ast.OrderBy = nil
	ast.Limit = nil
	ast.Offset = nil
	if a, ok := p.(Aggregator); ok {
		return a.AggregateQuery(ctx, ast)
	}
	return 0, fmt.Errorf("%w: aggregate", provider.ErrUnsupported)
}

// PaginateCollect runs the page + total (if WithCount / Paginate).
func PaginateCollect[T any](ctx context.Context, q Query, p provider.Provider, opts ...provider.Option) (PageResult[T], error) {
	ast := q.AST()
	var items []T
	if err := q.Collect(ctx, p, &items, opts...); err != nil {
		return PageResult[T]{}, err
	}

	var total int64
	if q.withCount {
		offset := 0
		if ast.Offset != nil {
			offset = int(*ast.Offset)
		}
		partial := ast.Limit == nil || len(items) < int(*ast.Limit)
		if partial && (len(items) > 0 || offset == 0) {
			// A short page is the last one: the total is exact without a
			// second round-trip (most small/filtered lists skip COUNT).
			total = int64(offset + len(items))
		} else {
			n, err := q.WithoutCount().Count(ctx, p)
			if err != nil {
				return PageResult[T]{}, err
			}
			total = n
		}
	}

	page := 1
	perPage := 20
	if ast.Limit != nil {
		perPage = int(*ast.Limit)
	}
	if ast.Offset != nil && perPage > 0 {
		page = (int(*ast.Offset) / perPage) + 1
	}
	totalPages := 0
	if perPage > 0 && total > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}

	return PageResult[T]{
		Items:      items,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}
