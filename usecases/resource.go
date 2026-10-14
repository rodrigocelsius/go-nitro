package usecases

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

// ResourceRoute represents a route generated from a resource.
type ResourceRoute struct {
	Method  string
	Path    string
	Handler domain.Handler
}

// ResourceOpts configures automatic defaults for generated CRUD.
//
// With no opts (or via DefaultResourceOpts), AutoID, AutoTimestamps, and cache (5m) are on.
//
//	opts := nitro.DefaultResourceOpts()
//	opts.Filterable = []string{"title", "done"}
//	opts.Sortable = []string{"created_at", "title"}
//	opts.CacheTTL = 10 * time.Minute // or &ResourceConfig{CacheTTL: …} / {NoCache: true}
//
// Anti-DoS defaults (when fields are 0): DefaultPerPage=20, MaxPerPage=100,
// MaxPage=10_000, MaxFilters=16, MaxFilterValueLen=200, MaxInMemoryFetch=2_000.
type ResourceOpts struct {
	// AutoID fills an empty string ID with UUID v7 on Create. Default true.
	AutoID bool
	// AutoTimestamps fills CreatedAt (create) and UpdatedAt (create/update). Default true.
	AutoTimestamps bool
	// Filterable lists JSON field names allowed as exact query filters (?title=x).
	Filterable []string
	// FilterRules maps query params to field + op (gte, contains, jsoncontains, …). Empty Op = eq.
	FilterRules []FilterRule
	// Sortable lists JSON field names allowed in ?sort=field or ?sort=-field.
	Sortable []string

	// DefaultPerPage is used when ?per_page is omitted (0 → 20).
	DefaultPerPage int
	// MaxPerPage clamps ?per_page (0 → 100). Prevents huge result sets.
	MaxPerPage int
	// MaxPage clamps ?page (0 → 10_000). Prevents deep offset scans.
	MaxPage int
	// MaxFilters caps how many Filterable/FilterRules may be active at once (0 → 16).
	MaxFilters int
	// MaxFilterValueLen truncates filter values longer than N (0 → 200).
	MaxFilterValueLen int
	// MaxInMemoryFetch caps the window fetched before in-memory filter/sort (0 → 2_000).
	MaxInMemoryFetch int

	// CacheTTL is the in-memory FindByID/List cache duration.
	// Zero → default 5m (applied by pkg). Negative → disables cache.
	CacheTTL time.Duration
	// PatchMerge (default true) unmarshals PATCH onto the existing entity.
	PatchMerge *bool
	// AcceptCamelQuery accepts snake_case and camelCase query params (nil/true = on).
	AcceptCamelQuery *bool
	// BeforeWrite runs on create and update after readonly/computed reset.
	BeforeWrite func(ctx context.Context, entity any) error
	// BeforeCreate/BeforeUpdate hooks (e.g. domain normalize).
	BeforeCreate func(ctx context.Context, entity any) error
	BeforeUpdate func(ctx context.Context, entity any) error
	// AfterCreate/AfterUpdate/AfterDelete hooks.
	AfterCreate func(ctx context.Context, entity any) error
	AfterUpdate func(ctx context.Context, entity any) error
	AfterDelete func(ctx context.Context, id any) error
	// OnMutate is called after create/update/delete with action "create"|"update"|"delete".
	OnMutate func(ctx context.Context, action string, entityOrID any) error

	// OwnerCheck, when set, runs right after Get/Update/Delete load the
	// entity by :id and before it is returned/mutated/removed. Return
	// false to deny access — the handler responds 404 (not 403), so an
	// unauthorized caller cannot distinguish "not mine" from "does not
	// exist" (avoids resource-ID enumeration). This closes the IDOR gap
	// where app.Resource's default CRUD does not scope by owner/tenant.
	// See docs/guide/business-resource.md#ownership and vuln.md (V-05).
	OwnerCheck func(c *domain.Ctx, entity any) bool

	// TenantColumn is the JSON/db field used for TenantScope filters and
	// ownership checks (default "tenant_id" when TenantScope is set).
	TenantColumn string
	// TenantScope, when set, scopes List/Get/Create/Update/Delete to the
	// returned tenant id. Return ok=false to deny (404 on item ops; empty
	// list / 401-style failure on list when no tenant). Prefer
	// nitro.TenantFromCtx() after TenantMiddleware. Combined with
	// OwnerCheck via AND. See docs/guide/resource-security.md.
	TenantScope func(c *domain.Ctx) (tenantID any, ok bool)
}

