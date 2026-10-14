package nitro

import (
	"context"
	"net/http"
	"reflect"
	"strconv"
	"sync"
)

// Empty is the In/Out type for typed handlers with no request body or no
// response body (a nil/Empty result is sent as 204 No Content).
type Empty struct{}

// Handle adapts a typed function to a Handler. The input is bound from the
// JSON/form body or the query string (same rules as Parse); path
// parameters are filled first into fields tagged `param:"name"`; the result is
// validated (`validate:` tags). The returned value is sent as JSON.
//
//	type GetUser struct {
//	    ID int64 // bound from ":id" by convention (override: `param:"user_id"`, opt out: `param:"-"`)
//	}
//	app.Get("/users/:id", nitro.Handle(func(ctx context.Context, in *GetUser) (*User, error) {
//	    return users.Find(ctx, in.ID)
//	}))
//
// status optionally overrides the success code (default 200; 201 is a good
// choice for creates). Errors returned by fn follow the normal error mapping
// (sentinels, NotFound, validation → 422, …).
func Handle[In, Out any](fn func(ctx context.Context, in *In) (Out, error), status ...int) Handler {
	ok := http.StatusOK
	if len(status) > 0 && status[0] > 0 {
		ok = status[0]
	}
	var zeroIn In
	bind := reflect.TypeOf(zeroIn) != reflect.TypeOf(Empty{})
	return func(c *Ctx) error {
		var in In
		if bind {
			// Path params first so `validate:"required"` sees them.
			if err := bindPathParams(c, &in); err != nil {
				return err
			}
			if err := c.BodyParser(&in); err != nil {
				return err
			}
		}
		out, err := fn(c.RequestContext(), &in)
		if err != nil {
			return err
		}
		if isEmptyResult(out) {
			return c.NoContent()
		}
		return c.JSON(ok, out)
	}
}

func isEmptyResult(v any) bool {
	if v == nil {
		return true
	}
	if _, ok := v.(Empty); ok {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice:
		if rv.IsNil() {
			return true
		}
		if rv.Kind() == reflect.Pointer {
			_, isEmpty := rv.Elem().Interface().(Empty)
			return isEmpty
		}
	}
	return false
}

type paramField struct {
	index []int
}

var paramFieldsCache sync.Map // reflect.Type -> map[string]paramField

// paramFieldsOf maps route-parameter names to scalar fields of t.
// Convention: the parameter name is the field's JSON name (or snake_case of
// the Go name), so `ID int64` receives ":id" with no tag. An explicit
// `param:"name"` tag overrides it and `param:"-"` opts the field out.
func paramFieldsOf(t reflect.Type) map[string]paramField {
	if v, ok := paramFieldsCache.Load(t); ok {
		return v.(map[string]paramField)
	}
	out := map[string]paramField{}
	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" || f.Anonymous {
				continue
			}
			switch f.Type.Kind() {
			case reflect.String, reflect.Bool,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float32, reflect.Float64:
			default:
				continue
			}
			name := f.Tag.Get("param")
			if name == "-" {
				continue
			}
			if name == "" {
				name = columnJSONName(f)
			}
			out[name] = paramField{index: f.Index}
		}
	}
	paramFieldsCache.Store(t, out)
	return out
}

// bindPathParams fills fields from the route parameters of the request.
func bindPathParams(c *Ctx, in any) error {
	if c.ParamCount == 0 {
		return nil
	}
	rv := reflect.ValueOf(in).Elem()
	fields := paramFieldsOf(rv.Type())
	if len(fields) == 0 {
		return nil
	}
	for i := 0; i < c.ParamCount; i++ {
		pf, ok := fields[c.ParamNames[i]]
		if !ok {
			continue
		}
		raw := c.ParamValues[i]
		if raw == "" {
			continue
		}
		pname := c.ParamNames[i]
		fv := rv.FieldByIndex(pf.index)
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(raw)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || fv.OverflowInt(n) {
				return BadRequest("invalid path parameter: " + pname)
			}
			fv.SetInt(n)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(raw, 10, 64)
			if err != nil || fv.OverflowUint(n) {
				return BadRequest("invalid path parameter: " + pname)
			}
			fv.SetUint(n)
		case reflect.Bool:
			b, err := strconv.ParseBool(raw)
			if err != nil {
				return BadRequest("invalid path parameter: " + pname)
			}
			fv.SetBool(b)
		case reflect.Float32, reflect.Float64:
			f, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return BadRequest("invalid path parameter: " + pname)
			}
			fv.SetFloat(f)
		}
	}
	return nil
}
