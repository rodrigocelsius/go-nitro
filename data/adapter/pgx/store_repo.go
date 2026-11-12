package pgx

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// StoreRepo adapts Store[T] for nitro.Resource (TypedRepository + ListQuery).
type StoreRepo[T any] struct {
	store    *Store[T]
	jsonToDB map[string]string
	dbTypes  map[string]reflect.Type
}

// NewStoreRepo creates the typed adapter.
func NewStoreRepo[T any](store *Store[T]) *StoreRepo[T] {
	m, types := buildColumnMeta[T]()
	return &StoreRepo[T]{store: store, jsonToDB: m, dbTypes: types}
}

// AdaptStore returns ResourceRepository (+ QueryableLister via ListQuery).
func AdaptStore[T any](store *Store[T]) domain.ResourceRepository {
	return &storeRepoBridge[T]{inner: NewStoreRepo(store)}
}

// AsResourceRepository lets you pass *Store[T] directly to app.Resource (no AdaptStore).
func (s *Store[T]) AsResourceRepository() domain.ResourceRepository {
	return AdaptStore(s)
}

// storeRepoBridge exposes ResourceRepository + QueryableLister.
type storeRepoBridge[T any] struct {
	inner *StoreRepo[T]
}

func (b *storeRepoBridge[T]) FindByID(ctx context.Context, id any) (any, error) {
	return b.inner.FindByID(ctx, id)
}
func (b *storeRepoBridge[T]) List(ctx context.Context, page, perPage int) (*domain.ResourceListResult, error) {
	return b.inner.List(ctx, page, perPage)
}
func (b *storeRepoBridge[T]) ListQuery(ctx context.Context, q domain.ListQuery) (*domain.ResourceListResult, error) {
	return b.inner.ListQuery(ctx, q)
}
func (b *storeRepoBridge[T]) Create(ctx context.Context, entity any) error {
	t, ok := entity.(*T)
	if !ok {
		return fmt.Errorf("pgx: Create expects *T")
	}
	return b.inner.Create(ctx, t)
}
func (b *storeRepoBridge[T]) Update(ctx context.Context, entity any) error {
	t, ok := entity.(*T)
	if !ok {
		return fmt.Errorf("pgx: Update expects *T")
	}
	return b.inner.Update(ctx, t)
}
func (b *storeRepoBridge[T]) UpdateFields(ctx context.Context, entity any, fields []string) error {
	t, ok := entity.(*T)
	if !ok {
		return fmt.Errorf("pgx: UpdateFields expects *T")
	}
	return b.inner.UpdateFields(ctx, t, fields)
}
func (b *storeRepoBridge[T]) Delete(ctx context.Context, id any) error {
	return b.inner.Delete(ctx, id)
}

func (r *StoreRepo[T]) FindByID(ctx context.Context, id any) (*T, error) {
	return r.store.Get(ctx, id)
}

func (r *StoreRepo[T]) List(ctx context.Context, page, perPage int) (*domain.ResourceListResult, error) {
	return r.ListQuery(ctx, domain.ListQuery{Page: page, PerPage: perPage})
}

func (r *StoreRepo[T]) ListQuery(ctx context.Context, lq domain.ListQuery) (*domain.ResourceListResult, error) {
	page, per := lq.Page, lq.PerPage
	if page < 1 {
		page = 1
	}
	if per < 1 {
		per = 20
	}
	q := r.store.Query()
	for _, f := range lq.Filters {
		col, ok := r.jsonToDB[f.Field]
		if !ok {
			continue // unknown fields ignored (whitelist only)
		}
		val := coerceValue(r.dbTypes[col], f.Value)
		fld := field.Col[any](r.store.table, col)
		q = q.Where(filterPred(fld, f.Op, val, f.Value))
	}
	q, keys := orderForList(q, r.store.table, r.store.idCol, r.jsonToDB, r.dbTypes, lq)
	if lq.Keyset {
		// Cursor paging: seek past the previous page instead of OFFSET,
		// and no COUNT.
		pred, ok, err := keysetAfter(r.store.table, keys, r.dbTypes, lq.After)
		if err != nil {
			return nil, err
		}
		if ok {
			q = q.Where(pred)
		}
		var rows []T
		if err := q.Limit(uint64(per)).Collect(ctx, r.store.api.Provider(), &rows); err != nil {
			return nil, err
		}
		items := make([]any, len(rows))
		for i := range rows {
			items[i] = &rows[i]
		}
		return &domain.ResourceListResult{Items: items, PerPage: per}, nil
	}
	q = q.Paginate(uint64(page), uint64(per))
	pageRes, err := query.PaginateCollect[T](ctx, q, r.store.api.Provider())
	if err != nil {
		return nil, err
	}
	items := make([]any, len(pageRes.Items))
	for i := range pageRes.Items {
		items[i] = &pageRes.Items[i]
	}
	return &domain.ResourceListResult{
		Items:      items,
		Total:      int(pageRes.Total),
		Page:       pageRes.Page,
		PerPage:    pageRes.PerPage,
		TotalPages: pageRes.TotalPages,
	}, nil
}

