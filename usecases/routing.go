package usecases

import "github.com/rodrigocelsius/go-nitro/domain/v2"

// MatchPathLinear is the naive path comparison (fallback).
// It compares the route pattern with the request path, supporting
// dynamic parameters (:param). Used when Config.UseLinearRouter=true.
func MatchPathLinear(pattern, path string, c *domain.Ctx) bool {
	pi, pj := 0, 0
	for pi < len(pattern) && pj < len(path) {
		if pattern[pi] == ':' {
			start := pi + 1
			end := start
			for end < len(pattern) && pattern[end] != '/' {
				end++
			}
			paramName := pattern[start:end]

			vstart := pj
			for pj < len(path) && path[pj] != '/' {
				pj++
			}
			c.SetParam(paramName, path[vstart:pj])
			pi = end
			continue
		}
		if pattern[pi] != path[pj] {
			return false
		}
		pi++
		pj++
	}
	return pi == len(pattern) && pj == len(path)
}

// MatchLinear looks up a route in a LinearRoutes list using
// linear comparison. Used as fallback when Config.UseLinearRouter=true.
func MatchLinear(routes []domain.LinearRoute, method, path string, c *domain.Ctx) (domain.Handler, bool) {
	for _, r := range routes {
		if r.Method != method {
			continue
		}
		params := c.ParamCount
		matched := MatchPathLinear(r.Path, path, c)
		if !matched && len(r.Path) > 1 && r.Path[len(r.Path)-1] == '/' {
			// The trailing slash of a registered route is optional (same as
			// the radix router): /users/ also answers /users.
			c.ParamCount = params
			matched = MatchPathLinear(r.Path[:len(r.Path)-1], path, c)
		}
		if matched {
			// The route template (not the concrete path) is what tracing and
			// metrics label requests with, same as the radix router.
			c.SetRoute(r.Path)
			return r.Handler, true
		}
		// A pattern that failed halfway may have captured parameters.
		c.ParamCount = params
	}
	return nil, false
}
