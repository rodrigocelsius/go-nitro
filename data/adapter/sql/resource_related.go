package sqladapter

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// RecalcFunc mutates the parent and its children before every write that
// touches children (create, add a child, or PATCH on a child). It has full
// read/write access to both structs, so it can update as many parent fields
// as needed (Total, ItemCount, UpdatedAt, ...), not just a single sum. It
// runs inside the same db.Tx as the write — no HTTP route is created for it.
type RecalcFunc[T any, C any] func(parent *T, children []C)

// Related registers a parent Resource (T) with a dependent child table (C),
// atomically. The FK column on C and the child route segment follow the same
// naming convention as app.Resource (snake_case(T)+"_id" / table of C) — no
// strings to configure.
//
// T must declare exactly one field of type []C (any name, `db:"-"`) — the
// nested input accepted by POST /{parent}.
//
//	POST   /{parent}                          — creates T + []C, one Tx, runs recalc
//	GET    /{parent}/:id/{children}           — lists child rows for that parent
//	POST   /{parent}/:id/{children}           — adds one child to an existing parent, one Tx, runs recalc
//	PATCH  /{parent}/:id/{children}/:childID  — updates one child, reruns recalc, one Tx
func Related[T any, C any](app *nitro.App, db *DB, recalc RecalcFunc[T, C], cfg *nitro.ResourceConfig) *nitro.App {
	if db == nil {
		panic("sqladapter: Related requires a non-nil db")
	}
	if recalc == nil {
		panic("sqladapter: Related requires a non-nil RecalcFunc")
	}
	meta := buildRelatedMeta[T, C]()

	cfg2 := &nitro.ResourceConfig{}
	if cfg != nil {
		cp := *cfg
		cfg2 = &cp
	}
	replace := make(map[string]nitro.Handler, len(cfg2.Replace)+1)
	for k, v := range cfg2.Replace {
		replace[k] = v
	}
	replace[nitro.ActionCreate] = relatedCreateHandler(db, meta, recalc)
	cfg2.Replace = replace

	childPath := "/:id/" + meta.childTable
	cfg2.ExtraRoutes = append(append([]nitro.ResourceExtraRoute(nil), cfg2.ExtraRoutes...),
		nitro.ResourceExtraRoute{Method: "GET", Path: childPath, Handler: relatedListChildrenHandler[C](db, meta)},
		nitro.ResourceExtraRoute{Method: "POST", Path: childPath, Handler: relatedAddChildHandler(db, meta, recalc)},
		nitro.ResourceExtraRoute{Method: "PATCH", Path: childPath + "/:childID", Handler: relatedUpdateChildHandler(db, meta, recalc)},
	)

	store := StoreOf[T](db, meta.table)
	return app.Resource(new(T), store, cfg2)
}

type relatedMeta struct {
	table      string
	childTable string
	childField string // T field of type []C — nested body on create
	fkField    string // C field carrying the FK to T
	fkColumn   string
}

func buildRelatedMeta[T any, C any]() relatedMeta {
	pt := reflect.TypeOf(*new(T))
	ct := reflect.TypeOf(*new(C))

	childField, ok := sliceFieldOf(pt, ct)
	if !ok {
		panic(fmt.Sprintf(
			"sqladapter: Related[%s, %s]: %s must declare a field of type []%s (e.g. `Items []%s `json:\"items\" db:\"-\"``) for the nested create body",
			pt.Name(), ct.Name(), pt.Name(), ct.Name(), ct.Name(),
		))
	}

	fkColumn := nitro.SnakeCase(pt.Name()) + "_id"
	fkField, ok := fieldWithDBTag(ct, fkColumn)
	if !ok {
		panic(fmt.Sprintf(
			"sqladapter: Related[%s, %s]: %s has no field with `db:\"%s\"` (expected FK to %s)",
			pt.Name(), ct.Name(), ct.Name(), fkColumn, pt.Name(),
		))
	}

	return relatedMeta{
		table:      nitro.ResourcePath(new(T)),
		childTable: nitro.ResourcePath(new(C)),
		childField: childField,
		fkField:    fkField,
		fkColumn:   fkColumn,
	}
}

// sliceFieldOf finds the field on t (exported, top-level) of type []elem.
func sliceFieldOf(t, elem reflect.Type) (string, bool) {
	want := reflect.SliceOf(elem)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == want {
			return f.Name, true
		}
	}
	return "", false
}

// fieldWithDBTag finds the (possibly embedded) field whose `db:"..."` tag equals col.
func fieldWithDBTag(t reflect.Type, col string) (string, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if name, ok := fieldWithDBTag(ft, col); ok {
					return name, true
				}
			}
			continue
		}
		if f.PkgPath != "" {
			continue
		}
		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			tag = tag[:idx]
		}
		if tag == col {
			return f.Name, true
		}
	}
	return "", false
}

