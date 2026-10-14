package nitro

import (
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
	"reflect"
)

// TenantFromCtx returns a ResourceConfig.TenantScope that reads c.TenantID()
// (set by JWTMiddleware claim and/or TenantMiddleware). ok is false when empty.
func TenantFromCtx() func(c *domain.Ctx) (tenantID any, ok bool) {
	return func(c *domain.Ctx) (any, bool) {
		id := tenantIDFromCtx(c)
		if id == "" {
			return nil, false
		}
		return id, true
	}
}

func tenantIDFromCtx(c *domain.Ctx) string {
	if c == nil {
		return ""
	}
	if fc, ok := c.AdapterData.(interface{ TenantID() string }); ok {
		return fc.TenantID()
	}
	return ""
}

// FillTenantID sets entity's tenant field (TenantID / json tenant_id) to id.
// Used when stamping outside Resource (custom handlers). Prefer TenantScope
// on Resource for CRUD.
func FillTenantID(entity any, id any) error {
	if entity == nil || id == nil {
		return nil
	}
	return usecases.StampEntityTenant(entity, "tenant_id", id)
}

// applyTenantConvention enables TenantFromCtx() on Resources whose model has
// a tenant column, so multi-tenant isolation needs no configuration and cannot
// be forgotten (fail-closed: no tenant → 401).
func (a *App) applyTenantConvention(model any, cfg *ResourceConfig) *ResourceConfig {
	if cfg != nil && (cfg.TenantScope != nil || cfg.NoTenantScope) {
		return cfg
	}
	col := "tenant_id"
	if cfg != nil && cfg.TenantColumn != "" {
		col = cfg.TenantColumn
	}
	if !modelHasColumn(model, col) {
		return cfg
	}
	if cfg == nil {
		cfg = &ResourceConfig{}
	}
	cfg.TenantScope = TenantFromCtx()
	return cfg
}

// modelHasColumn reports whether the model struct (including embedded
// structs) exposes a field whose JSON/db column name is col.
func modelHasColumn(model any, col string) bool {
	t := reflect.TypeOf(model)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return false
	}
	var walk func(reflect.Type, int) bool
	walk = func(rt reflect.Type, depth int) bool {
		if depth > 4 {
			return false
		}
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if f.Anonymous {
				ft := f.Type
				if ft.Kind() == reflect.Ptr {
					ft = ft.Elem()
				}
				if ft.Kind() == reflect.Struct && walk(ft, depth+1) {
					return true
				}
				continue
			}
			if f.PkgPath != "" {
				continue
			}
			if columnJSONName(f) == col {
				return true
			}
		}
		return false
	}
	return walk(t, 0)
}
