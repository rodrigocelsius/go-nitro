package repository

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// BaseRepository is the default Repository implementation — a facade over pgx.Store[T] (BLinq).
type BaseRepository[T any] struct {
	store *pgxadapter.Store[T]
	table string
	idCol string
}

// NewBaseRepository creates a BaseRepository over DataAPI/Gateway + Store.
func NewBaseRepository[T any](gw *gateway.Gateway, table, idCol string) *BaseRepository[T] {
	if idCol == "" {
		idCol = "id"
	}
	api := pgxadapter.NewDataAPIFromGateway(gw)
	return &BaseRepository[T]{
		store: pgxadapter.NewStore[T](api, table, idCol),
		table: table,
		idCol: idCol,
	}
}

// Store exposes the underlying Store[T] (advanced BLinq queries).
func (r *BaseRepository[T]) Store() *pgxadapter.Store[T] { return r.store }

// FindByID finds a record by ID.
func (r *BaseRepository[T]) FindByID(ctx context.Context, id any) (*T, error) {
	item, err := r.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("repository: record not found: %w", err)
	}
	return item, nil
}

// Find runs a query and returns multiple records.
func (r *BaseRepository[T]) Find(ctx context.Context, q query.Query) ([]T, error) {
	return r.store.Find(ctx, q)
}

// FindOne runs a query and returns the first record.
func (r *BaseRepository[T]) FindOne(ctx context.Context, q query.Query) (*T, error) {
	return r.store.FindOne(ctx, q)
}

// Count returns the record count.
func (r *BaseRepository[T]) Count(ctx context.Context, q query.Query) (int, error) {
	return r.store.Count(ctx, q)
}

// List returns a paginated list.
func (r *BaseRepository[T]) List(ctx context.Context, q query.Query) (*query.PageResult[T], error) {
	if !q.WithCountEnabled() {
		q = q.WithCount()
	}
	page, err := query.PaginateCollect[T](ctx, q, r.store.API().Provider())
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// Create inserts a new record (exported fields; `db` tags preferred).
func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	cols, vals := entityDBValues(entity, true)
	if len(cols) == 0 {
		return fmt.Errorf("repository: Create: no columns")
	}
	created, err := r.store.CreateValues(ctx, cols, vals)
	if err != nil {
		return err
	}
	*entity = *created
	return nil
}

// Update updates an existing record.
func (r *BaseRepository[T]) Update(ctx context.Context, entity *T) error {
	id, sets := entityIDAndSets(entity, r.table, r.idCol)
	if id == nil {
		return fmt.Errorf("repository: cannot update without ID")
	}
	if len(sets) == 0 {
		return nil
	}
	updated, err := r.store.Update(ctx, id, sets...)
	if err != nil {
		return err
	}
	*entity = *updated
	return nil
}

// Delete removes a record by ID.
func (r *BaseRepository[T]) Delete(ctx context.Context, id any) error {
	return r.store.Delete(ctx, id)
}

func entityDBValues(entity any, skipZeroID bool) (cols []string, vals []any) {
	rv := reflect.ValueOf(entity)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue
		}
		db := columnDB(f)
		fv := rv.Field(i)
		if skipZeroID && (db == "id" || strings.EqualFold(f.Name, "ID")) && fv.IsZero() {
			continue
		}
		cols = append(cols, db)
		vals = append(vals, fv.Interface())
	}
	return cols, vals
}

func entityIDAndSets(entity any, table, idCol string) (id any, sets []expr.Assignment) {
	rv := reflect.ValueOf(entity)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue
		}
		db := columnDB(f)
		fv := rv.Field(i)
		if db == idCol || (idCol == "id" && strings.EqualFold(f.Name, "ID")) {
			id = fv.Interface()
			continue
		}
		sets = append(sets, expr.Assignment{
			Field: expr.FieldRef{Table: table, Column: db},
			Value: fv.Interface(),
		})
	}
	return id, sets
}

func columnDB(f reflect.StructField) string {
	db := f.Tag.Get("db")
	if db != "" && db != "-" {
		if j := strings.IndexByte(db, ','); j >= 0 {
			db = db[:j]
		}
		if db != "" {
			return db
		}
	}
	return scan.ToSnakeCase(f.Name)
}
