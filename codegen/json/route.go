package gen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	nitroast "github.com/rodrigocelsius/go-nitro/codegen/ast/v2"
)

// route.go implements Phase 5: typed route-parameter extraction, Axum
// style. The developer writes
//
//	//nitro:route GET /users/:id
//	func GetUser(c *nitro.Ctx, id int) error {
//	    ...
//	}
//
// and the generator produces a GetUser_NitroHandler(c *nitro.Ctx) error
// wrapper that extracts "id" from the path, converts it to int, and
// calls GetUser. Zero reflection at any point — benchmark-validated
// (see extract_proto/bench_test.go): the reflect.Call alternative was
// ~30-40x slower than codegen or hand-written wrappers.
//
// KNOWN LIMITATION (documented, not hidden): the Go parameter name in
// the function signature MUST match the name used in the path
// (":id" -> parameter "id"). This is convention, not magic inference —
// go/ast gives us the parameter name for free, so we use it directly
// instead of requiring a second annotation.

// RouteAnnotation is a typed route extracted from a //nitro:route
// comment above a function.
type RouteAnnotation struct {
	Method     string
	Path       string
	FuncName   string
	CtxParam   string // *nitro.Ctx parameter name (usually "c")
	PathParams []RouteParam
}

// RouteParam is a parameter extracted from the path, with Go name and type.
type RouteParam struct {
	Name string    // Go parameter name AND path segment name (e.g. "id")
	Kind FieldKind // reuses FieldKind from parser.go (KindInt, KindString, etc)
}

const routeAnnotationPrefix = "//nitro:route "

// ParseRoutes scans a Go file for functions annotated with
// //nitro:route, returning a RouteAnnotation for each.
func ParseRoutes(filename string) ([]RouteAnnotation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("gen: failed to parse %s: %w", filename, err)
	}

	var routes []RouteAnnotation

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Doc == nil {
			continue
		}

		method, path, found := extractRouteComment(funcDecl.Doc)
		if !found {
			continue
		}

		route, err := resolveRouteFunc(funcDecl, method, path)
		if err != nil {
			return nil, fmt.Errorf("gen: route %s: %w", funcDecl.Name.Name, err)
		}
		routes = append(routes, route)
	}

	return routes, nil
}

func extractRouteComment(doc *ast.CommentGroup) (method, path string, found bool) {
	for _, comment := range doc.List {
		text := comment.Text
		if !strings.HasPrefix(text, routeAnnotationPrefix) {
			continue
		}
		rest := strings.TrimPrefix(text, routeAnnotationPrefix)
		parts := strings.Fields(rest)
		if len(parts) != 2 {
			continue
		}
		return parts[0], parts[1], true
	}
	return "", "", false
}

// resolveRouteFunc validates and extracts the signature of an annotated function.
// Expects: func Name(ctxParam *nitro.Ctx, param1 T1, param2 T2, ...) error
func resolveRouteFunc(funcDecl *ast.FuncDecl, method, path string) (RouteAnnotation, error) {
	params := funcDecl.Type.Params.List
	if len(params) == 0 {
		return RouteAnnotation{}, fmt.Errorf("expected at least one parameter (*nitro.Ctx)")
	}

	// First parameter must be *nitro.Ctx (or *Ctx, if in the same package).
	firstParam := params[0]
	if !isCtxType(firstParam.Type) {
		return RouteAnnotation{}, fmt.Errorf("first parameter must be *nitro.Ctx or *Ctx")
	}
	if len(firstParam.Names) == 0 {
		return RouteAnnotation{}, fmt.Errorf("first parameter (*Ctx) must be named")
	}
	ctxParamName := firstParam.Names[0].Name

	// Following parameters must match :name segments in the path.
	pathParamNames := extractPathParamNames(path)

	var routeParams []RouteParam
	paramIdx := 0
	for _, field := range params[1:] {
		for _, name := range field.Names {
			if paramIdx >= len(pathParamNames) {
				return RouteAnnotation{}, fmt.Errorf("more function parameters than path params (:name segments)")
			}
			expectedName := pathParamNames[paramIdx]
			if name.Name != expectedName {
				return RouteAnnotation{}, fmt.Errorf("parameter %q does not match path segment :%s (order and name must match)", name.Name, expectedName)
			}

			kind, _, _, _ := nitroast.ResolveType(field.Type)
			if kind != KindString && kind != KindInt && kind != KindFloat && kind != KindBool {
				return RouteAnnotation{}, fmt.Errorf("parameter %q has unsupported type for route extraction", name.Name)
			}

			routeParams = append(routeParams, RouteParam{Name: name.Name, Kind: kind})
			paramIdx++
		}
	}

	if paramIdx != len(pathParamNames) {
		return RouteAnnotation{}, fmt.Errorf("path has %d params but function declares %d", len(pathParamNames), paramIdx)
	}

	return RouteAnnotation{
		Method:     method,
		Path:       path,
		FuncName:   funcDecl.Name.Name,
		CtxParam:   ctxParamName,
		PathParams: routeParams,
	}, nil
}

// isCtxType reports whether an ast.Expr is *nitro.Ctx or *Ctx.
func isCtxType(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	switch t := star.X.(type) {
	case *ast.Ident:
		return t.Name == "Ctx"
	case *ast.SelectorExpr:
		return t.Sel.Name == "Ctx"
	default:
		return false
	}
}

// extractPathParamNames extracts :name segment names from a path, in
// appearance order (e.g. "/orders/:id/items/:itemId" -> ["id", "itemId"]).
func extractPathParamNames(path string) []string {
	var names []string
	segs := strings.Split(path, "/")
	for _, seg := range segs {
		if strings.HasPrefix(seg, ":") {
			names = append(names, seg[1:])
		}
	}
	return names
}
