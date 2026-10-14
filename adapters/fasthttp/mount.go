package fasthttp

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

var mountMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

// Mount mounts a sub-App under the given prefix.
// Sub-app routes and middlewares are preserved; the path is rewritten
// by stripping the prefix before forwarding to the sub-app Handler.
//
//	v1 := fasthttp.New()
//	v1.Get("/users", listUsers)
//	app.Mount("/api/v1", v1) // GET /api/v1/users
func (a *App) Mount(prefix string, sub *App) *App {
	if sub == nil {
		return a
	}
	prefix = normalizeMountPrefix(prefix)
	subHandler := sub.Handler()

	h := func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return domain.ErrNotFound
		}
		fctx := fc.FasthttpCtx()
		path := b2s(fctx.Path())
		stripped := stripMountPrefix(path, prefix)

		origURI := append([]byte(nil), fctx.Request.RequestURI()...)
		qs := fctx.URI().QueryString()
		if len(qs) > 0 {
			fctx.Request.SetRequestURI(stripped + "?" + b2s(qs))
		} else {
			fctx.Request.SetRequestURI(stripped)
		}
		subHandler(fctx)
		fctx.Request.SetRequestURIBytes(origURI)
		return nil
	}

	for _, method := range mountMethods {
		a.add(method, prefix, h, nil)
		if prefix != "/" {
			a.add(method, prefix+"/*filepath", h, nil)
		}
	}

	// Reverse routing: expose sub-app names with a prefixed path.
	for name, path := range sub.namedPaths {
		a.namedPaths[name] = joinMountPath(prefix, path)
	}
	for key, name := range sub.routeNames {
		method, path, ok := strings.Cut(key, " ")
		if !ok {
			continue
		}
		a.routeNames[method+" "+joinMountPath(prefix, path)] = name
	}
	return a
}

func normalizeMountPrefix(prefix string) string {
	if prefix == "" || prefix == "/" {
		return "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimRight(prefix, "/")
}

func stripMountPrefix(path, prefix string) string {
	if prefix == "/" {
		if path == "" {
			return "/"
		}
		return path
	}
	if path == prefix {
		return "/"
	}
	stripped := strings.TrimPrefix(path, prefix)
	if stripped == "" {
		return "/"
	}
	if !strings.HasPrefix(stripped, "/") {
		stripped = "/" + stripped
	}
	return stripped
}

func joinMountPath(prefix, path string) string {
	if prefix == "/" {
		if path == "" {
			return "/"
		}
		if !strings.HasPrefix(path, "/") {
			return "/" + path
		}
		return path
	}
	if path == "" || path == "/" {
		return prefix
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return prefix + path
}
