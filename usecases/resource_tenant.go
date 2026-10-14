package usecases

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// resolveTenantScope returns the tenant id for this request when TenantScope
// is configured. ok=false means no usable tenant (caller should 404 / empty).
func resolveTenantScope(c *domain.Ctx, opts ResourceOpts) (tenantID any, col string, ok bool) {
	if opts.TenantScope == nil {
		return nil, "", false
	}
	id, ok := opts.TenantScope(c)
	if !ok || id == nil || isEmptyTenantID(id) {
		return nil, "", false
	}
	col = opts.TenantColumn
	if col == "" {
		col = "tenant_id"
	}
	return id, col, true
}

func isEmptyTenantID(id any) bool {
	switch v := id.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case fmt.Stringer:
		return strings.TrimSpace(v.String()) == ""
	default:
		rv := reflect.ValueOf(id)
		if !rv.IsValid() {
			return true
		}
		if rv.Kind() == reflect.Pointer {
			return rv.IsNil()
		}
		z := reflect.Zero(rv.Type())
		return reflect.DeepEqual(rv.Interface(), z.Interface())
	}
}

func tenantIDString(id any) string {
	switch v := id.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(id)
	}
}

// injectTenantFilter appends a mandatory eq filter for the tenant column.
// Not sourced from querystring — closes IDOR on List when TenantScope is set.
func injectTenantFilter(lq *domain.ListQuery, col string, tenantID any) {
	if lq == nil || col == "" {
		return
	}
	lq.Filters = append(lq.Filters, domain.ListFilter{
		Field: col,
		Value: tenantIDString(tenantID),
		Op:    "eq",
	})
}

// entityBelongsToTenant reports whether entity's tenant field equals tenantID.
// Looks up by Go field name TenantID or json/db tag matching col.
func entityBelongsToTenant(entity any, col string, tenantID any) bool {
	got, ok := readEntityTenantField(entity, col)
	if !ok {
		return false
	}
	return tenantValuesEqual(got, tenantID)
}

func tenantValuesEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	return strings.EqualFold(strings.TrimSpace(tenantIDString(a)), strings.TrimSpace(tenantIDString(b)))
}

func readEntityTenantField(entity any, col string) (any, bool) {
	ev := reflect.ValueOf(entity)
	for ev.Kind() == reflect.Pointer {
		if ev.IsNil() {
			return nil, false
		}
		ev = ev.Elem()
	}
	if ev.Kind() != reflect.Struct {
		return nil, false
	}
	return findTenantField(ev, col)
}

func findTenantField(ev reflect.Value, col string) (any, bool) {
	t := ev.Type()
	colLower := strings.ToLower(col)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fv := ev.Field(i)
		if f.Anonymous && fv.Kind() == reflect.Struct {
			if v, ok := findTenantField(fv, col); ok {
				return v, true
			}
			continue
		}
		if !fv.CanInterface() {
			continue
		}
		jsonTag := strings.Split(f.Tag.Get("json"), ",")[0]
		dbTag := strings.Split(f.Tag.Get("db"), ",")[0]
		if f.Name == "TenantID" ||
			strings.EqualFold(jsonTag, col) ||
			strings.EqualFold(dbTag, col) ||
			(jsonTag == "" && dbTag == "" && strings.EqualFold(toSnakeCase(f.Name), colLower)) {
			return fv.Interface(), true
		}
	}
	return nil, false
}

// stampEntityTenant sets the tenant field on entity to tenantID (create path).
func stampEntityTenant(entity any, col string, tenantID any) error {
	ev := reflect.ValueOf(entity)
	for ev.Kind() == reflect.Pointer {
		if ev.IsNil() {
			return fmt.Errorf("nitro: stamp tenant: nil entity")
		}
		ev = ev.Elem()
	}
	if ev.Kind() != reflect.Struct {
		return fmt.Errorf("nitro: stamp tenant: expected struct")
	}
	fv, ok := findTenantFieldValue(ev, col)
	if !ok || !fv.CanSet() {
		return fmt.Errorf("nitro: stamp tenant: field %q not settable on %s", col, ev.Type())
	}
	return assignTenantValue(fv, tenantID)
}

