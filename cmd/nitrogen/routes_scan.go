package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type scannedRoute struct {
	method string
	path   string
	file   string
	line   int
}

func collectGoFiles(root string) []string {
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			// path != root: the default root "." itself starts with a dot.
			if info != nil && info.IsDir() && path != root && (strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// scanProjectRoutes statically finds Get/Post/…, Resource, AuthRoutes, KeycloakAuthRoutes.
func scanProjectRoutes(root string) []scannedRoute {
	files := collectGoFiles(root)
	if len(files) == 0 {
		return nil
	}

	routerVars := map[string]bool{"app": true}
	groupVars := map[string]string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, ":= nitro.New()") || strings.Contains(trimmed, ":= fasthttp.New()") ||
				strings.Contains(trimmed, ":= nitro.New(") {
				if idx := strings.Index(trimmed, ":="); idx > 0 {
					varName := strings.TrimSpace(trimmed[:idx])
					if varName != "" {
						routerVars[varName] = true
					}
				}
			}
			if strings.Contains(trimmed, ".Group(") && strings.Contains(trimmed, ":=") {
				if idx := strings.Index(trimmed, ":="); idx > 0 {
					varName := strings.TrimSpace(trimmed[:idx])
					rest := trimmed[idx+2:]
					if strings.Contains(rest, ".Group(") && strings.Contains(rest, `"`) {
						groupVars[varName] = extractQuoted(rest) // may be ""
						routerVars[varName] = true
					}
				}
			}
		}
	}

	var routes []scannedRoute
	methods := []string{"Get", "Post", "Put", "Delete", "Patch", "Head", "Options"}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			for varName := range routerVars {
				for _, method := range methods {
					prefix := varName + "." + method + "("
					if !strings.HasPrefix(trimmed, prefix) {
						continue
					}
					rest := trimmed[len(prefix):]
					path := extractQuoted(rest)
					if path == "" {
						continue
					}
					if g, ok := groupVars[varName]; ok {
						path = strings.TrimSuffix(g, "/") + "/" + strings.TrimPrefix(path, "/")
						if !strings.HasPrefix(path, "/") {
							path = "/" + path
						}
					}
					routes = append(routes, scannedRoute{strings.ToUpper(method), path, f, i + 1})
				}

				for _, resName := range []string{"Resource", "ResourceWith"} {
					resPrefix := varName + "." + resName + "("
					if !strings.HasPrefix(trimmed, resPrefix) {
						continue
					}
					path := resourcePathFromCall(trimmed[len(resPrefix):], resName == "ResourceWith")
					if path == "" {
						continue
					}
					base := path
					for _, m := range []string{"GET", "POST"} {
						routes = append(routes, scannedRoute{m, base, f, i + 1})
					}
					for _, m := range []string{"GET", "PATCH", "DELETE"} {
						routes = append(routes, scannedRoute{m, base + "/:id", f, i + 1})
					}
				}

				for _, authName := range []string{"AuthRoutes", "KeycloakAuthRoutes"} {
					authPrefix := varName + "." + authName + "("
					if !strings.HasPrefix(trimmed, authPrefix) {
						continue
					}
					prefix := extractQuoted(trimmed[len(authPrefix):])
					if prefix == "" {
						prefix = "/auth"
					}
					prefix = "/" + strings.Trim(prefix, "/")
					if authName == "AuthRoutes" {
						for _, p := range []string{"/register", "/login", "/refresh"} {
							routes = append(routes, scannedRoute{"POST", prefix + p, f, i + 1})
						}
					} else {
						for _, p := range []string{"/login", "/refresh"} {
							routes = append(routes, scannedRoute{"POST", prefix + p, f, i + 1})
						}
					}
				}
			}

			// nitro.Resource(app, "/items", …) package helper
			if strings.Contains(trimmed, "nitro.Resource(") {
				idx := strings.Index(trimmed, "nitro.Resource(")
				rest := trimmed[idx+len("nitro.Resource("):]
				// skip first arg (app), take second if quoted
				if comma := strings.Index(rest, ","); comma >= 0 {
					path := extractQuoted(rest[comma+1:])
					if path != "" {
						if !strings.HasPrefix(path, "/") {
							path = "/" + path
						}
						for _, m := range []string{"GET", "POST"} {
							routes = append(routes, scannedRoute{m, path, f, i + 1})
						}
						for _, m := range []string{"GET", "PATCH", "DELETE"} {
							routes = append(routes, scannedRoute{m, path + "/:id", f, i + 1})
						}
					}
				}
			}
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].method != routes[j].method {
			return routes[i].method < routes[j].method
		}
		return routes[i].path < routes[j].path
	})
	return routes
}

// resourcePathFromCall extracts /products from Resource("/products"…) or Resource(&Product{}…).
func resourcePathFromCall(rest string, forceQuoted bool) string {
	if p := extractQuoted(rest); p != "" {
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		return p
	}
	if forceQuoted {
		return ""
	}
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "&") {
		return ""
	}
	rest = rest[1:]
	end := len(rest)
	for i, r := range rest {
		if r == '{' || r == ',' || r == ' ' || r == ')' {
			end = i
			break
		}
	}
	name := rest[:end]
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return "/" + toSnake(pluralize(name))
}

func extractQuoted(s string) string {
	start := strings.IndexByte(s, '"')
	if start < 0 {
		return ""
	}
	rest := s[start+1:]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func cmdRoutes(args []string) {
	root := "."
	for i := 0; i < len(args); i++ {
		if args[i] == "--dir" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}

	routes := scanProjectRoutes(root)
	if len(collectGoFiles(root)) == 0 {
		fmt.Fprintln(os.Stderr, "nitrogen: no Go files found in project")
		fmt.Fprintln(os.Stderr, "nitrogen: tip — use: nitrogen routes --dir ./cmd/myapp")
		os.Exit(1)
	}

	fmt.Printf("nitrogen: scanning routes in %s...\n\n", root)

	if len(routes) == 0 {
		fmt.Println("nitrogen: no routes found")
		fmt.Println("nitrogen: tip — Resource() and Group() prefixes are detected when possible")
		return
	}

	fmt.Printf("┌─────────┬──────────────────────────────┬──────────────────────┐\n")
	fmt.Printf("│ %-7s │ %-28s │ %-20s │\n", "Method", "Path", "File")
	fmt.Printf("├─────────┼──────────────────────────────┼──────────────────────┤\n")
	for _, r := range routes {
		fileLine := fmt.Sprintf("%s:%d", r.file, r.line)
		fmt.Printf("│ %-7s │ %-28s │ %-20s │\n", r.method, r.path, fileLine)
	}
	fmt.Printf("└─────────┴──────────────────────────────┴──────────────────────┘\n")
	fmt.Printf("\nnitrogen: %d route(s) found\n", len(routes))
}
