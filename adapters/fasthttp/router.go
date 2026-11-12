package fasthttp

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// radixNode is a node of the segment tree used for all routes (static
// and dynamic, unified). A node has any number of static children and at most
// one parameter child and one catch-all child: parameter NAMES belong to the
// route (paramNames on the terminal node), not to the tree edge, so
// /users/:id and /users/:name/posts share the same edge.
type radixNode struct {
	segment    string
	handler    domain.Handler
	routePath  string   // registered pattern, e.g. /users/:id
	paramNames []string // names of the dynamic segments of routePath, in order
	// trailingSlash is set when routePath ends in "/" (other than the root).
	// The slash of such a route is optional: /users/ (what
	// Group("/users").Get("/") registers) answers /users and /users/.
	trailingSlash bool
	children      []*radixNode
	param         *radixNode
	catchAll      *radixNode
}

// RadixRouter implements ports.Router using a segment tree
// per HTTP method. Structure validated by benchmark for high performance.
// Includes an O(1) fast path for static routes via map.
//
// The tree is immutable after registration: Lookup never writes to it, so it
// is safe for concurrent use by every request goroutine.
type RadixRouter struct {
	methodRoot *radixNode
	// staticRoutes is the O(1) fast path for static routes (no parameters).
	// Two-level map: method → path → Handler. Uses b2s for zero-allocation lookup.
	staticRoutes map[string]map[string]domain.Handler
}

// NewRadixRouter creates a new RadixRouter.
func NewRadixRouter() *RadixRouter {
	return &RadixRouter{
		methodRoot:   &radixNode{},
		staticRoutes: make(map[string]map[string]domain.Handler),
	}
}

// Add registers a route on the tree for the matching method.
// If the route is static (no parameters), it is also added to the fast-path map.
// Supports:
//   - Static routes: /users
//   - Named parameters: /users/:id
//   - Catch-all: /static/*filepath (must be the last segment)
func (rt *RadixRouter) Add(method, path string, h domain.Handler) {
	node := rt.methodNode(method)
	var names []string
	dynamic := false
	for _, seg := range splitSegments(path) {
		switch seg[0] {
		case ':':
			dynamic = true
			names = append(names, seg[1:])
			if node.param == nil {
				node.param = &radixNode{segment: seg}
			}
			node = node.param
		case '*':
			dynamic = true
			names = append(names, seg[1:])
			if node.catchAll == nil {
				node.catchAll = &radixNode{segment: seg}
			}
			node = node.catchAll
		default:
			var found *radixNode
			for _, child := range node.children {
				if child.segment == seg {
					found = child
					break
				}
			}
			if found == nil {
				found = &radixNode{segment: seg}
				node.children = append(node.children, found)
			}
			node = found
		}
	}
	node.handler = h
	node.routePath = path
	node.paramNames = names
	node.trailingSlash = len(path) > 1 && path[len(path)-1] == '/'

	// Fast path: static routes are also resolved by an exact-match map.
	if !dynamic {
		mm, ok := rt.staticRoutes[method]
		if !ok {
			mm = make(map[string]domain.Handler)
			rt.staticRoutes[method] = mm
		}
		mm[path] = h
	}
}

// Lookup resolves method+path (both []byte) to a Handler.
// Fast path: checks the O(1) static-route map before the tree.
func (rt *RadixRouter) Lookup(method, path []byte, c *domain.Ctx) (domain.Handler, bool) {
	// Fast path O(1): static-route map (zero-allocation via b2s)
	if mm, ok := rt.staticRoutes[b2s(method)]; ok {
		if h, ok := mm[b2s(path)]; ok {
			c.SetRoute(b2s(path))
			return h, true
		}
	}

	// Fallback: tree for dynamic routes
	methodRoot := rt.findMethodNodeBytes(method)
	if methodRoot == nil {
		return nil, false
	}
	return radixLookupNoAlloc(methodRoot, path, c)
}

// AllowedMethods lists the methods registered for path (used to answer 405
// with an Allow header). Only called after a lookup miss — never on the hot path.
func (rt *RadixRouter) AllowedMethods(path []byte) []string {
	var out []string
	for _, methodNode := range rt.methodRoot.children {
		method := methodNode.segment
		if mm, ok := rt.staticRoutes[method]; ok {
			if _, ok := mm[b2s(path)]; ok {
				out = append(out, method)
				continue
			}
		}
		var scratch domain.Ctx
		if h, ok := radixLookupNoAlloc(methodNode, path, &scratch); ok && h != nil {
			out = append(out, method)
		}
	}
	return out
}

