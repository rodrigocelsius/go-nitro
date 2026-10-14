package fasthttp

import (
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// radixNode is a node of the radix tree used for all routes (static
// and dynamic, unified).
type radixNode struct {
	segment    string
	isParam    bool
	isCatchAll bool
	handler    domain.Handler
	routePath  string // registered pattern, e.g. /users/:id
	children   []*radixNode
	hits       uint64 // branch prediction: contagem de acessos
}

// RadixRouter implements ports.Router using a radix tree
// per HTTP method. Structure validated by benchmark for high performance.
// Includes an O(1) fast path for static routes via map.
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

// Add registers a route on the radix tree for the matching method.
// If the route is static (no parameters), it is also added to the fast-path map.
// Supports:
//   - Static routes: /users
//   - Named parameters: /users/:id
//   - Catch-all: /static/*filepath (must be the last segment)
func (rt *RadixRouter) Add(method, path string, h domain.Handler) {
	// Check whether it contains parameters or catch-all
	hasParams := false
	hasCatchAll := false
	for i := 1; i < len(path); i++ {
		if path[i] == ':' {
			hasParams = true
		}
		if path[i] == '*' {
			hasCatchAll = true
		}
	}

	// Fast path: add to the map for static routes
	if !hasParams && !hasCatchAll {
		mm, ok := rt.staticRoutes[method]
		if !ok {
			mm = make(map[string]domain.Handler)
			rt.staticRoutes[method] = mm
		}
		mm[path] = h
	}

	// Always add to the radix tree for consistency
	node := rt.methodNode(method)
	segs := splitSegments(path)
	for _, seg := range segs {
		isParam := len(seg) > 0 && seg[0] == ':'
		isCatchAll := len(seg) > 0 && seg[0] == '*'
		var found *radixNode
		for _, child := range node.children {
			if child.isParam == isParam && child.isCatchAll == isCatchAll && child.segment == seg {
				found = child
				break
			}
		}
		if found == nil {
			found = &radixNode{segment: seg, isParam: isParam, isCatchAll: isCatchAll}
			node.children = append(node.children, found)
		}
		node = found
	}
	node.handler = h
	node.routePath = path
}

// Lookup resolves method+path (both []byte) to a Handler.
// Fast path: checks the O(1) static-route map before the radix tree.
func (rt *RadixRouter) Lookup(method, path []byte, c *domain.Ctx) (domain.Handler, bool) {
	// Fast path O(1): static-route map (zero-allocation via b2s)
	if mm, ok := rt.staticRoutes[b2s(method)]; ok {
		if h, ok := mm[b2s(path)]; ok {
			c.SetRoute(b2s(path))
			return h, true
		}
	}

	// Fallback: radix tree for dynamic routes
	methodRoot := rt.findMethodNodeBytes(method)
	if methodRoot == nil {
		return nil, false
	}
	h, ok := radixLookupNoAlloc(methodRoot, path, c)
	if ok && h != nil {
		// Branch prediction: increment hits on the method root
		methodRoot.hits++
	}
	return h, ok
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
		method := methodNode.segment
		collectRoutes(methodNode, method, "", &routes)
	}
	return routes
}

// methodNode returns (creating if needed) the radix-tree root node
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

func collectRoutes(node *radixNode, method, pathSoFar string, out *[]domain.RouteInfo) {
	if node.handler != nil {
		p := pathSoFar
		if p == "" {
			p = "/"
		}
		*out = append(*out, domain.RouteInfo{Method: method, Path: p})
	}
	for _, child := range node.children {
		seg := child.segment
		if child.isCatchAll {
			seg = "*" + seg[1:]
		}
		collectRoutes(child, method, pathSoFar+"/"+seg, out)
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

// SortChildrenByHits sorts each node's children by hit count (most accessed first).
// Should be called periodically (e.g. every 1000 requests) to keep branch prediction.
func (rt *RadixRouter) SortChildrenByHits() {
	sortNodeChildrenByHits(rt.methodRoot)
}

func sortNodeChildrenByHits(node *radixNode) {
	if node == nil || len(node.children) <= 1 {
		return
	}

	// Merge sort preserves stability and is O(n log n)
	sorted := mergeSortNodes(node.children)
	node.children = sorted

	for _, child := range sorted {
		sortNodeChildrenByHits(child)
	}
}

func mergeSortNodes(nodes []*radixNode) []*radixNode {
	if len(nodes) <= 1 {
		return nodes
	}
	mid := len(nodes) / 2
	left := mergeSortNodes(nodes[:mid])
	right := mergeSortNodes(nodes[mid:])
	return mergeNodes(left, right)
}

func mergeNodes(left, right []*radixNode) []*radixNode {
	result := make([]*radixNode, 0, len(left)+len(right))
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		if left[i].hits >= right[j].hits {
			result = append(result, left[i])
			i++
		} else {
			result = append(result, right[j])
			j++
		}
	}
	result = append(result, left[i:]...)
	result = append(result, right[j:]...)
	return result
}

// FlattenTree returns all nodes in pre-order for cache locality.
func (rt *RadixRouter) FlattenTree() []*radixNode {
	var nodes []*radixNode
	flattenNode(rt.methodRoot, &nodes)
	return nodes
}

func flattenNode(node *radixNode, out *[]*radixNode) {
	if node == nil {
		return
	}
	*out = append(*out, node)
	for _, child := range node.children {
		flattenNode(child, out)
	}
}

// radixLookupNoAlloc walks the radix tree by indexes over the
// []byte path, never materializing a []string of segments.
func radixLookupNoAlloc(root *radixNode, path []byte, c *domain.Ctx) (domain.Handler, bool) {
	node := root
	i := 1
	n := len(path)

	for i < n || (i == n && n > 1) {
		start := i
		for i < n && path[i] != '/' {
			i++
		}
		seg := path[start:i]
		i++

		var next *radixNode
		var paramMatch *radixNode
		var catchAllMatch *radixNode
		for _, child := range node.children {
			if !child.isParam && !child.isCatchAll && matchPathBytes(seg, child.segment) {
				next = child
				break
			}
			if child.isParam {
				paramMatch = child
			}
			if child.isCatchAll {
				catchAllMatch = child
			}
		}
		if next == nil {
			next = paramMatch
		}
		if next == nil {
			next = catchAllMatch
		}
		if next == nil {
			return nil, false
		}
		if next.isParam {
			c.SetParam(next.segment[1:], b2s(seg))
		}
		if next.isCatchAll {
			// For catch-all, capture everything from this point
			c.SetParam(next.segment[1:], b2s(path[start:]))
			if next.handler != nil {
				if next.routePath != "" {
					c.SetRoute(next.routePath)
				}
				return next.handler, true
			}
			return nil, false
		}
		node = next
	}

	// Check for root-level catch-all if no handler was found
	if node.handler == nil {
		for _, child := range node.children {
			if child.isCatchAll && child.handler != nil {
				c.SetParam(child.segment[1:], "")
				if child.routePath != "" {
					c.SetRoute(child.routePath)
				}
				return child.handler, true
			}
		}
	}

	if node.handler == nil {
		return nil, false
	}
	if node.routePath != "" {
		c.SetRoute(node.routePath)
	}
	return node.handler, true
}

// stringsIndexByte is a copy of strings.IndexByte for []byte paths.
func stringsIndexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