func (r *StoreRepo[T]) Create(ctx context.Context, entity *T) error {
	domain.ApplyEntityDefaults(entity, domain.EntityDefaults{AutoID: true, AutoTimestamps: true}, true)
	cols, vals := structDBValues(entity, true)
	if len(cols) == 0 {
		return fmt.Errorf("pgx: Create: no columns")
	}
	created, err := r.store.CreateValues(ctx, cols, vals)
	if err != nil {
		return err
	}
	mergeDBColumns(entity, created)
	return nil
}

func (r *StoreRepo[T]) Update(ctx context.Context, entity *T) error {
	domain.ApplyEntityDefaults(entity, domain.EntityDefaults{AutoTimestamps: true}, false)
	id, sets := structIDAndSets(entity, r.store.table, r.store.idCol)
	if id == nil {
		return fmt.Errorf("pgx: Update: missing id")
	}
	if len(sets) == 0 {
		return nil
	}
	updated, err := r.store.Update(ctx, id, sets...)
	if err != nil {
		return err
	}
	mergeDBColumns(entity, updated)
	return nil
}

// UpdateFields persists only the named Go fields (plus UpdatedAt when the
// model has it) and reloads the row. See domain.ResourcePartialUpdater.
func (r *StoreRepo[T]) UpdateFields(ctx context.Context, entity *T, fields []string) error {
	domain.ApplyEntityDefaults(entity, domain.EntityDefaults{AutoTimestamps: true}, false)
	id, sets := structIDAndSetsOnly(entity, r.store.table, r.store.idCol, fieldSet(fields))
	if id == nil {
		return fmt.Errorf("pgx: UpdateFields: missing id")
	}
	if len(sets) == 0 {
		return nil
	}
	updated, err := r.store.Update(ctx, id, sets...)
	if err != nil {
		return err
	}
	mergeDBColumns(entity, updated)
	return nil
}

func (r *StoreRepo[T]) Delete(ctx context.Context, id any) error {
	return r.store.Delete(ctx, id)
}

func filterPred(fld field.Field[any], op string, val any, raw string) field.Pred {
	switch strings.ToLower(op) {
	case "", "eq":
		return fld.Eq(val)
	case "gte":
		return fld.Gte(val)
	case "lte":
		return fld.Lte(val)
	case "gt":
		return fld.Gt(val)
	case "lt":
		return fld.Lt(val)
	case "contains":
		return fld.Contains(raw)
	case "icontains":
		return fld.IContains(raw)
	case "jsoncontains":
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			v = raw
		}
		return fld.JSONContains(v)
	default:
		return fld.Eq(val)
	}
}

func buildColumnMeta[T any]() (jsonToDB map[string]string, dbTypes map[string]reflect.Type) {
	jsonToDB = map[string]string{}
	dbTypes = map[string]reflect.Type{}
	var zero T
	t := reflect.TypeOf(zero)
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
		jsonName := nitro.SnakeCase(f.Name)
		if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
			if n := strings.Split(tag, ",")[0]; n != "" {
				jsonName = n
			}
		}
		jsonToDB[jsonName] = db
		// Accept camelCase filter/sort keys that match the same column.
		if camel := nitro.CamelCase(jsonName); camel != "" && camel != jsonName {
			jsonToDB[camel] = db
		}
		if snake := nitro.SnakeCase(jsonName); snake != "" && snake != jsonName {
			jsonToDB[snake] = db
		}
		dbTypes[db] = f.Type
	})
	return jsonToDB, dbTypes
}

