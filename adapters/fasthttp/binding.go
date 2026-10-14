package fasthttp

import (
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
)

// fieldInfo stores pre-computed metadata for a struct field.
type fieldInfo struct {
	Index     int
	TagName   string
	FieldName string
}

// structCache caches parsed struct field metadata by type.
var structCache sync.Map
var formStructCache sync.Map

// getFieldInfos returns cached field metadata for a struct type.
// The tag is tried in order: param, query, header, cookie, form.
func getFieldInfos(rt reflect.Type) []fieldInfo {
	if cached, ok := structCache.Load(rt); ok {
		return cached.([]fieldInfo)
	}

	infos := make([]fieldInfo, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Tag.Get("param")
		if name == "" {
			name = f.Tag.Get("query")
		}
		if name == "" {
			name = f.Tag.Get("header")
		}
		if name == "" {
			name = f.Tag.Get("cookie")
		}
		if name == "" {
			name = formTagName(f)
		}
		if name != "" {
			infos = append(infos, fieldInfo{
				Index:     i,
				TagName:   name,
				FieldName: f.Name,
			})
		}
	}

	// Store a copy to avoid races on the slice
	stored := make([]fieldInfo, len(infos))
	copy(stored, infos)
	structCache.Store(rt, stored)
	return stored
}

var queryStructCache sync.Map

// getQueryFieldInfos is getFieldInfos plus the query-string convention: a
// scalar field without an explicit param/query/form tag is bound from its JSON
// name, else the snake_case of its Go name (`Shout` ← ?shout=, `UnitPrice` ←
// ?unit_price=). Explicit tags always win; `query:"-"` opts a field out.
func getQueryFieldInfos(rt reflect.Type) []fieldInfo {
	if cached, ok := queryStructCache.Load(rt); ok {
		return cached.([]fieldInfo)
	}
	infos := make([]fieldInfo, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() || f.Anonymous {
			continue
		}
		name := f.Tag.Get("param")
		if name == "" {
			name = f.Tag.Get("query")
		}
		if name == "" {
			name = formTagName(f)
		}
		if name == "-" {
			continue
		}
		if name == "" {
			switch f.Type.Kind() {
			case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
			default:
				continue
			}
			if j := f.Tag.Get("json"); j != "" && j != "-" {
				name = strings.Split(j, ",")[0]
			}
			if name == "" {
				name = transform.SnakeCase(f.Name)
			}
		}
		infos = append(infos, fieldInfo{Index: i, TagName: name, FieldName: f.Name})
	}
	stored := make([]fieldInfo, len(infos))
	copy(stored, infos)
	queryStructCache.Store(rt, stored)
	return stored
}

func formTagName(f reflect.StructField) string {
	if name := f.Tag.Get("form"); name != "" {
		if name == "-" {
			return ""
		}
		if i := strings.IndexByte(name, ','); i >= 0 {
			name = name[:i]
		}
		return name
	}
	return ""
}

func getFormFieldInfos(rt reflect.Type) []fieldInfo {
	if cached, ok := formStructCache.Load(rt); ok {
		return cached.([]fieldInfo)
	}
	infos := make([]fieldInfo, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		name := formTagName(f)
		if name == "" {
			if j := f.Tag.Get("json"); j != "" && j != "-" {
				if i := strings.IndexByte(j, ','); i >= 0 {
					j = j[:i]
				}
				name = j
			}
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		infos = append(infos, fieldInfo{
			Index:     i,
			TagName:   name,
			FieldName: f.Name,
		})
	}
	stored := make([]fieldInfo, len(infos))
	copy(stored, infos)
	formStructCache.Store(rt, stored)
	return stored
}

// BindPath binds path parameters.
func (c *FastCtx) BindPath(dest any) error {
	return bindFromSource(c, dest, domain.SourcePath)
}

// BindQuery binds query parameters.
func (c *FastCtx) BindQuery(dest any) error {
	return bindFromSource(c, dest, domain.SourceQuery)
}

// BindHeader binds request headers.
func (c *FastCtx) BindHeader(dest any) error {
	return bindFromSource(c, dest, domain.SourceHeader)
}

// BindCookie binds request cookies.
func (c *FastCtx) BindCookie(dest any) error {
	return bindFromSource(c, dest, domain.SourceCookie)
}

// BindForm binds application/x-www-form-urlencoded or multipart fields onto the struct.
// Tags: `form:"name"`, fallback `json:"name"`, otherwise the field name in lowercase.
func (c *FastCtx) BindForm(dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return &domain.BindError{Source: domain.SourceForm, Message: "destination must be a pointer to struct"}
	}
	rv = rv.Elem()
	infos := getFormFieldInfos(rv.Type())
	for _, info := range infos {
		raw := c.FormValue(info.TagName)
		if raw == "" {
			continue
		}
		if err := convertAndSet(rv.Field(info.Index), raw); err != nil {
			return &domain.BindError{
				Field:   info.FieldName,
				Source:  domain.SourceForm,
				Message: err.Error(),
			}
		}
	}
	return nil
}

func bindFromSource(c *FastCtx, dest any, source domain.ParamSource) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return &domain.BindError{Source: source, Message: "destination must be a pointer to struct"}
	}

	rv = rv.Elem()
	rt := rv.Type()

	infos := getFieldInfos(rt)
	if source == domain.SourceQuery {
		infos = getQueryFieldInfos(rt)
	}
	if len(infos) == 0 {
		return nil
	}

	for _, info := range infos {
		fieldValue := rv.Field(info.Index)
		var raw string

		switch source {
		case domain.SourcePath:
			raw = c.Ctx.Param(info.TagName)
		case domain.SourceQuery:
			raw = c.Query(info.TagName)
		case domain.SourceHeader:
			raw = c.Header(info.TagName)
		case domain.SourceCookie:
			raw = c.Cookie(info.TagName)
		case domain.SourceBody:
			raw = string(c.Body())
		case domain.SourceForm:
			raw = c.FormValue(info.TagName)
		}

		if raw == "" {
			continue
		}

		if err := convertAndSet(fieldValue, raw); err != nil {
			return &domain.BindError{
				Field:   info.FieldName,
				Source:  source,
				Message: err.Error(),
			}
		}
	}

	return nil
}

func convertAndSet(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	default:
		return &domain.BindError{Message: "unsupported type: " + field.Type().String()}
	}
	return nil
}
