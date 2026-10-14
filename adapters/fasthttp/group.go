package fasthttp

import "github.com/rodrigocelsius/go-nitro/domain/v2"

// Group represents a subset of routes with a shared prefix and middlewares.
// Organizes routes by domain (e.g. /api/v1/users, /api/v1/posts).
type Group struct {
	app         *App
	prefix      string
	middlewares []domain.Handler
}

// Group creates a new sub-group with prefix and middlewares.
// Prefixes accumulate: app.Group("/api").Group("/v1") → "/api/v1"
func (a *App) Group(prefix string, mws ...domain.Handler) *Group {
	return &Group{
		app:         a,
		prefix:      prefix,
		middlewares: mws,
	}
}

// Group creates a sub-group inside an existing group.
// Prefixes accumulate: group.Group("/v1") → previous prefix + "/v1"
func (g *Group) Group(prefix string, mws ...domain.Handler) *Group {
	combined := make([]domain.Handler, len(g.middlewares)+len(mws))
	copy(combined, g.middlewares)
	copy(combined[len(g.middlewares):], mws)
	return &Group{
		app:         g.app,
		prefix:      g.prefix + prefix,
		middlewares: combined,
	}
}

// Get registers a GET handler on the group.
func (g *Group) Get(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return g.add("GET", path, handler, mws)
}

// Post registers a POST handler on the group.
func (g *Group) Post(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return g.add("POST", path, handler, mws)
}

// Put registers a PUT handler on the group.
func (g *Group) Put(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return g.add("PUT", path, handler, mws)
}

// Delete registers a DELETE handler on the group.
func (g *Group) Delete(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return g.add("DELETE", path, handler, mws)
}

// Patch registers a PATCH handler on the group.
func (g *Group) Patch(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return g.add("PATCH", path, handler, mws)
}

// Head registers a HEAD handler on the group.
func (g *Group) Head(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return g.add("HEAD", path, handler, mws)
}

// Options registers an OPTIONS handler on the group.
func (g *Group) Options(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return g.add("OPTIONS", path, handler, mws)
}

// Use adds middlewares to the group. They apply to all group routes.
func (g *Group) Use(mws ...domain.Handler) *Group {
	g.middlewares = append(g.middlewares, mws...)
	return g
}

// Prefix returns the accumulated path prefix for this group.
func (g *Group) Prefix() string { return g.prefix }

// Middlewares returns a copy of the group middleware chain.
func (g *Group) Middlewares() []domain.Handler {
	if len(g.middlewares) == 0 {
		return nil
	}
	out := make([]domain.Handler, len(g.middlewares))
	copy(out, g.middlewares)
	return out
}

// add registers a group route, concatenating prefix and merging middlewares.
func (g *Group) add(method, path string, handler domain.Handler, routeMws []domain.Handler) *Route {
	// Merge: group middlewares + route middlewares
	merged := make([]domain.Handler, 0, len(g.middlewares)+len(routeMws))
	merged = append(merged, g.middlewares...)
	merged = append(merged, routeMws...)

	return g.app.add(method, g.prefix+path, handler, merged)
}