// FilterRule associates a query param with a DB/JSON field and operator.
type FilterRule struct {
	Param string // query param name (e.g. "min_price")
	Field string // JSON/db field name (e.g. "price")
	Op    string // eq|gte|lte|gt|lt|contains|icontains|jsoncontains; empty=eq
}

// Anti-DoS defaults for Resource list endpoints.
const (
	DefaultListPerPage      = 20
	DefaultListMaxPerPage   = 100
	DefaultListMaxPage      = 10_000
	DefaultListMaxFilters   = 16
	DefaultListMaxFilterLen = 200
	DefaultListMaxInMemory  = 2_000
)

// DefaultResourceOpts returns opts with AutoID, AutoTimestamps, and default cache on.
func DefaultResourceOpts() ResourceOpts {
	return ResourceOpts{
		AutoID:         true,
		AutoTimestamps: true,
		CacheTTL:       5 * time.Minute,
	}
}

func resolveResourceOpts(opts []ResourceOpts) ResourceOpts {
	if len(opts) == 0 {
		return DefaultResourceOpts()
	}
	return opts[0]
}

// ScanResource scans a struct for CRUD methods.
// Supports methods: List, Get, Create, Update, Delete.
// Kept for compatibility with controllers that implement those methods.
func ScanResource(prefix string, controller any) []ResourceRoute {
	rv := reflect.ValueOf(controller)
	rt := rv.Type()

	var routes []ResourceRoute

	for i := 0; i < rt.NumMethod(); i++ {
		method := rt.Method(i)
		name := method.Name

		var httpMethod, path string
		switch name {
		case "List":
			httpMethod = "GET"
			path = prefix
		case "Get":
			httpMethod = "GET"
			path = prefix + "/:id"
		case "Create":
			httpMethod = "POST"
			path = prefix
		case "Update":
			httpMethod = "PATCH"
			path = prefix + "/:id"
		case "Delete":
			httpMethod = "DELETE"
			path = prefix + "/:id"
		default:
			continue
		}

		handler := method.Func
		wrapped := func(h reflect.Value) domain.Handler {
			return func(c *domain.Ctx) error {
				args := []reflect.Value{reflect.ValueOf(controller), reflect.ValueOf(c)}
				results := h.Call(args)
				if len(results) > 0 && !results[0].IsNil() {
					return results[0].Interface().(error)
				}
				return nil
			}
		}(handler)

		routes = append(routes, ResourceRoute{
			Method:  httpMethod,
			Path:    path,
			Handler: wrapped,
		})
	}

	return routes
}

// ResourceMetadata holds the metadata extracted from a model struct.
type ResourceMetadata struct {
	TableName string
	IDColumn  string
	ModelType reflect.Type
	Fields    []ResourceField
}

// ResourceField describes a single field in a model struct.
type ResourceField struct {
	Name     string
	JSONName string
	Type     reflect.Type
	IsID     bool
}

// ExtractResourceMetadata extracts metadata from a model struct type.
// It derives table name, ID column, and field information.
func ExtractResourceMetadata(modelType reflect.Type) ResourceMetadata {
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	meta := ResourceMetadata{
		TableName: deriveTableName(modelType.Name()),
		IDColumn:  "id",
		ModelType: modelType,
	}

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		if field.PkgPath != "" {
			continue // unexported
		}

		jsonName := field.Tag.Get("json")
		if jsonName == "" {
			jsonName = toSnakeCase(field.Name)
		} else {
			jsonName = strings.Split(jsonName, ",")[0]
		}

		isID := strings.EqualFold(field.Name, "id") || field.Tag.Get("primary") == "true"

		meta.Fields = append(meta.Fields, ResourceField{
			Name:     field.Name,
			JSONName: jsonName,
			Type:     field.Type,
			IsID:     isID,
		})

		if isID {
			meta.IDColumn = jsonName
		}
	}

	return meta
}