func findTenantFieldValue(ev reflect.Value, col string) (reflect.Value, bool) {
	t := ev.Type()
	colLower := strings.ToLower(col)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fv := ev.Field(i)
		if f.Anonymous && fv.Kind() == reflect.Struct {
			if v, ok := findTenantFieldValue(fv, col); ok {
				return v, true
			}
			continue
		}
		jsonTag := strings.Split(f.Tag.Get("json"), ",")[0]
		dbTag := strings.Split(f.Tag.Get("db"), ",")[0]
		if f.Name == "TenantID" ||
			strings.EqualFold(jsonTag, col) ||
			strings.EqualFold(dbTag, col) ||
			(jsonTag == "" && dbTag == "" && strings.EqualFold(toSnakeCase(f.Name), colLower)) {
			return fv, true
		}
	}
	return reflect.Value{}, false
}

func assignTenantValue(fv reflect.Value, tenantID any) error {
	if !fv.CanSet() {
		return fmt.Errorf("nitro: stamp tenant: cannot set field")
	}
	want := reflect.ValueOf(tenantID)
	if want.IsValid() && want.Type().AssignableTo(fv.Type()) {
		fv.Set(want)
		return nil
	}
	// Common case: model uses uuid.UUID, scope returns string.
	if s, ok := tenantID.(string); ok {
		if u, err := parseUUIDInto(fv, s); err == nil {
			fv.Set(u)
			return nil
		}
		if fv.Kind() == reflect.String {
			fv.SetString(s)
			return nil
		}
	}
	if want.IsValid() && want.Type().ConvertibleTo(fv.Type()) {
		fv.Set(want.Convert(fv.Type()))
		return nil
	}
	// Stringer / fmt into string field
	if fv.Kind() == reflect.String {
		fv.SetString(tenantIDString(tenantID))
		return nil
	}
	return fmt.Errorf("nitro: stamp tenant: cannot assign %T to %s", tenantID, fv.Type())
}

func parseUUIDInto(fv reflect.Value, s string) (reflect.Value, error) {
	// Avoid hard import cycle: call Parse via type that has UnmarshalText or Parse method.
	pt := fv.Type()
	ptr := reflect.New(pt)
	if u, ok := ptr.Interface().(interface{ UnmarshalText([]byte) error }); ok {
		if err := u.UnmarshalText([]byte(s)); err != nil {
			return reflect.Value{}, err
		}
		return ptr.Elem(), nil
	}
	// pkg/uuid.UUID has Parse as package func — try Scan
	if sc, ok := ptr.Interface().(interface{ Scan(any) error }); ok {
		if err := sc.Scan(s); err != nil {
			return reflect.Value{}, err
		}
		return ptr.Elem(), nil
	}
	return reflect.Value{}, fmt.Errorf("no uuid parse")
}

// filterItemsByTenant keeps only items whose tenant field matches tenantID.
func filterItemsByTenant(items []any, col string, tenantID any) []any {
	if len(items) == 0 {
		return items
	}
	out := make([]any, 0, len(items))
	for _, it := range items {
		if entityBelongsToTenant(it, col, tenantID) {
			out = append(out, it)
		}
	}
	return out
}

// accessAllowed combines OwnerCheck and TenantScope for item operations.
func accessAllowed(c *domain.Ctx, entity any, opts ResourceOpts) bool {
	if opts.TenantScope != nil {
		tid, col, ok := resolveTenantScope(c, opts)
		if !ok || !entityBelongsToTenant(entity, col, tid) {
			return false
		}
	}
	if opts.OwnerCheck != nil && !opts.OwnerCheck(c, entity) {
		return false
	}
	return true
}

func needsEntityLoadForAccess(opts ResourceOpts) bool {
	return opts.OwnerCheck != nil || opts.TenantScope != nil
}

// StampEntityTenant sets the tenant field on entity (create / FillTenantID).
func StampEntityTenant(entity any, col string, tenantID any) error {
	return stampEntityTenant(entity, col, tenantID)
}

// EntityBelongsToTenant reports whether entity's tenant field matches tenantID.
func EntityBelongsToTenant(entity any, col string, tenantID any) bool {
	return entityBelongsToTenant(entity, col, tenantID)
}