// Routes returns all routes registered on this router.
func (rt *RadixRouter) Routes() []domain.RouteInfo {
	var routes []domain.RouteInfo
	for _, methodNode := range rt.methodRoot.children {
		collectRoutes(methodNode, methodNode.segment, &routes)
	}
	return routes
}

// methodNode returns (creating if needed) the tree root node
// for a specific HTTP method.
func (rt *RadixRouter) methodNode(method string) *radixNode {
	for _, child := range rt.methodRoot.children {
		if child.segment == method {
			return child
		}
	}
	node := &radixNode{segment: method}
	rt.methodRoot.children = append(rt.methodRoot.children, node)
	return node
}

// findMethodNodeBytes looks up a method root node from []byte.
func (rt *RadixRouter) findMethodNodeBytes(method []byte) *radixNode {
	for _, child := range rt.methodRoot.children {
		if matchPathBytes(method, child.segment) {
			return child
		}
	}
	return nil
}

func collectRoutes(node *radixNode, method string, out *[]domain.RouteInfo) {
	if node.handler != nil {
		p := "/" + strings.Join(splitSegments(node.routePath), "/")
		*out = append(*out, domain.RouteInfo{Method: method, Path: p})
	}
	for _, child := range node.children {
		collectRoutes(child, method, out)
	}
	if node.param != nil {
		collectRoutes(node.param, method, out)
	}
	if node.catchAll != nil {
		collectRoutes(node.catchAll, method, out)
	}
}

// splitSegments splits a path into segments, ignoring empty slashes.
func splitSegments(path string) []string {
	var segs []string
	start := 1
	for i := 1; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				segs = append(segs, path[start:i])
			}
			start = i + 1
		}
	}
	return segs
}

// radixLookupNoAlloc walks the tree by indexes over the []byte path, never
// materializing a []string of segments. Captured values live in a
// stack-allocated array and are bound to the route's parameter names only
// once a handler is found.
func radixLookupNoAlloc(root *radixNode, path []byte, c *domain.Ctx) (domain.Handler, bool) {
	var vals [domain.MaxParams]string
	node := radixFind(root, path, 1, &vals, 0)
	if node == nil {
		return nil, false
	}
	for i, name := range node.paramNames {
		if i >= domain.MaxParams {
			break
		}
		c.SetParam(name, vals[i])
	}
	if node.routePath != "" {
		c.SetRoute(node.routePath)
	}
	return node.handler, true
}

// radixFind matches path[i:] under node. Precedence at every level is
// static > parameter > catch-all, with backtracking: when the preferred
// branch dead-ends further down, the next one is tried, so /items/me and
// /items/:id/detail can coexist (/items/me/detail resolves to the latter).
//
// A parameter never matches an empty segment (/users/ is not /users/:id
// with id=""); a catch-all does (/static/ → filepath="").
func radixFind(node *radixNode, path []byte, i int, vals *[domain.MaxParams]string, nv int) *radixNode {
	n := len(path)
	if i > n || n <= 1 {
		// Path fully consumed.
		if node.handler != nil {
			return node
		}
		if ca := node.catchAll; ca != nil && ca.handler != nil {
			if nv < domain.MaxParams {
				vals[nv] = ""
			}
			return ca
		}
		return nil
	}

	start := i
	for i < n && path[i] != '/' {
		i++
	}
	seg := path[start:i]

	// Trailing slash: the path ends in an empty segment.
	if len(seg) == 0 && i == n && node.handler != nil && node.trailingSlash {
		return node
	}

	for _, child := range node.children {
		if matchPathBytes(seg, child.segment) {
			if found := radixFind(child, path, i+1, vals, nv); found != nil {
				return found
			}
			break
		}
	}
	if p := node.param; p != nil && len(seg) > 0 {
		if nv < domain.MaxParams {
			vals[nv] = b2s(seg)
		}
		if found := radixFind(p, path, i+1, vals, nv+1); found != nil {
			return found
		}
	}
	if ca := node.catchAll; ca != nil && ca.handler != nil {
		if nv < domain.MaxParams {
			vals[nv] = b2s(path[start:])
		}
		return ca
	}
	return nil
}