func deriveTableName(name string) string {
	snake := toSnakeCase(name)
	return Pluralize(snake)
}

func toSnakeCase(s string) string {
	var result []rune
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := runes[i-1]
				if prev >= 'a' && prev <= 'z' {
					result = append(result, '_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z' {
					result = append(result, '_')
				}
			}
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

// GenerateResourceRoutes creates 5 CRUD routes for a model struct.
// Optional opts: with no args, uses DefaultResourceOpts() (AutoID + timestamps).
// If the model has a numeric ID (BIGSERIAL), AutoID is turned off automatically.
func GenerateResourceRoutes(prefix string, modelType reflect.Type, repo domain.ResourceRepository, opts ...ResourceOpts) []ResourceRoute {
	meta := ExtractResourceMetadata(modelType)
	o := resolveResourceOpts(opts)
	if o.AutoID && domain.ModelIDIsInteger(modelType) {
		o.AutoID = false
	}

	return []ResourceRoute{
		{Method: "GET", Path: prefix, Handler: makeListHandler(meta, repo, o)},
		{Method: "GET", Path: prefix + "/:id", Handler: makeGetHandler(meta, repo, o)},
		{Method: "POST", Path: prefix, Handler: makeCreateHandler(meta, repo, o)},
		{Method: "PATCH", Path: prefix + "/:id", Handler: makeUpdateHandler(meta, repo, o)},
		{Method: "DELETE", Path: prefix + "/:id", Handler: makeDeleteHandler(meta, repo, o)},
	}
}

func makeListHandler(meta ResourceMetadata, repo domain.ResourceRepository, opts ResourceOpts) domain.Handler {
	return func(c *domain.Ctx) error {
		page, perPage := clampListPagination(c, opts)

		var tenantID any
		var tenantCol string
		var hasTenant bool
		if opts.TenantScope != nil {
			var ok bool
			tenantID, tenantCol, ok = resolveTenantScope(c, opts)
			if !ok {
				return respondJSON(c, 200, &domain.ResourceListResult{Items: []any{}, Page: page, PerPage: perPage, Total: 0})
			}
			hasTenant = true
		}

		// BLinq/SQL push-down when the repo implements QueryableLister.
		if ql, ok := repo.(domain.QueryableLister); ok {
			lq := buildListQuery(c, page, perPage, opts)
			if hasTenant {
				injectTenantFilter(&lq, tenantCol, tenantID)
			}
			result, err := ql.ListQuery(c.Context(), lq)
			if err != nil {
				return mapRepoError(err, meta.TableName+": list failed")
			}
			return respondJSON(c, 200, result)
		}

		fetchPage, fetchPer := page, perPage
		needPost := len(opts.Filterable) > 0 || len(opts.FilterRules) > 0 || len(opts.Sortable) > 0 || hasTenant
		if needPost {
			// Fetch a bounded window then filter/sort/paginate in-memory (MemoryRepo / small datasets).
			fetchPage, fetchPer = 1, effectiveMaxInMemory(opts)
		}

		result, err := repo.List(c.Context(), fetchPage, fetchPer)
		if err != nil {
			return mapRepoError(err, meta.TableName+": list failed")
		}

		if needPost && result != nil {
			items := result.Items
			if hasTenant {
				items = filterItemsByTenant(items, tenantCol, tenantID)
			}
			items = filterItemsOpts(c, items, opts)
			items = sortItems(c, items, opts.Sortable)
			result = paginateAnyItems(items, page, perPage)
		}

		return respondJSON(c, 200, result)
	}
}

func makeGetHandler(meta ResourceMetadata, repo domain.ResourceRepository, opts ResourceOpts) domain.Handler {
	return func(c *domain.Ctx) error {
		id := c.Param("id")
		if id == "" {
			return domain.NewBadRequest("missing id parameter")
		}

		entity, err := repo.FindByID(c.Context(), id)
		if err != nil {
			return mapRepoError(err, meta.TableName+": not found")
		}
		if entity == nil {
			return domain.NewNotFound(meta.TableName + ": not found")
		}
		if !accessAllowed(c, entity, opts) {
			return domain.NewNotFound(meta.TableName + ": not found")
		}

		return respondJSON(c, 200, entity)
	}
}

func makeCreateHandler(meta ResourceMetadata, repo domain.ResourceRepository, opts ResourceOpts) domain.Handler {
	return func(c *domain.Ctx) error {
		entity := reflect.New(meta.ModelType).Interface()

		if err := bindAndValidateJSON(c, entity); err != nil {
			return err
		}
		resetReadonlyFields(entity, nil)
		resetSystemFields(entity, nil, opts, true)

		if opts.TenantScope != nil {
			tid, col, ok := resolveTenantScope(c, opts)
			if !ok {
				return domain.NewUnauthorized("missing tenant")
			}
			if err := stampEntityTenant(entity, col, tid); err != nil {
				return domain.NewInternal(fmt.Errorf("tenant stamp: %w", err))
			}
		}

		applyResourceDefaults(entity, opts, true)

		if opts.BeforeWrite != nil {
			if err := opts.BeforeWrite(c.Context(), entity); err != nil {
				return err
			}
		}
		if opts.BeforeCreate != nil {
			if err := opts.BeforeCreate(c.Context(), entity); err != nil {
				return err
			}
		}

		if err := repo.Create(c.Context(), entity); err != nil {
			return mapRepoError(err, meta.TableName+": create failed")
		}

		if opts.AfterCreate != nil {
			if err := opts.AfterCreate(c.Context(), entity); err != nil {
				return err
			}
		}
		if opts.OnMutate != nil {
			if err := opts.OnMutate(c.Context(), "create", entity); err != nil {
				return err
			}
		}

		return respondJSON(c, 201, entity)
	}
}

func makeUpdateHandler(meta ResourceMetadata, repo domain.ResourceRepository, opts ResourceOpts) domain.Handler {
	return func(c *domain.Ctx) error {
		id := c.Param("id")
		if id == "" {
			return domain.NewBadRequest("missing id parameter")
		}

		merge := opts.PatchMerge == nil || *opts.PatchMerge
		var entity any

		// When OwnerCheck or TenantScope is configured, always load the
		// existing entity first — even in replace mode — so access can be
		// verified before any mutation, closing the IDOR gap on PATCH/PUT :id.
		var existing any
		if merge || needsEntityLoadForAccess(opts) {
			var err error
			existing, err = repo.FindByID(c.Context(), id)
			if err != nil {
				return mapRepoError(err, meta.TableName+": not found")
			}
			if existing == nil {
				return domain.NewNotFound(meta.TableName + ": not found")
			}
			if !accessAllowed(c, existing, opts) {
				return domain.NewNotFound(meta.TableName + ": not found")
			}
		}

		if merge {
			entity = cloneEntity(existing, meta.ModelType)
			if err := bindPatchJSON(c, entity); err != nil {
				return err
			}
			resetReadonlyFields(entity, existing)
			resetSystemFields(entity, existing, opts, false)
		} else {
			entity = reflect.New(meta.ModelType).Interface()
			if err := bindAndValidateJSON(c, entity); err != nil {
				return err
			}
			resetReadonlyFields(entity, nil)
			resetSystemFields(entity, existing, opts, false)
		}

		setEntityIDField(domain.FieldByName(reflect.ValueOf(entity).Elem(), "ID"), id)

		// The tenant is owned by the server: re-stamp it on every update so a
		// client can neither move the record to another tenant (merge/PATCH)
		// nor orphan it by wiping the column (replace/PUT).
		if opts.TenantScope != nil {
			tid, col, ok := resolveTenantScope(c, opts)
			if !ok {
				return domain.NewUnauthorized("missing tenant")
			}
			if err := stampEntityTenant(entity, col, tid); err != nil {
				return domain.NewInternal(fmt.Errorf("tenant stamp: %w", err))
			}
		}

		applyResourceDefaults(entity, opts, false)

		if opts.BeforeWrite != nil {
			if err := opts.BeforeWrite(c.Context(), entity); err != nil {
				return err
			}
		}
		if opts.BeforeUpdate != nil {
			if err := opts.BeforeUpdate(c.Context(), entity); err != nil {
				return err
			}
		}

		if err := repo.Update(c.Context(), entity); err != nil {
			return mapRepoError(err, meta.TableName+": not found")
		}

		if opts.AfterUpdate != nil {
			if err := opts.AfterUpdate(c.Context(), entity); err != nil {
				return err
			}
		}
		if opts.OnMutate != nil {
			if err := opts.OnMutate(c.Context(), "update", entity); err != nil {
				return err
			}
		}

		return respondJSON(c, 200, entity)
	}
}

func makeDeleteHandler(meta ResourceMetadata, repo domain.ResourceRepository, opts ResourceOpts) domain.Handler {
	return func(c *domain.Ctx) error {
		id := c.Param("id")
		if id == "" {
			return domain.NewBadRequest("missing id parameter")
		}

		if needsEntityLoadForAccess(opts) {
			existing, err := repo.FindByID(c.Context(), id)
			if err != nil {
				return mapRepoError(err, meta.TableName+": not found")
			}
			if existing == nil || !accessAllowed(c, existing, opts) {
				return domain.NewNotFound(meta.TableName + ": not found")
			}
		}

		if err := repo.Delete(c.Context(), id); err != nil {
			return mapRepoError(err, meta.TableName+": not found")
		}

		if opts.AfterDelete != nil {
			if err := opts.AfterDelete(c.Context(), id); err != nil {
				return err
			}
		}
		if opts.OnMutate != nil {
			if err := opts.OnMutate(c.Context(), "delete", id); err != nil {
				return err
			}
		}

		return domain.RespNoContent()
	}
}

func cloneEntity(existing any, modelType reflect.Type) any {
	dst := reflect.New(modelType).Interface()
	ev := reflect.ValueOf(existing)
	for ev.Kind() == reflect.Pointer {
		if ev.IsNil() {
			return dst
		}
		ev = ev.Elem()
	}
	dv := reflect.ValueOf(dst).Elem()
	if ev.Type().AssignableTo(dv.Type()) {
		dv.Set(ev)
	} else if ev.Type().ConvertibleTo(dv.Type()) {
		dv.Set(ev.Convert(dv.Type()))
	}
	return dst
}

// resetReadonlyFields closes the classic mass-assignment gap (CWE-915):
// after binding an untrusted JSON body onto a model, any field tagged
// `readonly:"true"` or `computed:"true"` is reset — to its zero value on
// create/replace, or restored to the pre-update value on merge/PATCH — so a
// client can never set Role/IsAdmin/TenantID/denorm names just because they
// are exported on the persistence model. See docs/guide/models.md.
func resetReadonlyFields(entity any, existing any) {
	ev := reflect.ValueOf(entity)
	for ev.Kind() == reflect.Pointer {
		if ev.IsNil() {
			return
		}
		ev = ev.Elem()
	}
	if ev.Kind() != reflect.Struct {
		return
	}

	var xv reflect.Value
	if existing != nil {
		xv = reflect.ValueOf(existing)
		for xv.Kind() == reflect.Pointer {
			xv = xv.Elem()
		}
	}

	if xv.IsValid() && (xv.Kind() != reflect.Struct || xv.Type() != ev.Type()) {
		xv = reflect.Value{}
	}
	resetTaggedFields(ev, xv)
}

// resetTaggedFields resets readonly/computed fields of ev, descending into
// embedded structs (nitro.Model, nitro.TenantModel, …) so promoted fields are
// protected too. xv is the pre-update value of the same type, or invalid.
func resetTaggedFields(ev, xv reflect.Value) {
	t := ev.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fv := ev.Field(i)
		if !fv.CanSet() {
			continue
		}
		var xf reflect.Value
		if xv.IsValid() {
			xf = xv.Field(i)
		}
		if f.Anonymous && f.Type.Kind() == reflect.Struct && !isProtectedFieldTag(f.Tag) {
			resetTaggedFields(fv, xf)
			continue
		}
		if !isProtectedFieldTag(f.Tag) {
			continue
		}
		if xf.IsValid() {
			fv.Set(xf)
		} else {
			fv.Set(reflect.Zero(fv.Type()))
		}
	}
}

// resetSystemFields enforces the convention that ID, CreatedAt, UpdatedAt and
// DeletedAt are owned by the server: values bound from the request body are
// discarded on create, and restored from the stored row on update. Without
// this a client could pick the primary key (overwriting another row or
// tenant), back-date timestamps, or create rows that are already
// soft-deleted. A string ID is left alone when AutoID is off (natural keys).
// On update without a loaded row (replace mode, no access check), CreatedAt
// is left as sent and DeletedAt is cleared.
func resetSystemFields(entity, existing any, opts ResourceOpts, isCreate bool) {
	ev := reflect.ValueOf(entity)
	for ev.Kind() == reflect.Pointer {
		if ev.IsNil() {
			return
		}
		ev = ev.Elem()
	}
	if ev.Kind() != reflect.Struct {
		return
	}
	var xv reflect.Value
	if existing != nil {
		xv = reflect.ValueOf(existing)
		for xv.Kind() == reflect.Pointer && !xv.IsNil() {
			xv = xv.Elem()
		}
		if xv.Kind() != reflect.Struct || xv.Type() != ev.Type() {
			xv = reflect.Value{}
		}
	}
	field := domain.FieldByName // cached; nil embedded pointer → invalid
	restore := func(name string) {
		fv := field(ev, name)
		if !fv.IsValid() || !fv.CanSet() {
			return
		}
		if xf := field(xv, name); xf.IsValid() {
			fv.Set(xf)
		} else {
			fv.Set(reflect.Zero(fv.Type()))
		}
	}
	if isCreate {
		if idf := field(ev, "ID"); idf.IsValid() && (opts.AutoID || isIntegerKind(idf.Kind())) {
			restore("ID")
		}
		if opts.AutoTimestamps {
			restore("CreatedAt")
			restore("UpdatedAt")
		}
	} else if xv.IsValid() {
		restore("CreatedAt")
	}
	restore("DeletedAt")
}

func isIntegerKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

func isProtectedFieldTag(tag reflect.StructTag) bool {
	return tag.Get("readonly") == "true" || tag.Get("computed") == "true"
}

func bindPatchJSON(c *domain.Ctx, v any) error {
	body, err := readBodyBytes(c)
	if err != nil {
		return domain.NewBadRequest("invalid json body")
	}
	if err := validator.UnmarshalJSONPatch(body, v); err != nil {
		return domain.NewBadRequest("invalid json body")
	}
	if errs := ValidateStruct(v); errs.HasErrors() {
		return domain.NewValidationFailed(errs)
	}
	return nil
}

func readBodyBytes(c *domain.Ctx) ([]byte, error) {
	type bodyReader interface{ Body() []byte }
	if b, ok := c.AdapterData.(bodyReader); ok {
		return b.Body(), nil
	}
	return nil, fmt.Errorf("no body")
}

// applyResourceDefaults fills ID and timestamps according to ResourceOpts.
func applyResourceDefaults(entity any, opts ResourceOpts, isCreate bool) {
	domain.ApplyEntityDefaults(entity, domain.EntityDefaults{
		AutoID:         opts.AutoID,
		AutoTimestamps: opts.AutoTimestamps,
	}, isCreate)
}

func queryIntDefault(c *domain.Ctx, key string, defaultVal int) int {
	s := c.QueryParam(key)
	if s == "" {
		return defaultVal
	}
	v := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			v = v*10 + int(ch-'0')
		} else {
			return defaultVal
		}
	}
	if v <= 0 {
		return defaultVal
	}
	return v
}

