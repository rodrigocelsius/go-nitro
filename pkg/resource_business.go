package nitro

import "reflect"

// Page holds pagination from ?page= and ?per_page= (defaults 1 and 20).
type Page struct {
	Number  int
	PerPage int
}

// PageFrom reads pagination query params from the request context.
func PageFrom(c *Ctx, defaultPerPage int) Page {
	if defaultPerPage <= 0 {
		defaultPerPage = DefaultListPerPage
	}
	return Page{
		Number:  c.QueryIntDefault("page", 1),
		PerPage: c.QueryIntDefault("per_page", defaultPerPage),
	}
}

// DefaultListPerPage is the default page size for PageFrom and Resource list.
const DefaultListPerPage = 20

// ResourceRule validates an entity before a resource Command runs.
type ResourceRule struct {
	fn reflect.Value
}

// ResourceCommand is a business action on a Resource (item or collection).
type ResourceCommand struct {
	Name  string
	fn    reflect.Value
	roles []string
	when  reflect.Value // func(*T) bool or func(T) bool
}

// ResourceQuery is a named read-only route under /queries/{name}.
type ResourceQuery struct {
	Name string
	fn   reflect.Value
}

// Rules builds global rules for ResourceConfig (evaluated before item Commands).
func Rules[T any](fns ...func(*T) error) []ResourceRule {
	out := make([]ResourceRule, 0, len(fns))
	for _, fn := range fns {
		if fn == nil {
			panic("nitro: Rules requires non-nil functions")
		}
		out = append(out, ResourceRule{fn: reflect.ValueOf(fn)})
	}
	return out
}

// Cmd registers a business Command. Name becomes the URL segment.
//
// Item (POST /:id/{name}): func(ctx, id string) (T, error) or func(ctx, id, in Input) (T, error).
// Collection (POST /{name}): func(ctx, in Input) (T, error) or func(ctx) (T, error).
func Cmd(name string, fn any) ResourceCommand {
	if name == "" {
		panic("nitro: Cmd requires a non-empty name")
	}
	if fn == nil {
		panic("nitro: Cmd requires a non-nil function")
	}
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		panic("nitro: Cmd requires a function")
	}
	return ResourceCommand{Name: name, fn: v}
}

// Commands builds a Command slice for ResourceConfig.
func Commands(cmds ...ResourceCommand) []ResourceCommand {
	return append([]ResourceCommand(nil), cmds...)
}

// Roles requires JWT roles when the Command runs (use with JWTMiddleware).
func (c ResourceCommand) Roles(roles ...string) ResourceCommand {
	c.roles = append([]string(nil), roles...)
	return c
}

// When applies Roles only when the predicate is true on the loaded entity.
func (c ResourceCommand) When(fn any) ResourceCommand {
	if fn == nil {
		panic("nitro: When requires a non-nil function")
	}
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		panic("nitro: When requires a function")
	}
	c.when = v
	return c
}

// Q registers a named query (GET /queries/{name}).
//
//	func(ctx context.Context, page Page) (any, error)
func Q(name string, fn any) ResourceQuery {
	if name == "" {
		panic("nitro: Q requires a non-empty name")
	}
	if fn == nil {
		panic("nitro: Q requires a non-nil function")
	}
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		panic("nitro: Q requires a function")
	}
	return ResourceQuery{Name: name, fn: v}
}

// Queries builds a Query slice for ResourceConfig.
func Queries(qs ...ResourceQuery) []ResourceQuery {
	return append([]ResourceQuery(nil), qs...)
}
