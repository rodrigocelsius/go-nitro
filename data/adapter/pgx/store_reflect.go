package pgx

import (
	"context"
	"fmt"
	"reflect"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// ResourceRepository implements domain.ResourceDB — store from the model (no Bind[T]).
func (d *DB) ResourceRepository(model any, table string) domain.ResourceRepository {
	t := derefStructType(model)
	if t == nil {
		panic(fmt.Sprintf("pgx: ResourceRepository: model must be struct or *struct, got %T", model))
	}
	if table == "" {
		table = "items"
	}
	jsonToDB, dbTypes := buildColumnMetaType(t)
	return &reflectRepo{
		api:      NewDataAPIFromGateway(d.Gateway),
		table:    table,
		idCol:    "id",
		softCol:  domain.SoftDeleteColumn(t),
		typ:      t,
		jsonToDB: jsonToDB,
		dbTypes:  dbTypes,
	}
}

func derefStructType(model any) reflect.Type {
	if model == nil {
		return nil
	}
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	return t
}

func buildColumnMetaType(t reflect.Type) (jsonToDB map[string]string, dbTypes map[string]reflect.Type) {
	jsonToDB = map[string]string{}
	dbTypes = map[string]reflect.Type{}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return jsonToDB, dbTypes
	}
	walkDBFieldsType(t, func(f reflect.StructField) {
		db := dbTag(f)
		if db == "" {
			return
		}
		jsonName := f.Name
		if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
			if n := splitCSVFirst(tag); n != "" {
				jsonName = n
			}
		}
		jsonToDB[jsonName] = db
		dbTypes[db] = f.Type
	})
	return jsonToDB, dbTypes
}

func splitCSVFirst(tag string) string {
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			return tag[:i]
		}
	}
	return tag
}

type reflectRepo struct {
	api      *DataAPI
	table    string
	idCol    string
	softCol  string
	typ      reflect.Type
	jsonToDB map[string]string
	dbTypes  map[string]reflect.Type
}

func (r *reflectRepo) entity() field.Entity {
	return field.Entity{Name: r.table, SoftColumn: r.softCol}
}

func (r *reflectRepo) newPtr() any {
	return reflect.New(r.typ).Interface()
}