type bodyJSONBinder interface {
	BodyJSON(any) error
}

type plainBinder interface {
	Bind(any) error
}

func bindAndValidateJSON(c *domain.Ctx, v any) error {
	if b, ok := c.AdapterData.(bodyJSONBinder); ok {
		return b.BodyJSON(v)
	}
	if b, ok := c.AdapterData.(plainBinder); ok {
		if err := b.Bind(v); err != nil {
			return domain.NewBadRequest("invalid json body")
		}
		validator.ApplyTransforms(v)
		if errs := ValidateStruct(v); errs.HasErrors() {
			return domain.NewValidationFailed(errs)
		}
		return nil
	}
	return domain.NewBadRequest("bind: adapter does not support Bind")
}

func mapRepoError(err error, fallbackNotFound string) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(domain.HTTPError); ok {
		return err
	}
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		if fallbackNotFound != "" {
			return domain.NewNotFound(fallbackNotFound)
		}
		return domain.NewNotFound(err.Error())
	}
	type notFounder interface{ NotFound() bool }
	var nf notFounder
	if errors.As(err, &nf) && nf.NotFound() {
		if fallbackNotFound != "" {
			return domain.NewNotFound(fallbackNotFound)
		}
		return domain.NewNotFound(err.Error())
	}
	return domain.NewInternal(err)
}

