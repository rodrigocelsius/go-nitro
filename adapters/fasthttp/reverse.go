package fasthttp

import (
	"fmt"
	"strings"
)

// GetRouteURL rebuilds the path of a named route, replacing
// parameters (`:id`, `*filepath`) with values from params.
//
//	app.Get("/users/:id", h).Name("user.show")
//	app.GetRouteURL("user.show", map[string]any{"id": 42}) // "/users/42"
func (a *App) GetRouteURL(name string, params ...map[string]any) string {
	path, ok := a.namedPaths[name]
	if !ok {
		return ""
	}
	var p map[string]any
	if len(params) > 0 {
		p = params[0]
	}
	return fillRouteParams(path, p)
}

// URL is an alias of GetRouteURL.
func (a *App) URL(name string, params ...map[string]any) string {
	return a.GetRouteURL(name, params...)
}

// GetRouteURL rebuilds a named route URL from the context.
func (c *FastCtx) GetRouteURL(name string, params ...map[string]any) string {
	if c.app == nil {
		return ""
	}
	return c.app.GetRouteURL(name, params...)
}

// URL is an alias of GetRouteURL on the context.
func (c *FastCtx) URL(name string, params ...map[string]any) string {
	return c.GetRouteURL(name, params...)
}

func fillRouteParams(path string, params map[string]any) string {
	if params == nil || len(params) == 0 {
		return path
	}
	segs := strings.Split(path, "/")
	for i, seg := range segs {
		if seg == "" {
			continue
		}
		switch seg[0] {
		case ':':
			key := seg[1:]
			if v, ok := params[key]; ok {
				segs[i] = fmt.Sprint(v)
			}
		case '*':
			key := seg[1:]
			if v, ok := params[key]; ok {
				segs[i] = strings.TrimPrefix(fmt.Sprint(v), "/")
			}
		}
	}
	return strings.Join(segs, "/")
}