func (r *reflectRepo) FindByID(ctx context.Context, id any) (any, error) {
	q := query.From(r.entity()).Where(field.Col[any](r.table, r.idCol).Eq(id)).Limit(1)
	rows, err := r.api.Select(ctx, q.AST())
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	ptr := r.newPtr()
	if err := scan.MapToStruct(rows[0], ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

func (r *reflectRepo) List(ctx context.Context, page, perPage int) (*domain.ResourceListResult, error) {
	return r.ListQuery(ctx, domain.ListQuery{Page: page, PerPage: perPage})
}

func (r *reflectRepo) ListQuery(ctx context.Context, lq domain.ListQuery) (*domain.ResourceListResult, error) {
	page, per := lq.Page, lq.PerPage
	if page < 1 {
		page = 1
	}
	if per < 1 {
		per = 20
	}
	q := query.From(r.entity())
	for _, f := range lq.Filters {
		// Unknown fields are ignored (whitelist only) — matches
		// StoreRepo.ListQuery. Falling back to the raw f.Field as the
		// column name would let a caller reference any column not
		// covered by jsonToDB, bypassing the Filterable/Sortable
		// allow-list if it ever diverges from the model's tags.
		col, ok := r.jsonToDB[f.Field]
		if !ok {
			continue
		}
		val := coerceValue(r.dbTypes[col], f.Value)
		fld := field.Col[any](r.table, col)
		q = q.Where(filterPred(fld, f.Op, val, f.Value))
	}
	for _, s := range lq.Sort {
		col, ok := r.jsonToDB[s.Field]
		if !ok {
			continue
		}
		ord := field.Col[any](r.table, col)
		if s.Desc {
			q = q.OrderBy(ord.OrderDesc())
		} else {
			q = q.OrderBy(ord.OrderAsc())
		}
	}
	q = q.Paginate(uint64(page), uint64(per))

	total, err := q.WithoutCount().Count(ctx, r.api.Provider())
	if err != nil {
		return nil, err
	}
	rows, err := r.api.Select(ctx, q.AST())
	if err != nil {
		return nil, err
	}
	items := make([]any, 0, len(rows))
	for _, row := range rows {
		ptr := r.newPtr()
		if err := scan.MapToStruct(row, ptr); err != nil {
			return nil, err
		}
		items = append(items, ptr)
	}
	totalPages := 0
	if per > 0 && total > 0 {
		totalPages = int((total + int64(per) - 1) / int64(per))
	}
	return &domain.ResourceListResult{
		Items:      items,
		Total:      int(total),
		Page:       page,
		PerPage:    per,
		TotalPages: totalPages,
	}, nil
}

func (r *reflectRepo) Create(ctx context.Context, entity any) error {
	if err := r.ensureType(entity); err != nil {
		return err
	}
	domain.ApplyEntityDefaults(entity, domain.EntityDefaults{AutoID: true, AutoTimestamps: true}, true)
	cols, vals := structDBValues(entity, true)
	if len(cols) == 0 {
		return fmt.Errorf("pgx: Create: no columns")
	}
	created, err := r.createValues(ctx, cols, vals)
	if err != nil {
		return err
	}
	reflect.ValueOf(entity).Elem().Set(reflect.ValueOf(created).Elem())
	return nil
}

// UpdateFields persists only the named Go fields (+ UpdatedAt).
func (r *reflectRepo) UpdateFields(ctx context.Context, entity any, fields []string) error {
	if err := r.ensureType(entity); err != nil {
		return err
	}
	domain.ApplyEntityDefaults(entity, domain.EntityDefaults{AutoTimestamps: true}, false)
	id, sets := structIDAndSetsOnly(entity, r.table, r.idCol, fieldSet(fields))
	if id == nil {
		return fmt.Errorf("pgx: UpdateFields: missing id")
	}
	if len(sets) == 0 {
		return nil
	}
	updated, err := r.update(ctx, id, sets...)
	if err != nil {
		return err
	}
	reflect.ValueOf(entity).Elem().Set(reflect.ValueOf(updated).Elem())
	return nil
}

func (r *reflectRepo) Update(ctx context.Context, entity any) error {
	if err := r.ensureType(entity); err != nil {
		return err
	}
	domain.ApplyEntityDefaults(entity, domain.EntityDefaults{AutoTimestamps: true}, false)
	id, sets := structIDAndSets(entity, r.table, r.idCol)
	if id == nil {
		return fmt.Errorf("pgx: Update: missing id")
	}
	if len(sets) == 0 {
		return nil
	}
	updated, err := r.update(ctx, id, sets...)
	if err != nil {
		return err
	}
	reflect.ValueOf(entity).Elem().Set(reflect.ValueOf(updated).Elem())
	return nil
}

func (r *reflectRepo) Delete(ctx context.Context, id any) error {
	q := query.DeleteFrom(r.entity()).Where(field.Col[any](r.table, r.idCol).Eq(id))
	n, err := r.api.Delete(ctx, q.AST())
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *reflectRepo) ensureType(entity any) error {
	t := reflect.TypeOf(entity)
	if t.Kind() != reflect.Ptr || t.Elem() != r.typ {
		return fmt.Errorf("pgx: expected *%s, got %T", r.typ.String(), entity)
	}
	return nil
}

func (r *reflectRepo) createValues(ctx context.Context, columns []string, values []any) (any, error) {
	sets := make([]expr.Assignment, len(columns))
	for i, c := range columns {
		sets[i] = expr.Assignment{
			Field: expr.FieldRef{Table: r.table, Column: c},
			Value: values[i],
		}
	}
	return r.scanReturning(ctx, query.InsertInto(r.entity()).Values(sets...).AST())
}

func (r *reflectRepo) update(ctx context.Context, id any, sets ...expr.Assignment) (any, error) {
	q := query.Update(r.entity()).Set(sets...).Where(field.Col[any](r.table, r.idCol).Eq(id))
	return r.scanReturning(ctx, q.AST())
}

func (r *reflectRepo) scanReturning(ctx context.Context, ast expr.Query) (any, error) {
	res, err := r.api.Provider().Execute(ctx, ast)
	if err != nil {
		return nil, err
	}
	ptr := r.newPtr()
	if err := res.ScanOne(ptr); err != nil {
		if err == provider.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return ptr, nil
}
