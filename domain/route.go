package domain

// RouteInfo describes a registered route, for introspection (used
// by compile-time compiled-router generation). Rebuilt from the
// radix tree, so it reflects the REAL App state after all
// Get/Post/etc. calls.
type RouteInfo struct {
	Method string
	Path   string
	// Name is the explicit name assigned via Route.Name, or "" if the
	// route was not named. Unnamed routes are ignored by the compiled
	// router generator, but still work normally.
	Name string
}

// LinearRoute is a route entry for the linear-search implementation
// (fallback). Used when Config.UseLinearRouter=true.
type LinearRoute struct {
	Method  string
	Path    string
	Handler Handler
}
