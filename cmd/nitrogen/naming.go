package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	validProjectNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	validIdentRe       = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// validateProjectName ensures a Go-friendly project directory name.
func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("project name must not contain path separators")
	}
	if !validProjectNameRe.MatchString(name) {
		return fmt.Errorf("project name must match [a-zA-Z][a-zA-Z0-9_-]* (got %q)", name)
	}
	return nil
}

// normalizeModulePath lowercases and sanitizes a module path segment / default module.
func normalizeModulePath(module string) string {
	module = strings.TrimSpace(module)
	module = strings.ReplaceAll(module, " ", "-")
	return strings.ToLower(module)
}

// resourceNames holds Go CA naming for a CRUD resource.
type resourceNames struct {
	Type    string // PascalCase singular: Product
	Package string // lowercase singular: product
	Table   string // snake plural: products
	File    string // lowercase singular file stem: product
}

// normalizeResourceName converts user input (Product, products, product) into Go conventions.
func normalizeResourceName(raw string) (resourceNames, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return resourceNames{}, fmt.Errorf("resource name is required")
	}
	if strings.ContainsAny(raw, "/\\ ") {
		return resourceNames{}, fmt.Errorf("resource name must be a single identifier (got %q)", raw)
	}
	// Allow product, Product, products, Products
	cleaned := strings.ReplaceAll(raw, "-", "_")
	if !validIdentRe.MatchString(cleaned) && !validIdentRe.MatchString(exportIdent(cleaned)) {
		return resourceNames{}, fmt.Errorf("invalid resource name %q (use letters/digits, e.g. Product)", raw)
	}

	pascal := toPascal(cleaned)
	singular := singularizePascal(pascal)
	if singular == "" {
		return resourceNames{}, fmt.Errorf("could not normalize resource name %q", raw)
	}
	pkg := strings.ToLower(singular)
	table := toSnake(pluralize(singular))
	return resourceNames{
		Type:    singular,
		Package: pkg,
		Table:   table,
		File:    pkg,
	}, nil
}

func toPascal(s string) string {
	if s == "" {
		return s
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		// split camelCase
		runes := []rune(p)
		start := 0
		for i := 1; i < len(runes); i++ {
			if unicode.IsUpper(runes[i]) && unicode.IsLower(runes[i-1]) {
				b.WriteString(exportIdent(string(runes[start:i])))
				start = i
			}
		}
		b.WriteString(exportIdent(string(runes[start:])))
	}
	if b.Len() == 0 {
		return exportIdent(s)
	}
	return b.String()
}

func singularizePascal(name string) string {
	if name == "" {
		return name
	}
	lower := strings.ToLower(name)
	// common irregulars
	irregular := map[string]string{
		"people": "Person", "men": "Man", "women": "Woman",
		"children": "Child", "teeth": "Tooth", "feet": "Foot",
		"mice": "Mouse", "geese": "Goose",
	}
	if s, ok := irregular[lower]; ok {
		return s
	}
	switch {
	case strings.HasSuffix(lower, "ies") && len(name) > 3:
		return name[:len(name)-3] + "y"
	case strings.HasSuffix(lower, "ses") || strings.HasSuffix(lower, "xes") ||
		strings.HasSuffix(lower, "zes") || strings.HasSuffix(lower, "ches") ||
		strings.HasSuffix(lower, "shes"):
		return name[:len(name)-2]
	case strings.HasSuffix(lower, "us") || strings.HasSuffix(lower, "is") ||
		strings.HasSuffix(lower, "ss") || strings.HasSuffix(lower, "ous"):
		return name
	case strings.HasSuffix(lower, "s") && len(name) > 3:
		return name[:len(name)-1]
	default:
		return name
	}
}
