package nitro

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

// forceProdResourceAuthPanicForTest lets unit tests exercise the production
// panic path even though testing.Testing() is true under `go test`.
var forceProdResourceAuthPanicForTest bool

// warnIfResourceHasNoAuthMiddleware enforces V-04: app.Resource does not apply
// auth by default. In EnvProduction, an unprotected Resource (no Middleware /
// ReadMiddleware / WriteMiddleware and Public != true) records a configuration error (Listen refuses to start) unless
// Config.Security.AllowUnauthenticatedResources is set. In other envs it only
// warns on stderr. Set Public:true for intentional open CRUD, or call
// app.SetDefaultResourceMiddleware(JWT…) so Resources inherit auth.
func warnIfResourceHasNoAuthMiddleware(a *App, prefix string, resCfg *ResourceConfig) {
	if a == nil {
		return
	}
	if resCfg != nil && resCfg.Public {
		return
	}
	if resCfg != nil && (len(resCfg.Middleware) > 0 || len(resCfg.ReadMiddleware) > 0 || len(resCfg.WriteMiddleware) > 0) {
		return
	}
	msg := fmt.Sprintf("nitro: Resource %q has no Middleware/ReadMiddleware/WriteMiddleware — every CRUD route is public; set ResourceConfig.Public=true if intentional, or app.SetDefaultResourceMiddleware(JWT…); see vuln.md V-04", prefix)
	if a.cfg.Env != EnvProduction {
		fmt.Fprintf(os.Stderr, "nitro: warning: %s\n", msg)
		return
	}
	if a.cfg.Security.AllowUnauthenticatedResources {
		fmt.Fprintf(os.Stderr, "nitro: warning: %s (AllowUnauthenticatedResources=true)\n", msg)
		return
	}
	// Under `go test`, keep a warning so unit suites using New() (EnvProduction zero
	// value) stay focused; production binaries refuse to start (V-04).
	if testing.Testing() && !forceProdResourceAuthPanicForTest {
		fmt.Fprintf(os.Stderr, "nitro: warning: %s (test process)\n", msg)
		return
	}
	a.fail(errors.New(msg))
}

// applyDefaultResourceMiddleware prepends App-level Resource middleware unless Public.
func (a *App) applyDefaultResourceMiddleware(resCfg *ResourceConfig) *ResourceConfig {
	if a == nil || len(a.defaultResourceMiddleware) == 0 {
		return resCfg
	}
	if resCfg != nil && resCfg.Public {
		return resCfg
	}
	if resCfg == nil {
		resCfg = &ResourceConfig{}
	}
	resCfg.Middleware = append(append([]Handler(nil), a.defaultResourceMiddleware...), resCfg.Middleware...)
	return resCfg
}

// resourceOpts is the engine-level configuration derived from *ResourceConfig
// (the only public Resource configuration type).
type resourceOpts = usecases.ResourceOpts

// FilterRule maps a query param → field + operator (resourceOpts.FilterRules).
type FilterRule = usecases.FilterRule

// ActiveStatus is the conventional business-status value "active".
// Independent of soft delete (DeletedAt) — Store already excludes soft-deleted rows.
const ActiveStatus = "active"

// FilterActive maps the query param (default "status") to the status field with Op eq.
// The value comes from the query (?status=active). Soft delete remains automatic in Store.
//
//	FilterRules: []nitro.FilterRule{nitro.FilterActive()}
func FilterActive(param ...string) FilterRule {
	p := "status"
	if len(param) > 0 && param[0] != "" {
		p = param[0]
	}
	return FilterRule{Param: p, Field: "status", Op: "eq"}
}

// Resource registers CRUD routes automatically for a model or controller.
//
//	app.Resource("/users", &User{}, repo)           // explicit path
//	app.Resource(&Product{}, store)                 // path = /products (plural)
//	app.Resource(&Product{}, store, &ResourceConfig{}) // filters via blinq:"index"
//
// first: path string OR model pointer. Remaining args: ResourceDB, repo, config.
func (a *App) Resource(first any, args ...any) *App {
	prefix, modelArgs := splitResourceFirst(first, args)
	model, rest := resolveResourceArgs(a, prefix, modelArgs)
	if model == nil {
		return a.fail(errors.New("nitro: Resource requires a model pointer (e.g. &User{})"))
	}
	if prefix == "" {
		prefix = "/" + resourcePathFromModel(model)
	} else if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	var repos []domain.ResourceRepository
	var resCfg *ResourceConfig
	for i, arg := range rest {
		switch v := arg.(type) {
		case domain.ResourceRepository:
			repos = append(repos, v)
		case domain.AsResourceRepository:
			repos = append(repos, v.AsResourceRepository())
		case *ResourceConfig:
			if resCfg != nil {
				return a.fail(errors.New("nitro: Resource: pass only one *ResourceConfig"))
			}
			resCfg = v
		case ResourceConfig:
			if resCfg != nil {
				return a.fail(errors.New("nitro: Resource: pass only one *ResourceConfig"))
			}
			cp := v
			resCfg = &cp
		case nil:
			// ignore
		default:
			return a.fail(fmt.Errorf("nitro: Resource %q: argument #%d has type %T (pass a ResourceRepository, nitro.AdaptRepo(typedRepo) or *ResourceConfig; for a typed repo you can also call nitro.Resource(app, path, repo))", prefix, i+3, arg))
		}
	}
	if len(repos) == 0 {
		if repo := a.repoForModel(model, prefix); repo != nil {
			repos = append(repos, repo)
		}
	}
	resCfg = a.applyResourceCacheDefaults(prefix, resCfg)
	resCfg = a.applyDefaultResourceMiddleware(resCfg)
	resCfg = a.applyTenantConvention(model, resCfg)
	if resCfg != nil {
		applyInferredFilters(model, resCfg, nil)
	}

	// Compute opts exactly once after all ResourceConfig merges.
	var opts resourceOpts
	if resCfg != nil {
		opts = resCfg.toOpts()
	} else {
		opts = usecases.DefaultResourceOpts()
		applyInferredFilters(model, nil, &opts)
	}
	return a.registerResource(prefix, model, repos, opts, resCfg)
}

