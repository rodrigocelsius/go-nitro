package nitro

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

func allowedResourceActions(cfg *ResourceConfig) map[string]bool {
	all := []string{ActionList, ActionGet, ActionCreate, ActionUpdate, ActionDelete}
	allowed := make(map[string]bool, len(all))
	if cfg != nil && len(cfg.Only) > 0 {
		for _, a := range cfg.Only {
			allowed[strings.ToLower(strings.TrimSpace(a))] = true
		}
		return allowed
	}
	for _, a := range all {
		allowed[a] = true
	}
	if cfg != nil {
		for _, a := range cfg.Except {
			delete(allowed, strings.ToLower(strings.TrimSpace(a)))
		}
	}
	return allowed
}

func resourceActionName(method, path, prefix string) string {
	method = strings.ToUpper(method)
	p := strings.TrimSuffix(path, "/")
	pref := strings.TrimSuffix(prefix, "/")
	switch {
	case method == "GET" && p == pref:
		return ActionList
	case method == "GET" && strings.HasSuffix(p, "/:id"):
		return ActionGet
	case method == "POST" && p == pref:
		return ActionCreate
	case method == "PATCH" && strings.HasSuffix(p, "/:id"):
		return ActionUpdate
	case method == "PUT" && strings.HasSuffix(p, "/:id"):
		return ActionUpdate
	case method == "DELETE" && strings.HasSuffix(p, "/:id"):
		return ActionDelete
	default:
		return ""
	}
}

func filterResourceRoutes(prefix string, routes []usecases.ResourceRoute, cfg *ResourceConfig) []usecases.ResourceRoute {
	allowed := allowedResourceActions(cfg)
	out := make([]usecases.ResourceRoute, 0, len(routes))
	for _, r := range routes {
		action := resourceActionName(r.Method, r.Path, prefix)
		if action == "" {
			out = append(out, r)
			continue
		}
		if !allowed[action] {
			continue
		}
		if cfg != nil && cfg.Replace != nil {
			if h, ok := cfg.Replace[action]; ok && h != nil {
				r.Handler = wrapHandler(h)
			}
		}
		out = append(out, r)
	}
	return out
}
