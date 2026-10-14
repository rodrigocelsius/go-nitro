package routergen

import (
	"fmt"
	"strings"
)

// generateMatchBody emits the Go code that walks the path segment by
// segment from a tree node, replicating the pattern validated in
// prototype A (router_compiled_proto/match_a.go): indexes into the
// path string, no segment slice, a direct string switch at each level.
func generateMatchBody(root *segmentNode, indent string) (string, error) {
	var b strings.Builder
	b.WriteString(indent + "n := len(path)\n")
	b.WriteString(indent + "if n == 0 || path[0] != '/' {\n")
	b.WriteString(indent + "\tbreak\n")
	b.WriteString(indent + "}\n")

	// The first segment starts right after the leading slash (position 1)
	// and is read directly — WITHOUT checking "path[i] == '/'" first,
	// because that check is for the SEPARATOR slash between a segment
	// and the next, which only exists from the second segment onward.
	// Real bug found during development: generateNode() treated the
	// root as a regular child, introducing that "if" incorrectly and
	// making EVERY route fail (even /health) — caught by
	// TestMatchCompiled_ConsistentWithRuntimeRouter, which compares
	// against the Phase 2 runtime router.
	b.WriteString(indent + "i := 1\n")
	b.WriteString(indent + "segStartRoot := i\n")
	b.WriteString(indent + "for i < n && path[i] != '/' {\n")
	b.WriteString(indent + "\ti++\n")
	b.WriteString(indent + "}\n")
	b.WriteString(indent + "segRoot := path[segStartRoot:i]\n")

	code, err := generateFirstLevel(root, indent, "segRoot")
	if err != nil {
		return "", err
	}
	b.WriteString(code)

	return b.String(), nil
}

// generateFirstLevel emits the switch for the first segment (already
// read by generateMatchBody) and delegates later levels to
// generateNode, which DOES need to check the separator slash before
// each subsequent segment.
func generateFirstLevel(root *segmentNode, indent, segVar string) (string, error) {
	var b strings.Builder

	if root.handlerName != "" {
		fmt.Fprintf(&b, "%sif i == n {\n", indent)
		fmt.Fprintf(&b, "%s\treturn %s, true\n", indent, root.handlerName)
		fmt.Fprintf(&b, "%s}\n", indent)
	}

	var staticChildren []*segmentNode
	var paramChild *segmentNode
	for _, child := range root.children {
		if child.isParam {
			paramChild = child
		} else {
			staticChildren = append(staticChildren, child)
		}
	}

	if len(staticChildren) > 0 {
		fmt.Fprintf(&b, "%sswitch %s {\n", indent, segVar)
		for _, child := range staticChildren {
			fmt.Fprintf(&b, "%scase %q:\n", indent, child.name)
			childCode, err := generateNode(child, indent+"\t", 1)
			if err != nil {
				return "", err
			}
			b.WriteString(childCode)
		}
		fmt.Fprintf(&b, "%s}\n", indent)
	}

	if paramChild != nil {
		paramName := paramChild.name[1:]
		fmt.Fprintf(&b, "%sc.SetParam(%q, %s)\n", indent, paramName, segVar)
		childCode, err := generateNode(paramChild, indent, 1)
		if err != nil {
			return "", err
		}
		b.WriteString(childCode)
	}

	return b.String(), nil
}

// generateNode emits code to walk a node's children. depth is used to
// generate unique variable names (seg0, seg1...) avoiding collisions
// on deep paths.
func generateNode(node *segmentNode, indent string, depth int) (string, error) {
	if len(node.children) == 0 {
		if node.handlerName != "" {
			var b strings.Builder
			fmt.Fprintf(&b, "%sif i == n {\n", indent)
			fmt.Fprintf(&b, "%s\treturn %s, true\n", indent, node.handlerName)
			fmt.Fprintf(&b, "%s}\n", indent)
			return b.String(), nil
		}
		return "", nil
	}

	var b strings.Builder

	if node.handlerName != "" {
		fmt.Fprintf(&b, "%sif i == n {\n", indent)
		fmt.Fprintf(&b, "%s\treturn %s, true\n", indent, node.handlerName)
		fmt.Fprintf(&b, "%s}\n", indent)
	}

	fmt.Fprintf(&b, "%sif i < n && path[i] == '/' {\n", indent)
	inner := indent + "\t"
	fmt.Fprintf(&b, "%si++\n", inner)
	segVar := fmt.Sprintf("segStart%d", depth)
	fmt.Fprintf(&b, "%s%s := i\n", inner, segVar)
	fmt.Fprintf(&b, "%sfor i < n && path[i] != '/' {\n", inner)
	fmt.Fprintf(&b, "%s\ti++\n", inner)
	fmt.Fprintf(&b, "%s}\n", inner)
	segValueVar := fmt.Sprintf("seg%d", depth)
	fmt.Fprintf(&b, "%s%s := path[%s:i]\n", inner, segValueVar, segVar)

	var staticChildren []*segmentNode
	var paramChild *segmentNode
	for _, child := range node.children {
		if child.isParam {
			paramChild = child
		} else {
			staticChildren = append(staticChildren, child)
		}
	}

	if len(staticChildren) > 0 {
		fmt.Fprintf(&b, "%sswitch %s {\n", inner, segValueVar)
		for _, child := range staticChildren {
			fmt.Fprintf(&b, "%scase %q:\n", inner, child.name)
			childCode, err := generateNode(child, inner+"\t", depth+1)
			if err != nil {
				return "", err
			}
			b.WriteString(childCode)
		}
		fmt.Fprintf(&b, "%s}\n", inner)
	}

	if paramChild != nil {
		paramName := paramChild.name[1:] // remove ":"
		fmt.Fprintf(&b, "%sc.SetParam(%q, %s)\n", inner, paramName, segValueVar)
		childCode, err := generateNode(paramChild, inner, depth+1)
		if err != nil {
			return "", err
		}
		b.WriteString(childCode)
	}

	fmt.Fprintf(&b, "%s}\n", indent)

	return b.String(), nil
}