func respondJSON(c *domain.Ctx, status int, v any) error {
	if j, ok := c.AdapterData.(interface{ JSON(int, any) error }); ok {
		return j.JSON(status, v)
	}
	return fmt.Errorf("respond: adapter does not support JSON response")
}

func normalizePath(prefix string) string {
	prefix = strings.TrimPrefix(prefix, "/")
	prefix = strings.TrimSuffix(prefix, "/")
	return "/" + prefix
}

var irregularPlurals = map[string]string{
	"person":   "people",
	"man":      "men",
	"woman":    "women",
	"child":    "children",
	"foot":     "feet",
	"tooth":    "teeth",
	"mouse":    "mice",
	"goose":    "geese",
	"datum":    "data",
	"medium":   "media",
	"ox":       "oxen",
	"matrix":   "matrices",
	"vertex":   "vertices",
	"index":    "indices",
	"appendix": "appendices",
}

// Pluralize is a simple pluralization helper.
func Pluralize(word string) string {
	if word == "" {
		return word
	}

	lower := strings.ToLower(word)
	if plural, ok := irregularPlurals[lower]; ok {
		if len(word) > 0 && unicode.IsUpper(rune(word[0])) {
			return strings.ToUpper(plural[:1]) + plural[1:]
		}
		return plural
	}

	runes := []rune(word)
	last := runes[len(runes)-1]
	secondLast := rune(0)
	if len(runes) > 1 {
		secondLast = runes[len(runes)-2]
	}

	if last == 's' || last == 'x' ||
		(last == 'h' && (secondLast == 'c' || secondLast == 's')) {
		return word + "es"
	}
	if last == 'z' {
		return word + string(last) + "es"
	}

	if last == 'y' && secondLast != 0 && !isVowel(secondLast) {
		return string(runes[:len(runes)-1]) + "ies"
	}

	if last == 'f' {
		return string(runes[:len(runes)-1]) + "ves"
	}
	if last == 'e' && secondLast == 'f' {
		return string(runes[:len(runes)-2]) + "ves"
	}

	return word + "s"
}

func isVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

func setEntityIDField(idField reflect.Value, id string) {
	if !idField.IsValid() || !idField.CanSet() {
		return
	}
	switch idField.Kind() {
	case reflect.String:
		idField.SetString(id)
	default:
		if idField.Type() == reflect.TypeOf(nitrouuid.UUID{}) {
			u, err := nitrouuid.Parse(id)
			if err != nil {
				return
			}
			idField.Set(reflect.ValueOf(u))
		}
	}
}
