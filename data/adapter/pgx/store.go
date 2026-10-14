package pgx

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// ErrNotFound indicates no row was found (HTTP 404 via domain.HTTPError).
var ErrNotFound = domain.ErrNotFound

// Store is the typed facade for standard CRUD via BLinq + DataAPI.
type Store[T any] struct {
	api     *DataAPI
	table   string
	idCol   string
	softCol string // "" = hard delete; "deleted_at" = soft when the model has DeletedAt
}

// NewStore creates a Store[T]. Default idCol: "id".
// If T has DeletedAt (or a deleted_at column), DELETE/SELECT use soft delete.
func NewStore[T any](api *DataAPI, table string, idCol ...string) *Store[T] {
	col := "id"
	if len(idCol) > 0 && idCol[0] != "" {
		col = idCol[0]
	}
	return &Store[T]{
		api:     api,
		table:   table,
		idCol:   col,
		softCol: softColumnOf[T](),
	}
}

// NewStoreFromPool creates a Store with DataAPI/Gateway (default observability).
func NewStoreFromPool[T any](pool *Pool, table string, opts ...gateway.Option) *Store[T] {
	return NewStore[T](NewDataAPI(pool, opts...), table)
}

// Gateway returns the inner Gateway (telemetry / cache).
func (s *Store[T]) Gateway() *gateway.Gateway { return s.api.Gateway() }

// API returns the underlying DataAPI.
func (s *Store[T]) API() *DataAPI { return s.api }

// Table returns the table name.
func (s *Store[T]) Table() string { return s.table }

// Entity is the store table metadata (SoftColumn if the model has soft delete).
func (s *Store[T]) Entity() field.Entity {
	return field.Entity{Name: s.table, SoftColumn: s.softCol}
}

// SoftDeleteEnabled reports whether T uses deleted_at.
func (s *Store[T]) SoftDeleteEnabled() bool {
	return s.softCol != ""
}

// Query starts a BLinq SELECT on the store table (soft-delete via Entity SoftColumn).
func (s *Store[T]) Query() query.Query {
	return query.From(s.Entity())
}

// Find runs Query and returns []T.
func (s *Store[T]) Find(ctx context.Context, q query.Query) ([]T, error) {
	rows, err := s.api.Select(ctx, q.AST())
	if err != nil {
		return nil, err
	}
	return scan.MapsToSlice[T](rows)
}

