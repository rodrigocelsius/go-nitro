package nitro

import (
	"context"
	"fmt"
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

type commandKind int

const (
	commandItem       commandKind = iota // POST /:id/{name}
	commandCollection                    // POST /{name}
)

func mountBusinessRoutes(
	pfx string,
	resCfg *ResourceConfig,
	_ reflect.Type,
	repo domain.ResourceRepository,
	add func(method, path string, h domain.Handler),
) {
	if resCfg == nil {
		return
	}
	for _, q := range resCfg.Queries {
		path := joinResourcePath(pfx, "/queries/"+q.Name)
		h := wrapHandler(makeQueryHandler(q, resCfg))
		add("GET", path, h)
	}
	for _, cmd := range resCfg.Commands {
		kind, _, _ := analyzeHandlerFn(cmd.fn.Type())
		path := joinResourcePath(pfx, "/"+cmd.Name)
		if kind == commandItem {
			path = joinResourcePath(pfx, "/:id/"+cmd.Name)
		}
		h := wrapHandler(makeCommandHandler(cmd, resCfg, repo))
		add("POST", path, h)
	}
}

func makeQueryHandler(q ResourceQuery, cfg *ResourceConfig) Handler {
	return func(c *Ctx) error {
		page := PageFrom(c, defaultPerPage(cfg))
		results, err := callHandler(c, q.fn, []reflect.Value{
			reflect.ValueOf(c.Context()),
			reflect.ValueOf(page),
		})
		if err != nil {
			return err
		}
		if len(results) == 0 {
			return c.OK(nil)
		}
		return c.OK(results[0].Interface())
	}
}

func makeCommandHandler(
	cmd ResourceCommand,
	cfg *ResourceConfig,
	repo domain.ResourceRepository,
) Handler {
	return func(c *Ctx) error {
		kind, inputType, _ := analyzeHandlerFn(cmd.fn.Type())
		ctx := c.Context()

		var entity any
		var id string
		if kind == commandItem {
			id = c.Param("id")
			if id == "" {
				return BadRequest("missing id parameter")
			}
			var err error
			entity, err = repo.FindByID(ctx, id)
			if err != nil {
				return mapCommandRepoError(err)
			}
			if entity == nil {
				return NotFound("resource not found")
			}
			if cfg != nil && cfg.TenantScope != nil {
				tid, ok := cfg.TenantScope(&c.Ctx)
				col := cfg.TenantColumn
				if col == "" {
					col = "tenant_id"
				}
				if !ok || !usecases.EntityBelongsToTenant(entity, col, tid) {
					return NotFound("resource not found")
				}
			}
			if cfg != nil && cfg.OwnerCheck != nil && !cfg.OwnerCheck(&c.Ctx, entity) {
				return NotFound("resource not found")
			}
			if err := runResourceRules(entity, cfg.Rules); err != nil {
				return err
			}
			if cmd.when.IsValid() {
				if callPredicate(cmd.when, entity) && len(cmd.roles) > 0 {
					if err := requireRolesOnCtx(c, cmd.roles); err != nil {
						return err
					}
				}
			} else if len(cmd.roles) > 0 {
				if err := requireRolesOnCtx(c, cmd.roles); err != nil {
					return err
				}
			}
		}

		args := []reflect.Value{reflect.ValueOf(ctx)}
		if kind == commandItem {
			args = append(args, reflect.ValueOf(id))
		}

		if inputType != nil {
			in, err := parseCommandInput(c, inputType)
			if err != nil {
				return err
			}
			args = append(args, reflect.ValueOf(in))
		}

		results, err := callHandler(c, cmd.fn, args)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			return c.OK(nil)
		}
		if kind == commandCollection {
			return c.Created(results[0].Interface())
		}
		return c.OK(results[0].Interface())
	}
}