func splitResourceFirst(first any, args []any) (prefix string, modelArgs []any) {
	switch v := first.(type) {
	case string:
		return v, args
	case nil:
		return "", args
	default:
		out := make([]any, 0, 1+len(args))
		out = append(out, first)
		out = append(out, args...)
		return "", out
	}
}

// applyResourceCacheDefaults fills Cache / CacheBustPattern / NoCache from the
// App cache when ResourceConfig does not set those fields.
// With an App cache configured, the in-memory L1 is disabled (NoCache) unless CacheTTL is set.
func (a *App) applyResourceCacheDefaults(prefix string, cfg *ResourceConfig) *ResourceConfig {
	if a == nil || a.cache == nil {
		return cfg
	}
	if cfg == nil {
		cfg = &ResourceConfig{}
	}
	if cfg.Cache == nil {
		cfg.Cache = a.cache
	}
	if cfg.CacheBustPattern == "" {
		cfg.CacheBustPattern = resourceCacheBustPattern(prefix)
	}
	if !cfg.NoCache && cfg.CacheTTL == 0 {
		cfg.NoCache = true
	}
	return cfg
}

// resourceCacheBustPattern derives "drivers:*" from "/api/drivers".
func resourceCacheBustPattern(prefix string) string {
	p := strings.Trim(prefix, "/")
	if p == "" {
		return "resource:*"
	}
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	if p == "" {
		return "resource:*"
	}
	return p + ":*"
}

// ResourceDB creates a ResourceRepository from the model (e.g. *pgx.DB).
type ResourceDB = domain.ResourceDB

// Resource registers typed CRUD routes without AdaptRepo.
//
//	nitro.Resource(app, "/todos", repo)
//	nitro.Resource(app, "/todos", repo, &nitro.ResourceConfig{NoCache: true})
//	nitro.Resource(app, "/todos", repo, &nitro.ResourceConfig{CacheTTL: time.Minute})
func Resource[T any](a *App, prefix string, repo TypedRepository[T], args ...any) *App {
	var zero T
	all := make([]any, 0, 2+len(args))
	all = append(all, &zero, AdaptRepo(repo))
	all = append(all, args...)
	return a.Resource(prefix, all...)
}

func (a *App) registerResource(prefix string, controller any, repo []domain.ResourceRepository, opts resourceOpts, resCfg *ResourceConfig) *App {
	warnIfResourceHasNoAuthMiddleware(a, prefix, resCfg)
	mount := func(pfx string) {
		add := func(method, path string, h domain.Handler) {
			mws := wrapHandlers(resCfg.routeMiddleware(method))
			switch method {
			case "GET":
				a.inner.Get(path, h, mws...)
			case "POST":
				a.inner.Post(path, h, mws...)
			case "PUT":
				a.inner.Put(path, h, mws...)
			case "PATCH":
				a.inner.Patch(path, h, mws...)
			case "DELETE":
				a.inner.Delete(path, h, mws...)
			}
		}

		// Extra routes + business Commands/Queries before :id CRUD routes.
		if resCfg != nil {
			var r0 domain.ResourceRepository
			if len(repo) > 0 {
				r0 = repo[0]
			}
			modelType := reflect.TypeOf(controller)
			if modelType != nil && modelType.Kind() == reflect.Pointer {
				modelType = modelType.Elem()
			}
			mountBusinessRoutes(pfx, resCfg, modelType, r0, add)
			for _, er := range resCfg.ExtraRoutes {
				if er.Handler == nil || er.Method == "" {
					continue
				}
				path := joinResourcePath(pfx, er.Path)
				mws := wrapHandlers(resCfg.extraMiddleware(er.Write || isWriteMethod(er.Method)))
				h := wrapHandler(er.Handler)
				switch strings.ToUpper(er.Method) {
				case "GET":
					a.inner.Get(path, h, mws...)
				case "POST":
					a.inner.Post(path, h, mws...)
				case "PUT":
					a.inner.Put(path, h, mws...)
				case "PATCH":
					a.inner.Patch(path, h, mws...)
				case "DELETE":
					a.inner.Delete(path, h, mws...)
				}
			}
		}

		routes := usecases.ScanResource(pfx, controller)
		if len(routes) > 0 {
			routes = filterResourceRoutes(pfx, routes, resCfg)
			for _, r := range routes {
				add(r.Method, r.Path, r.Handler)
			}
			return
		}

		if len(repo) == 0 || repo[0] == nil {
			a.fail(fmt.Errorf("nitro: Resource %q with a struct model requires a ResourceRepository; pass one: app.Resource(\"/users\", &User{}, myRepo)", pfx))
			return
		}

		modelType := reflect.TypeOf(controller)
		cached := applyResourceCache(repo[0], opts)
		routes = usecases.GenerateResourceRoutes(pfx, modelType, cached, opts)
		routes = filterResourceRoutes(pfx, routes, resCfg)
		for _, r := range routes {
			add(r.Method, r.Path, r.Handler)
		}
	}

	mount(prefix)
	if resCfg != nil {
		for _, alias := range resCfg.Aliases {
			if alias != "" && alias != prefix {
				mount(alias)
			}
		}
	}
	return a
}

func isWriteMethod(m string) bool {
	switch strings.ToUpper(m) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}