// dbTag resolves the column name for a field: explicit `db` tag wins;
// `db:"-"` excludes the field from persistence; when the tag is absent
// entirely, the column defaults to snake_case of the Go field name (same
// convention already used for reads in blinq/scan, and for JSON keys when
// the `json` tag is also absent — see adapters/fasthttp jsonnames).
func dbTag(f reflect.StructField) string {
	db := f.Tag.Get("db")
	if db == "-" {
		return ""
	}
	if db == "" {
		return nitro.SnakeCase(f.Name)
	}
	if j := strings.IndexByte(db, ','); j >= 0 {
		db = db[:j]
	}
	return db
}

// mergeDBColumns copies the column-backed fields of the row the database
// returned (RETURNING / reload) into entity, keeping fields that are not
// columns (`db:"-"`, e.g. a nested Items slice from the request body) so
// After* hooks and the HTTP response still see them.
func mergeDBColumns(entity, row any) {
	dv := reflect.ValueOf(entity)
	sv := reflect.ValueOf(row)
	if dv.Kind() != reflect.Pointer || sv.Kind() != reflect.Pointer || dv.IsNil() || sv.IsNil() {
		return
	}
	mergeDBColumnsValue(dv.Elem(), sv.Elem())
}

func mergeDBColumnsValue(dst, src reflect.Value) {
	t := dst.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		df, sf := dst.Field(i), src.Field(i)
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				if sf.IsNil() {
					continue
				}
				if df.IsNil() {
					df.Set(reflect.New(ft.Elem()))
				}
				df, sf, ft = df.Elem(), sf.Elem(), ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				mergeDBColumnsValue(df, sf)
			}
			continue
		}
		if dbTag(f) == "" || !df.CanSet() {
			continue
		}
		df.Set(sf)
	}
}

// walkDBFieldsType walks exported fields, including anonymous embeds.
func walkDBFieldsType(rt reflect.Type, fn func(f reflect.StructField)) {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				walkDBFieldsType(ft, fn)
			}
			continue
		}
		fn(f)
	}
}

func walkDBFields(rt reflect.Type, rv reflect.Value, fn func(f reflect.StructField, v reflect.Value)) {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		fv := rv.Field(i)
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				if fv.IsNil() {
					continue
				}
				fv = fv.Elem()
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				walkDBFields(ft, fv, fn)
			}
			continue
		}
		fn(f, fv)
	}
}

func coerceValue(t reflect.Type, s string) any {
	if t == nil {
		return s
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return s == "1" || strings.EqualFold(s, "true") || strings.EqualFold(s, "yes")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return s
		}
		return n
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return s
		}
		return n
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return s
		}
		return n
	default:
		if t == reflect.TypeOf(time.Time{}) {
			if ts, err := time.Parse(time.RFC3339, s); err == nil {
				return ts
			}
		}
		return coercePortable(t, s)
	}
}

func structDBValues(entity any, skipZeroID bool) (cols []string, vals []any) {
	rv := reflect.ValueOf(entity)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()
	walkDBFields(rt, rv, func(f reflect.StructField, fv reflect.Value) {
		db := dbTag(f)
		if db == "" {
			return
		}
		if skipZeroID && fv.IsZero() {
			if db == "id" || strings.EqualFold(f.Name, "ID") {
				return
			}
			if fv.Type().PkgPath() == "time" && fv.Type().Name() == "Time" {
				return
			}
		}
		cols = append(cols, db)
		vals = append(vals, fv.Interface())
	})
	return cols, vals
}

// insertColumns lists T's db columns for bulk insert.
// Excludes numeric IDs (BIGSERIAL). Includes string ID (UUID) and timestamps.
func insertColumns[T any](idCol string) []string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var cols []string
	walkDBFieldsType(t, func(f reflect.StructField) {
		db := dbTag(f)
		if db == "" {
			return
		}
		if db == idCol || (idCol == "id" && strings.EqualFold(f.Name, "ID")) {
			switch f.Type.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return // BIGSERIAL / identity
			}
		}
		cols = append(cols, db)
	})
	return cols
}