func analyzeHandlerFn(ft reflect.Type) (kind commandKind, inputType reflect.Type, err error) {
	if ft.NumIn() < 1 || ft.In(0) != reflect.TypeOf((*context.Context)(nil)).Elem() {
		return 0, nil, fmt.Errorf("first parameter must be context.Context")
	}
	if ft.NumOut() < 1 || ft.Out(ft.NumOut()-1) != reflect.TypeOf((*error)(nil)).Elem() {
		return 0, nil, fmt.Errorf("last return must be error")
	}
	switch ft.NumIn() {
	case 1:
		return commandCollection, nil, nil
	case 2:
		if ft.In(1).Kind() == reflect.String {
			return commandItem, nil, nil
		}
		return commandCollection, ft.In(1), nil
	case 3:
		if ft.In(1).Kind() != reflect.String {
			return 0, nil, fmt.Errorf("item command second parameter must be id string")
		}
		return commandItem, ft.In(2), nil
	default:
		return 0, nil, fmt.Errorf("unsupported handler arity %d", ft.NumIn())
	}
}

func parseCommandInput(c *Ctx, inputType reflect.Type) (any, error) {
	ptr := reflect.New(inputType)
	if err := bindAndValidateCommandBody(c, ptr.Interface()); err != nil {
		return nil, err
	}
	return ptr.Elem().Interface(), nil
}

func bindAndValidateCommandBody(c *Ctx, v any) error {
	if err := c.Bind(v); err != nil {
		return BadRequest("invalid json body")
	}
	if errs := Validate(v); errs.HasErrors() {
		return NewValidationFailed(errs)
	}
	return nil
}

func callHandler(c *Ctx, fn reflect.Value, args []reflect.Value) ([]reflect.Value, error) {
	out := fn.Call(args)
	if len(out) == 0 {
		return nil, nil
	}
	last := out[len(out)-1]
	if last.IsValid() && last.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		if !last.IsNil() {
			return nil, last.Interface().(error)
		}
		out = out[:len(out)-1]
	}
	return out, nil
}

func runResourceRules(entity any, rules []ResourceRule) error {
	for _, rule := range rules {
		if !rule.fn.IsValid() {
			continue
		}
		ft := rule.fn.Type()
		if ft.NumIn() != 1 {
			continue
		}
		arg, err := coerceEntityArg(entity, ft.In(0))
		if err != nil {
			return err
		}
		out := rule.fn.Call([]reflect.Value{arg})
		if len(out) > 0 && !out[0].IsNil() {
			if err, ok := out[0].Interface().(error); ok {
				return err
			}
		}
	}
	return nil
}

func callPredicate(fn reflect.Value, entity any) bool {
	ft := fn.Type()
	if ft.NumIn() != 1 {
		return false
	}
	arg, err := coerceEntityArg(entity, ft.In(0))
	if err != nil {
		return false
	}
	out := fn.Call([]reflect.Value{arg})
	if len(out) == 0 || out[0].Kind() != reflect.Bool {
		return false
	}
	return out[0].Bool()
}

func coerceEntityArg(entity any, want reflect.Type) (reflect.Value, error) {
	ev := reflect.ValueOf(entity)
	for ev.Kind() == reflect.Pointer {
		if ev.IsNil() {
			return reflect.Value{}, fmt.Errorf("nil entity")
		}
		ev = ev.Elem()
	}
	if want.Kind() == reflect.Pointer {
		if ev.Type().AssignableTo(want.Elem()) {
			ptr := reflect.New(ev.Type())
			ptr.Elem().Set(ev)
			return ptr, nil
		}
	}
	if ev.Type().AssignableTo(want) {
		return ev, nil
	}
	if ev.Type().ConvertibleTo(want) {
		return ev.Convert(want), nil
	}
	if want.Kind() == reflect.Pointer && ev.Type().AssignableTo(want.Elem()) {
		ptr := reflect.New(want.Elem())
		ptr.Elem().Set(ev)
		return ptr, nil
	}
	return reflect.Value{}, fmt.Errorf("entity type mismatch")
}

func requireRolesOnCtx(c *Ctx, roles []string) error {
	userRoles, ok := c.Get("roles").([]string)
	if !ok || len(userRoles) == 0 {
		return Forbidden("insufficient permissions")
	}
	for _, userRole := range userRoles {
		for _, req := range roles {
			if userRole == req {
				return nil
			}
		}
	}
	return Forbidden("insufficient permissions")
}

func mapCommandRepoError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(HTTPError); ok {
		return err
	}
	return NotFound("resource not found")
}

func defaultPerPage(cfg *ResourceConfig) int {
	if cfg != nil && cfg.DefaultPerPage > 0 {
		return cfg.DefaultPerPage
	}
	return DefaultListPerPage
}