func relatedCreateHandler[T any, C any](db *DB, meta relatedMeta, recalc RecalcFunc[T, C]) nitro.Handler {
	return func(c *nitro.Ctx) error {
		parent, err := nitro.Parse[T](c)
		if err != nil {
			return err
		}
		children := childSliceOf[C](&parent, meta.childField)
		recalc(&parent, children)

		ctx := c.Context()
		if err := db.Tx(ctx, func(tx *DB) error {
			if err := repoFor[T](tx, meta.table).Create(ctx, &parent); err != nil {
				return err
			}
			if len(children) == 0 {
				return nil
			}
			setChildFK(children, meta.fkField, parentIDOf(&parent))
			childRepo := repoFor[C](tx, meta.childTable)
			for i := range children {
				if err := childRepo.Create(ctx, &children[i]); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}

		setChildSlice(&parent, meta.childField, children)
		return c.Created(parent)
	}
}

func relatedListChildrenHandler[C any](db *DB, meta relatedMeta) nitro.Handler {
	return func(c *nitro.Ctx) error {
		id := c.Param("id")
		if id == "" {
			return nitro.BadRequest("missing id parameter")
		}
		store := StoreOf[C](db, meta.childTable)
		items, err := store.Find(c.Context(), store.Query().Where(field.Col[any](meta.childTable, meta.fkColumn).Eq(id)))
		if err != nil {
			return err
		}
		return c.OK(items)
	}
}

func relatedAddChildHandler[T any, C any](db *DB, meta relatedMeta, recalc RecalcFunc[T, C]) nitro.Handler {
	return func(c *nitro.Ctx) error {
		id := c.Param("id")
		if id == "" {
			return nitro.BadRequest("missing id parameter")
		}
		child, err := nitro.Parse[C](c)
		if err != nil {
			return err
		}

		ctx := c.Context()
		var saved C
		if err := db.Tx(ctx, func(tx *DB) error {
			parent, items, err := loadParentAndChildren[T, C](ctx, tx, meta, id)
			if err != nil {
				return err
			}
			setFieldValue(&child, meta.fkField, parentIDOf(parent))
			items = append(items, child)
			recalc(parent, items)

			if err := repoFor[C](tx, meta.childTable).Create(ctx, &items[len(items)-1]); err != nil {
				return err
			}
			saved = items[len(items)-1]
			return repoFor[T](tx, meta.table).Update(ctx, parent)
		}); err != nil {
			return err
		}
		return c.Created(saved)
	}
}

func relatedUpdateChildHandler[T any, C any](db *DB, meta relatedMeta, recalc RecalcFunc[T, C]) nitro.Handler {
	return func(c *nitro.Ctx) error {
		id := c.Param("id")
		childID := c.Param("childID")
		if id == "" || childID == "" {
			return nitro.BadRequest("missing id parameter")
		}

		ctx := c.Context()
		var updated C
		if err := db.Tx(ctx, func(tx *DB) error {
			parent, items, err := loadParentAndChildren[T, C](ctx, tx, meta, id)
			if err != nil {
				return err
			}
			idx := indexByID(items, childID)
			if idx < 0 {
				return nitro.NotFound("item not found")
			}
			if err := nitro.PatchParse(c, &items[idx]); err != nil {
				return err
			}
			recalc(parent, items)

			if err := repoFor[C](tx, meta.childTable).Update(ctx, &items[idx]); err != nil {
				return err
			}
			if err := repoFor[T](tx, meta.table).Update(ctx, parent); err != nil {
				return err
			}
			updated = items[idx]
			return nil
		}); err != nil {
			return err
		}
		return c.OK(updated)
	}
}

func loadParentAndChildren[T any, C any](ctx context.Context, tx *DB, meta relatedMeta, id string) (*T, []C, error) {
	parent, err := repoFor[T](tx, meta.table).FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if parent == nil {
		return nil, nil, nitro.NotFound("resource not found")
	}
	store := StoreOf[C](tx, meta.childTable)
	items, err := store.Find(ctx, store.Query().Where(field.Col[any](meta.childTable, meta.fkColumn).Eq(id)))
	if err != nil {
		return nil, nil, err
	}
	return parent, items, nil
}

func childSliceOf[C any](parent any, fieldName string) []C {
	v := reflect.ValueOf(parent).Elem().FieldByName(fieldName)
	if !v.IsValid() || v.IsNil() {
		return nil
	}
	out, _ := v.Interface().([]C)
	return out
}

func setChildSlice[C any](parent any, fieldName string, children []C) {
	v := reflect.ValueOf(parent).Elem().FieldByName(fieldName)
	if v.IsValid() && v.CanSet() {
		v.Set(reflect.ValueOf(children))
	}
}

func setChildFK[C any](children []C, fkField string, parentID any) {
	for i := range children {
		setFieldValue(&children[i], fkField, parentID)
	}
}

func setFieldValue(item any, fieldName string, value any) {
	v := reflect.ValueOf(item).Elem().FieldByName(fieldName)
	if !v.CanSet() {
		return
	}
	iv := reflect.ValueOf(value)
	if iv.Type() != v.Type() && iv.Type().ConvertibleTo(v.Type()) {
		iv = iv.Convert(v.Type())
	}
	v.Set(iv)
}

func parentIDOf(parent any) any {
	return reflect.ValueOf(parent).Elem().FieldByName("ID").Interface()
}

func indexByID[C any](items []C, id string) int {
	for i := range items {
		if nitro.EntityID(&items[i]) == id {
			return i
		}
	}
	return -1
}
