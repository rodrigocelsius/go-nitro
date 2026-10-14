package nitro

import (
	"context"
	"reflect"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// MemoryRepository is an in-memory TypedRepository[T].
// If T has a DeletedAt field (*time.Time, time.Time, or sql.NullTime),
// Delete performs soft delete; FindByID/List ignore soft-deleted rows.
//
// Entities are cloned on the repository boundary (Create/Update store copies;
// FindByID/List hand out copies), so callers mutating a returned entity never
// race with readers or corrupt stored state. The copy is shallow: pointer and
// slice fields inside T remain shared between clones.
type MemoryRepository[T any] struct {
	items *SafeMap[string, *T]
	soft  bool
}

// NewMemoryRepo creates a typed in-memory repository.
//
//	repo := nitro.NewMemoryRepo[Todo]()
//	nitro.Resource(app, "/todos", repo)
func NewMemoryRepo[T any]() *MemoryRepository[T] {
	return &MemoryRepository[T]{
		items: NewSafeMap[string, *T](),
		soft:  domain.HasSoftDelete[T](),
	}
}

// MemoryRepo returns an AdaptRepo-wrapped in-memory store for app.Resource / CacheRepo.
//
//	app.Resource("/todos", &Todo{}, nitro.CacheRepo(nitro.MemoryRepo[Todo](), time.Minute))
//	nitro.Resource(app, "/todos", nitro.NewMemoryRepo[Todo](), &nitro.ResourceConfig{CacheTTL: time.Minute})
func MemoryRepo[T any]() domain.ResourceRepository {
	return AdaptRepo(NewMemoryRepo[T]())
}

// SoftDeleteEnabled reports whether soft delete is active for T.
func (r *MemoryRepository[T]) SoftDeleteEnabled() bool {
	return r.soft
}

func (r *MemoryRepository[T]) FindByID(_ context.Context, id any) (*T, error) {
	t, ok := r.items.Get(IDKey(id))
	if !ok || isSoftDeleted(t) {
		return nil, NotFound("not found")
	}
	return cloneEntity(t), nil
}

func (r *MemoryRepository[T]) List(_ context.Context, page, perPage int) (*ResourceListResult, error) {
	all := r.items.Values()
	if r.soft {
		alive := make([]*T, 0, len(all))
		for _, t := range all {
			if !isSoftDeleted(t) {
				alive = append(alive, t)
			}
		}
		all = alive
	}
	copies := make([]*T, len(all))
	for i, t := range all {
		copies[i] = cloneEntity(t)
	}
	return ListResult(copies, page, perPage), nil
}

func (r *MemoryRepository[T]) Create(_ context.Context, entity *T) error {
	id := entityID(entity)
	if id == "" {
		id = NewUUIDString()
		setEntityID(entity, id)
	}
	// Never overwrite an existing row (or a soft-deleted one) on Create — a
	// duplicate primary key is a conflict, as it would be in SQL.
	if !r.items.SetIfAbsent(IDKey(id), cloneEntity(entity)) {
		return Conflict("already exists")
	}
	return nil
}

func (r *MemoryRepository[T]) Update(_ context.Context, entity *T) error {
	id := entityID(entity)
	key := IDKey(id)
	existing, ok := r.items.Get(key)
	if !ok || isSoftDeleted(existing) {
		return NotFound("not found")
	}
	stored := cloneEntity(entity)
	preserveCreatedAt(existing, stored)
	if r.soft {
		preserveDeletedAt(existing, stored)
	}
	r.items.Set(key, stored)
	return nil
}

func (r *MemoryRepository[T]) Delete(_ context.Context, id any) error {
	key := IDKey(id)
	existing, ok := r.items.Get(key)
	if !ok || isSoftDeleted(existing) {
		return NotFound("not found")
	}
	if r.soft {
		marked := cloneEntity(existing)
		setDeletedAt(marked, time.Now())
		r.items.Set(key, marked)
		return nil
	}
	r.items.Delete(key)
	return nil
}

// Len returns the number of stored entities (including soft-deleted).
func (r *MemoryRepository[T]) Len() int {
	return r.items.Len()
}

// cloneEntity returns a shallow copy of *T so stored entries are never
// aliased by callers (and vice versa). Pointer/slice fields inside T remain
// shared between clones.
func cloneEntity[T any](entity *T) *T {
	if entity == nil {
		return nil
	}
	clone := new(T)
	reflect.ValueOf(clone).Elem().Set(reflect.ValueOf(entity).Elem())
	return clone
}

// EntityID reads the "ID" field of entity (or *entity) as a string — works
// for string IDs and nitro.UUID. Exported for adapters comparing an entity
// against a URL :id param outside app.Resource (e.g. sqladapter.Related).
func EntityID(entity any) string {
	return entityID(entity)
}

func entityID(entity any) string {
	rv := reflect.ValueOf(entity)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	f := domain.FieldByName(rv, "ID")
	if !f.IsValid() {
		return ""
	}
	switch f.Kind() {
	case reflect.String:
		return f.String()
	default:
		if f.Type() == reflect.TypeOf(UUID{}) {
			u := f.Interface().(UUID)
			if u.IsZero() {
				return ""
			}
			return u.String()
		}
		if stringer, ok := f.Interface().(interface{ String() string }); ok {
			return stringer.String()
		}
	}
	return ""
}

func setEntityID(entity any, id string) {
	rv := reflect.ValueOf(entity)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	f := domain.FieldByName(rv, "ID")
	if !f.IsValid() || !f.CanSet() {
		return
	}
	switch f.Kind() {
	case reflect.String:
		f.SetString(id)
	default:
		if f.Type() == reflect.TypeOf(UUID{}) {
			u, err := ParseUUID(id)
			if err != nil {
				return
			}
			f.Set(reflect.ValueOf(u))
		}
	}
}

func isSoftDeleted(entity any) bool {
	rv := reflect.ValueOf(entity)
	if !rv.IsValid() {
		return false
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	f := domain.FieldByName(rv, "DeletedAt")
	if !f.IsValid() {
		return false
	}
	switch f.Kind() {
	case reflect.Ptr:
		if f.IsNil() {
			return false
		}
		return !f.Elem().IsZero()
	case reflect.Struct:
		if f.Type() == reflect.TypeOf(time.Time{}) {
			return !f.Interface().(time.Time).IsZero()
		}
		if f.Type() == reflect.TypeOf(DateTime{}) {
			return !f.Interface().(DateTime).IsZero()
		}
		if vf := domain.FieldByName(f, "Valid"); vf.IsValid() && vf.Kind() == reflect.Bool {
			return vf.Bool()
		}
	}
	return false
}

func setDeletedAt(entity any, t time.Time) {
	rv := reflect.ValueOf(entity)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	f := domain.FieldByName(rv, "DeletedAt")
	if !f.IsValid() || !f.CanSet() {
		return
	}
	switch f.Kind() {
	case reflect.Ptr:
		if f.Type().Elem() == reflect.TypeOf(time.Time{}) {
			f.Set(reflect.ValueOf(&t))
		} else if f.Type().Elem() == reflect.TypeOf(DateTime{}) {
			dt := DateTimeFrom(t)
			f.Set(reflect.ValueOf(&dt))
		}
	case reflect.Struct:
		if f.Type() == reflect.TypeOf(time.Time{}) {
			f.Set(reflect.ValueOf(t))
		} else if f.Type() == reflect.TypeOf(DateTime{}) {
			f.Set(reflect.ValueOf(DateTimeFrom(t)))
		} else if vf := domain.FieldByName(f, "Valid"); vf.IsValid() && vf.CanSet() {
			if tf := domain.FieldByName(f, "Time"); tf.IsValid() && tf.CanSet() {
				tf.Set(reflect.ValueOf(t))
				vf.SetBool(true)
			}
		}
	}
}

func preserveCreatedAt(from, to any) {
	copyTimeField(from, to, "CreatedAt")
}

func preserveDeletedAt(from, to any) {
	rvFrom := reflect.ValueOf(from)
	rvTo := reflect.ValueOf(to)
	if rvFrom.Kind() == reflect.Ptr {
		rvFrom = rvFrom.Elem()
	}
	if rvTo.Kind() == reflect.Ptr {
		rvTo = rvTo.Elem()
	}
	src := domain.FieldByName(rvFrom, "DeletedAt")
	dst := domain.FieldByName(rvTo, "DeletedAt")
	if !src.IsValid() || !dst.IsValid() || !dst.CanSet() {
		return
	}
	dst.Set(src)
}

func copyTimeField(from, to any, name string) {
	rvFrom := reflect.ValueOf(from)
	rvTo := reflect.ValueOf(to)
	if rvFrom.Kind() == reflect.Ptr {
		rvFrom = rvFrom.Elem()
	}
	if rvTo.Kind() == reflect.Ptr {
		rvTo = rvTo.Elem()
	}
	src := domain.FieldByName(rvFrom, name)
	dst := domain.FieldByName(rvTo, name)
	if !src.IsValid() || !dst.IsValid() || !dst.CanSet() {
		return
	}
	if src.Type() == dst.Type() {
		dst.Set(src)
	}
}

// AsResourceRepository lets a memory repository be passed straight to
// app.Resource (no AdaptRepo): app.Resource(&Todo{}, nitro.NewMemoryRepo[Todo]()).
func (r *MemoryRepository[T]) AsResourceRepository() domain.ResourceRepository {
	return AdaptRepo[T](r)
}
