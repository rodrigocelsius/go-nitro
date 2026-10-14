package nitro

import (
	"context"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

// CRUD action names for ResourceConfig.Only / Except / Replace.
const (
	ActionList   = "list"
	ActionGet    = "get"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
)

// ResourceConfig is the single public configuration for Resource (preferred over resourceOpts).
type ResourceConfig struct {
	// NoCache disables in-memory L1 cache (same as CacheTTL < 0).
	NoCache        bool
	CacheTTL       time.Duration
	AutoID         *bool
	AutoTimestamps *bool
	Filterable     []string
	FilterRules    []FilterRule
	Sortable       []string

	// DefaultPerPage when ?per_page is omitted (0 → 20).
	DefaultPerPage int
	// MaxPerPage clamps ?per_page (0 → 100).
	MaxPerPage int
	// MaxPage clamps ?page (0 → 10_000).
	MaxPage int
	// MaxFilters caps concurrent Filterable/FilterRules (0 → 16).
	MaxFilters int
	// MaxFilterValueLen truncates filter values (0 → 200).
	MaxFilterValueLen int
	// MaxInMemoryFetch caps in-memory filter/sort window (0 → 2_000).
	MaxInMemoryFetch int

	Middleware      []Handler
	ReadMiddleware  []Handler
	WriteMiddleware []Handler

	// Public acknowledges intentional unauthenticated CRUD (list/get/create/
	// update/delete). Prefer app.SetDefaultResourceMiddleware(JWT…) so
	// Resources inherit auth; use Public only for consciously open APIs.
	// In EnvProduction, missing auth without Public panics unless
	// Config.Security.AllowUnauthenticatedResources (V-04).
	Public bool

	// AcceptCamelQuery accepts both snake_case and camelCase query params for
	// Filterable / FilterRules / sort (e.g. ?artistId= and ?artist_id=).
	// nil = true (default on). Set to a false pointer to opt out.
	AcceptCamelQuery *bool

	// Aliases mounts the same Resource on additional path prefixes.
	Aliases []string
	// ExtraRoutes extra routes under the prefix (registered before :id).
	ExtraRoutes []ResourceExtraRoute

	// Only declares which CRUD actions to register (list|get|create|update|delete).
	// Empty = all 5 actions (convention). Use when the Resource should not expose everything.
	Only []string
	// Except removes actions from the default set (ignored if Only is set).
	Except []string
	// Replace swaps the handler of a default action (targeted change without leaving Resource).
	Replace map[string]Handler

	// Cache + CacheBustPattern invalidates the key pattern in OnMutate automatically.
	Cache            Cache
	CacheBustPattern string

	// PatchMerge (nil=true) merges PATCH onto the existing entity.
	PatchMerge *bool

	// BeforeWrite runs on create and update after readonly/computed reset and
	// defaults, before BeforeCreate / BeforeUpdate. Prefer BeforeWrite when the
	// same normalize/denorm logic applies to both verbs.
	BeforeWrite func(ctx context.Context, entity any) error

	BeforeCreate func(ctx context.Context, entity any) error
	BeforeUpdate func(ctx context.Context, entity any) error
	AfterCreate  func(ctx context.Context, entity any) error
	AfterUpdate  func(ctx context.Context, entity any) error
	AfterDelete  func(ctx context.Context, id any) error
	OnMutate     func(ctx context.Context, action string, entityOrID any) error

	// OwnerCheck, when set, runs after Get/Update/Delete load the entity by
	// :id and before it is returned/mutated/removed — return false to deny
	// access (responds 404, not 403, so an unauthorized caller can't tell
	// "not yours" from "does not exist"). app.Resource's CRUD does not
	// scope by owner/tenant by default; use this to close that IDOR gap:
	//
	//	OwnerCheck: func(c *domain.Ctx, entity any) bool {
	//	    return entity.(*Order).UserID == c.UserID()
	//	}
	OwnerCheck func(c *domain.Ctx, entity any) bool

	// TenantColumn is the JSON/db field used with TenantScope (default
	// "tenant_id"). Models embedding nitro.TenantModel already use that name.
	TenantColumn string
	// TenantScope scopes List/Get/Create/Update/Delete to one tenant.
	//
	// Convention: when the model has a tenant column (embed nitro.TenantModel
	// or add a `tenant_id` field) and TenantScope is nil, Resource applies
	// TenantFromCtx() automatically — the tenant comes from the authenticated
	// token (claim tenant_id / tenantId / tid / org_id) and requests without a
	// tenant are rejected (401). Set NoTenantScope to opt out.
	// Prefer nitro.TenantFromCtx() after TenantMiddleware. Combined with
	// OwnerCheck via AND. See docs/guide/resource-security.md.
	//
	//	TenantScope: nitro.TenantFromCtx(),
	TenantScope func(c *domain.Ctx) (tenantID any, ok bool)
	// NoTenantScope opts a model that has a tenant column out of the automatic
	// tenant scoping (see the convention on TenantScope), e.g. an admin-only or
	// shared reference table.
	NoTenantScope bool

	// Rules run before every item Command (entity already loaded by :id).
	Rules []ResourceRule
	// Commands are business actions: POST /:id/{name} or POST /{name}.
	Commands []ResourceCommand
	// Queries are named read routes: GET /queries/{name}.
	Queries []ResourceQuery
}

// ResourceExtraRoute is a custom route under a Resource prefix.
type ResourceExtraRoute struct {
	Method  string  // GET, POST, …
	Path    string  // relative, e.g. "/export"
	Handler Handler // required
	Write   bool    // use WriteMiddleware (+ Middleware)
}

func (c *ResourceConfig) toOpts() resourceOpts {
	o := usecases.DefaultResourceOpts()
	if c == nil {
		return o
	}
	if c.NoCache {
		o.CacheTTL = -1
	} else if c.CacheTTL != 0 {
		o.CacheTTL = c.CacheTTL
	}
	if c.AutoID != nil {
		o.AutoID = *c.AutoID
	}
	if c.AutoTimestamps != nil {
		o.AutoTimestamps = *c.AutoTimestamps
	}
	if c.Filterable != nil {
		o.Filterable = append([]string(nil), c.Filterable...)
	}
	if c.FilterRules != nil {
		o.FilterRules = append([]FilterRule(nil), c.FilterRules...)
	}
	if c.Sortable != nil {
		o.Sortable = append([]string(nil), c.Sortable...)
	}
	o.DefaultPerPage = c.DefaultPerPage
	o.MaxPerPage = c.MaxPerPage
	o.MaxPage = c.MaxPage
	o.MaxFilters = c.MaxFilters
	o.MaxFilterValueLen = c.MaxFilterValueLen
	o.MaxInMemoryFetch = c.MaxInMemoryFetch
	o.PatchMerge = c.PatchMerge
	o.AcceptCamelQuery = c.AcceptCamelQuery
	o.BeforeWrite = c.BeforeWrite
	o.BeforeCreate = c.BeforeCreate
	o.BeforeUpdate = c.BeforeUpdate
	o.AfterCreate = c.AfterCreate
	o.AfterUpdate = c.AfterUpdate
	o.AfterDelete = c.AfterDelete
	o.OwnerCheck = c.OwnerCheck

	userOnMutate := c.OnMutate
	if c.Cache != nil && c.CacheBustPattern != "" {
		cacheInst := c.Cache
		pattern := c.CacheBustPattern
		o.OnMutate = func(ctx context.Context, action string, entityOrID any) error {
			Bust(cacheInst, pattern)
			if userOnMutate != nil {
				return userOnMutate(ctx, action, entityOrID)
			}
			return nil
		}
	} else {
		o.OnMutate = userOnMutate
	}
	o.TenantColumn = c.TenantColumn
	o.TenantScope = c.TenantScope

	return o
}

func (c *ResourceConfig) routeMiddleware(method string) []Handler {
	if c == nil {
		return nil
	}
	n := len(c.Middleware)
	switch method {
	case "GET", "HEAD":
		n += len(c.ReadMiddleware)
	default:
		n += len(c.WriteMiddleware)
	}
	if n == 0 {
		return nil
	}
	out := make([]Handler, 0, n)
	out = append(out, c.Middleware...)
	switch method {
	case "GET", "HEAD":
		out = append(out, c.ReadMiddleware...)
	default:
		out = append(out, c.WriteMiddleware...)
	}
	return out
}

func (c *ResourceConfig) extraMiddleware(write bool) []Handler {
	if c == nil {
		return nil
	}
	out := append([]Handler{}, c.Middleware...)
	if write {
		out = append(out, c.WriteMiddleware...)
	} else {
		out = append(out, c.ReadMiddleware...)
	}
	return out
}