func valuesForColumns(entity any, cols []string) []any {
	rv := reflect.ValueOf(entity)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()
	byCol := make(map[string]any)
	walkDBFields(rt, rv, func(f reflect.StructField, fv reflect.Value) {
		db := dbTag(f)
		if db == "" {
			return
		}
		byCol[db] = fv.Interface()
	})
	vals := make([]any, len(cols))
	for i, c := range cols {
		vals[i] = byCol[c]
	}
	return vals
}

func fieldSet(fields []string) map[string]bool {
	m := make(map[string]bool, len(fields)+1)
	for _, f := range fields {
		m[f] = true
	}
	m["UpdatedAt"] = true
	return m
}

func structIDAndSets(entity any, table, idCol string) (id any, sets []expr.Assignment) {
	return structIDAndSetsOnly(entity, table, idCol, nil)
}

// structIDAndSetsOnly is structIDAndSets restricted to Go field names in only
// (nil = every column).
func structIDAndSetsOnly(entity any, table, idCol string, only map[string]bool) (id any, sets []expr.Assignment) {
	rv := reflect.ValueOf(entity)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()
	walkDBFields(rt, rv, func(f reflect.StructField, fv reflect.Value) {
		db := dbTag(f)
		if db == "" {
			return
		}
		if db == idCol || (idCol == "id" && strings.EqualFold(f.Name, "ID")) {
			id = fv.Interface()
			return
		}
		if only != nil && !only[f.Name] {
			return
		}
		sets = append(sets, expr.Assignment{
			Field: expr.FieldRef{Table: table, Column: db},
			Value: fv.Interface(),
		})
	})
	return id, sets
}

// keysetKey is one column of the list ordering.
type keysetKey struct {
	col   string
	desc  bool
	value int // index into ListCursor.Values, or -1 for the cursor ID
}

// orderForList applies the list ordering — the requested sort fields, then
// the primary key — and returns it as keyset columns. Without a unique
// tiebreaker the database may return rows in any order (and repeat or skip
// rows across pages when the sort column has duplicates), so the primary
// key is always the last sort key.
func orderForList(q query.Query, table, idCol string, jsonToDB map[string]string, dbTypes map[string]reflect.Type, lq domain.ListQuery) (query.Query, []keysetKey) {
	var keys []keysetKey
	sortedByID := false
	for i, s := range lq.Sort {
		col, ok := jsonToDB[s.Field]
		if !ok {
			continue
		}
		if col == idCol {
			sortedByID = true
		}
		ord := field.Col[any](table, col)
		if s.Desc {
			q = q.OrderBy(ord.OrderDesc())
		} else {
			q = q.OrderBy(ord.OrderAsc())
		}
		keys = append(keys, keysetKey{col: col, desc: s.Desc, value: i})
	}
	if _, hasID := dbTypes[idCol]; hasID && !sortedByID {
		q = q.OrderBy(field.Col[any](table, idCol).OrderAsc())
		keys = append(keys, keysetKey{col: idCol, value: -1})
	}
	return q, keys
}

// keysetAfter builds the predicate "rows after the cursor" for the ordering
// keys (k1, …, kn):
//
//	k1 > v1 OR (k1 = v1 AND k2 > v2) OR … OR (k1 = v1 AND … AND kn > vn)
//
// with < for descending keys. ok is false when there is no cursor (first
// page). Values are bound parameters coerced to the column types.
func keysetAfter(table string, keys []keysetKey, dbTypes map[string]reflect.Type, after *domain.ListCursor) (pred field.Pred, ok bool, err error) {
	if after == nil || len(keys) == 0 {
		return pred, false, nil
	}
	vals := make([]any, len(keys))
	for i, k := range keys {
		raw := after.ID
		if k.value >= 0 {
			if k.value >= len(after.Values) {
				return pred, false, domain.NewBadRequest("invalid cursor")
			}
			raw = after.Values[k.value]
		}
		vals[i] = coerceValue(dbTypes[k.col], raw)
	}
	for i, k := range keys {
		fld := field.Col[any](table, k.col)
		term := fld.Gt(vals[i])
		if k.desc {
			term = fld.Lt(vals[i])
		}
		for j := i - 1; j >= 0; j-- {
			term = field.Col[any](table, keys[j].col).Eq(vals[j]).And(term)
		}
		if i == 0 {
			pred = term
		} else {
			pred = pred.Or(term)
		}
	}
	return pred, true, nil
}