// FindOne returns the first record or ErrNotFound.
func (s *Store[T]) FindOne(ctx context.Context, q query.Query) (*T, error) {
	row, err := s.api.SelectOne(ctx, q.AST())
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var item T
	if err := scan.MapToStruct(row, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// Get looks up by ID (FindOne shortcut).
func (s *Store[T]) Get(ctx context.Context, id any) (*T, error) {
	return s.FindOne(ctx, s.Query().Where(field.Col[any](s.table, s.idCol).Eq(id)))
}

// GetBy finds the first record with column = value (soft-delete honored via Query).
//
//	d, err := store.GetBy(ctx, "phone", nitro.Digits(phone))
func (s *Store[T]) GetBy(ctx context.Context, column string, value any) (*T, error) {
	if column == "" {
		return nil, fmt.Errorf("pgx: GetBy requires column")
	}
	return s.FindOne(ctx, s.Query().Where(field.Col[any](s.table, column).Eq(value)).Limit(1))
}

// Create inserts fields and returns the row via RETURNING *.
func (s *Store[T]) Create(ctx context.Context, fields map[string]any) (*T, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("pgx: Create requires fields")
	}
	cols := make([]string, 0, len(fields))
	for k := range fields {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	vals := make([]any, len(cols))
	for i, c := range cols {
		vals[i] = fields[c]
	}
	return s.CreateValues(ctx, cols, vals)
}

// CreateOne inserts from *T (`db` tags).
// Fills UUID v7 + timestamps when empty (parity with Resource).
//
//	p, err := store.CreateOne(ctx, &Product{Name: "K", Price: 10, Active: true})
func (s *Store[T]) CreateOne(ctx context.Context, row *T) (*T, error) {
	domain.ApplyEntityDefaults(row, domain.EntityDefaults{AutoID: true, AutoTimestamps: true}, true)
	cols, vals := structDBValues(row, true)
	if len(cols) == 0 {
		return nil, fmt.Errorf("pgx: CreateOne: no columns")
	}
	created, err := s.CreateValues(ctx, cols, vals)
	if err != nil {
		return nil, err
	}
	*row = *created
	return created, nil
}

// CreateSets inserts with typed Assignments (same style as Update).
//
//	p, err := store.CreateSets(ctx,
//	    ProductModel.Name.Set("K"),
//	    ProductModel.Price.Set(10.0),
//	)
func (s *Store[T]) CreateSets(ctx context.Context, sets ...expr.Assignment) (*T, error) {
	if len(sets) == 0 {
		return nil, fmt.Errorf("pgx: CreateSets requires sets")
	}
	return s.scanReturning(ctx, query.InsertInto(s.Entity()).Values(sets...).AST())
}

// CreateManyOption configures CreateMany.
type CreateManyOption func(*createManyCfg)

type createManyCfg struct {
	chunk int
}

// WithChunk sets the multi-row INSERT batch size (0 = all at once).
func WithChunk(n int) CreateManyOption {
	return func(c *createManyCfg) { c.chunk = n }
}

// CreateMany inserts multiple rows and returns the created rows (RETURNING * on
// Postgres; multi-row INSERT + SELECT by id when the dialect has no RETURNING).
// Auto-chunks when the bind budget (Dialect.MaxPlaceholders) would be exceeded.
//
//	created, err := store.CreateMany(ctx, []Product{
//	    {Name: "A", Price: 10, Active: true},
//	    {Name: "B", Price: 20, Active: true},
//	}, pgx.WithChunk(500))
func (s *Store[T]) CreateMany(ctx context.Context, rows []T, opts ...CreateManyOption) ([]T, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	cfg := createManyCfg{}
	for _, o := range opts {
		if o != nil {
			o(&cfg)
		}
	}

	for i := range rows {
		domain.ApplyEntityDefaults(&rows[i], domain.EntityDefaults{AutoID: true, AutoTimestamps: true}, true)
	}

	cols := insertColumns[T](s.idCol)
	if len(cols) == 0 {
		return nil, fmt.Errorf("pgx: CreateMany: no columns")
	}

	if cfg.chunk <= 0 && s.api != nil && s.api.d != nil {
		if max := s.api.d.MaxPlaceholders(); max > 0 {
			per := max / len(cols)
			if per < 1 {
				per = 1
			}
			if len(rows) > per {
				cfg.chunk = per
			}
		}
	}

	assignRows := make([][]expr.Assignment, len(rows))
	for i := range rows {
		vals := valuesForColumns(&rows[i], cols)
		sets := make([]expr.Assignment, len(cols))
		for j, c := range cols {
			sets[j] = expr.Assignment{
				Field: expr.FieldRef{Table: s.table, Column: c},
				Value: vals[j],
			}
		}
		assignRows[i] = sets
	}

	if s.api != nil && s.api.d != nil && !s.api.d.ReturningSupported() {
		return s.createManyNoReturning(ctx, rows, assignRows, cfg.chunk)
	}

	q := query.InsertInto(s.Entity()).Rows(assignRows...)
	if cfg.chunk > 0 {
		return s.createManyChunked(ctx, q, cfg.chunk)
	}
	res, err := q.Execute(ctx, s.api.Provider())
	if err != nil {
		return nil, err
	}
	var out []T
	if err := res.ScanSlice(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// createManyNoReturning multi-row INSERTs (OmitReturning), then reloads by id.
// Requires client-side IDs (UUID); BIGSERIAL without RETURNING falls back to CreateOne.
func (s *Store[T]) createManyNoReturning(ctx context.Context, rows []T, assignRows [][]expr.Assignment, chunk int) ([]T, error) {
	ids := make([]any, 0, len(rows))
	for i := range rows {
		id, _ := structIDAndSets(&rows[i], s.table, s.idCol)
		if isEmptyID(id) {
			out := make([]T, 0, len(rows))
			for j := range rows {
				created, err := s.CreateOne(ctx, &rows[j])
				if err != nil {
					return out, err
				}
				out = append(out, *created)
			}
			return out, nil
		}
		ids = append(ids, id)
	}

	q := query.InsertInto(s.Entity()).Rows(assignRows...).OmitReturning()
	if chunk > 0 {
		ast := q.AST()
		mv := ast.MultiValues
		for i := 0; i < len(mv); i += chunk {
			end := i + chunk
			if end > len(mv) {
				end = len(mv)
			}
			part := ast
			part.MultiValues = mv[i:end]
			part.ChunkSize = 0
			if _, err := s.api.Provider().Execute(ctx, part); err != nil {
				return nil, err
			}
		}
	} else if _, err := q.Execute(ctx, s.api.Provider()); err != nil {
		return nil, err
	}

	idAny := make([]any, len(ids))
	copy(idAny, ids)
	return s.Find(ctx, s.Query().Where(field.Col[any](s.table, s.idCol).In(idAny...)))
}

func (s *Store[T]) createManyChunked(ctx context.Context, q query.Query, chunk int) ([]T, error) {
	ast := q.AST()
	rows := ast.MultiValues
	out := make([]T, 0, len(rows))
	for i := 0; i < len(rows); i += chunk {
		end := i + chunk
		if end > len(rows) {
			end = len(rows)
		}
		part := ast
		part.MultiValues = rows[i:end]
		part.ChunkSize = 0
		res, err := s.api.Provider().Execute(ctx, part)
		if err != nil {
			return nil, err
		}
		var batch []T
		if err := res.ScanSlice(&batch); err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

// CreateValues inserts explicit columns/values and returns RETURNING *.
func (s *Store[T]) CreateValues(ctx context.Context, columns []string, values []any) (*T, error) {
	if len(columns) == 0 || len(columns) != len(values) {
		return nil, fmt.Errorf("pgx: CreateValues columns/values mismatch")
	}
	sets := make([]expr.Assignment, len(columns))
	for i, c := range columns {
		sets[i] = expr.Assignment{
			Field: expr.FieldRef{Table: s.table, Column: c},
			Value: values[i],
		}
	}
	return s.scanReturning(ctx, query.InsertInto(s.Entity()).Values(sets...).AST())
}

// UpdateWhere updates every row matching filter (typically store.Query().Where(...)).
// Skips RETURNING so millions of rows are not loaded. Returns rows affected.
//
//	n, err := store.UpdateWhere(ctx,
//	    store.Query().Where(modelgen.IP4Blocks.CityName.Eq("São Paulo")),
//	    modelgen.IP4Blocks.Lat.Set(-23.55),
//	    modelgen.IP4Blocks.Lng.Set(-46.63),
//	)
func (s *Store[T]) UpdateWhere(ctx context.Context, filter query.Query, sets ...expr.Assignment) (int64, error) {
	if len(sets) == 0 {
		return 0, fmt.Errorf("pgx: UpdateWhere requires sets")
	}
	q := query.Update(s.Entity()).Set(sets...).OmitReturning()
	if w := filter.AST().Where; w != nil {
		q = q.Where(field.PredFrom(w))
	}
	res, err := q.Execute(ctx, s.api.Provider())
	if err != nil {
		return 0, err
	}
	n, err := res.Count()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Update updates by ID with Assignments and returns the row.
func (s *Store[T]) Update(ctx context.Context, id any, sets ...expr.Assignment) (*T, error) {
	if len(sets) == 0 {
		return nil, fmt.Errorf("pgx: Update requires sets")
	}
	q := query.Update(s.Entity()).
		Set(sets...).
		Where(field.Col[any](s.table, s.idCol).Eq(id))
	item, err := s.scanReturning(ctx, q.AST())
	if err != nil {
		return nil, err
	}
	return item, nil
}

// Delete removes by ID (soft UPDATE deleted_at if the model has DeletedAt).
func (s *Store[T]) Delete(ctx context.Context, id any) error {
	q := query.DeleteFrom(s.Entity()).Where(field.Col[any](s.table, s.idCol).Eq(id))
	n, err := s.api.Delete(ctx, q.AST())
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Count counts records from Query.
func (s *Store[T]) Count(ctx context.Context, q query.Query) (int, error) {
	n, err := s.api.CountQuery(ctx, q)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *Store[T]) scanReturning(ctx context.Context, ast expr.Query) (*T, error) {
	if s.api != nil && s.api.d != nil && !s.api.d.ReturningSupported() {
		return s.execThenReload(ctx, ast)
	}
	res, err := s.api.Provider().Execute(ctx, ast)
	if err != nil {
		return nil, err
	}
	var item T
	if err := res.ScanOne(&item); err != nil {
		if errors.Is(err, provider.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

// execThenReload runs INSERT/UPDATE without RETURNING, then SELECTs by id.
func (s *Store[T]) execThenReload(ctx context.Context, ast expr.Query) (*T, error) {
	id := idFromAST(ast, s.idCol)
	if _, err := s.api.Provider().Execute(ctx, ast); err != nil {
		return nil, err
	}
	if id == nil || isEmptyID(id) {
		return nil, fmt.Errorf("pgx: missing %s after write (dialect %s has no RETURNING; set UUID/id on insert)", s.idCol, s.api.d.Name())
	}
	return s.Get(ctx, id)
}
