package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	migrationgen "github.com/rodrigocelsius/go-nitro/codegen/migration/v2"
)

// cmdGenerateResource: nitrogen g resource Product name:string price:float64 active:bool
func cmdGenerateResource(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen g resource <Name> [field:type ...] [--dir .] [--table name]")
		os.Exit(1)
	}
	name := args[0]
	if name == "" || !unicode.IsUpper(rune(name[0])) {
		fmt.Fprintln(os.Stderr, "nitrogen: model name must be exported PascalCase (e.g. Product)")
		os.Exit(1)
	}

	dir := "."
	table := ""
	var fields []fieldSpec
	for i := 1; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--dir" && i+1 < len(args):
			dir = args[i+1]
			i++
		case a == "--table" && i+1 < len(args):
			table = args[i+1]
			i++
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "nitrogen: unknown flag %s\n", a)
			os.Exit(1)
		default:
			fs, err := parseFieldSpec(a)
			if err != nil {
				fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
				os.Exit(1)
			}
			fields = append(fields, fs)
		}
	}
	if table == "" {
		table = pluralize(strings.ToLower(toSnake(name)))
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}

	modelFile := filepath.Join(dir, strings.ToLower(name)+".go")
	if err := writeFileFmt(modelFile, []byte(renderModelFile(name, table, fields)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: write model: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("nitrogen: wrote %s\n", modelFile)

	// BLinq metamodel (DSL in modelgen/)
	cmdMakeData(modelFile, false)

	// Migration
	migPath, err := migrationgen.Make(modelFile, migrationgen.MakeOptions{
		Dir:  filepath.Join(dir, "migrations"),
		Name: "create_" + table,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: migration: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("nitrogen: wrote %s\n", migPath)
}

type fieldSpec struct {
	Name string
	Type string
	Tag  string
}

func parseFieldSpec(s string) (fieldSpec, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return fieldSpec{}, fmt.Errorf("invalid field %q (want name:type or name:type:index)", s)
	}
	goName := exportIdent(parts[0])
	goType, validate := mapFieldType(parts[1])
	col := toSnake(goName)
	tag := fmt.Sprintf("`json:%q db:%q", col, col)
	if validate != "" {
		tag += fmt.Sprintf(" validate:%q", validate)
	}
	// blinq:"index" ⇒ CREATE INDEX + Resource Filterable/Sortable.
	// Do NOT index every scalar — only filter/sort keys (or explicit :index).
	wantIndex := shouldBlinqIndex(goName, col, parts[2:])
	if wantIndex {
		tag += ` blinq:"index"`
	}
	if goType == "bool" {
		tag += ` default:"true"`
	}
	tag += "`"
	return fieldSpec{Name: goName, Type: goType, Tag: tag}, nil
}

// shouldBlinqIndex: explicit :index / :noindex, else only common filter/FK names.
func shouldBlinqIndex(goName, col string, flags []string) bool {
	for _, f := range flags {
		switch strings.ToLower(strings.TrimSpace(f)) {
		case "index", "idx":
			return true
		case "noindex", "noidx":
			return false
		}
	}
	n := strings.ToLower(col)
	switch n {
	case "name", "email", "sku", "status", "active", "role", "type",
		"document", "number", "slug", "code", "username":
		return true
	}
	if strings.HasSuffix(n, "_id") || strings.HasSuffix(goName, "ID") || strings.HasSuffix(goName, "Id") {
		return true
	}
	return false
}

func mapFieldType(t string) (goType, validate string) {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "string", "text":
		return "string", "required"
	case "int", "int64", "integer":
		return "int64", ""
	case "float", "float64", "number":
		return "float64", "gte=0"
	case "bool", "boolean":
		return "bool", ""
	case "time", "timestamp", "datetime":
		return "time.Time", ""
	default:
		return t, ""
	}
}

func exportIdent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func toSnake(s string) string {
	// UserID → user_id (not user_i_d); HTTPServer → http_server.
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := runes[i-1]
				nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
				if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z' && nextLower) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func renderModelFile(name, table string, fields []fieldSpec) string {
	needTime := false
	for _, f := range fields {
		if f.Type == "time.Time" {
			needTime = true
			break
		}
	}
	var b strings.Builder
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")
	if needTime {
		b.WriteString("\t\"time\"\n\n")
	}
	b.WriteString("\tnitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n")
	b.WriteString(")\n\n")
	b.WriteString("//go:generate nitrogen make " + strings.ToLower(name) + ".go --data\n\n")
	fmt.Fprintf(&b, "//nitro:table=%s\n", table)
	fmt.Fprintf(&b, "type %s struct {\n", name)
	b.WriteString("\tnitro.Model\n")
	for _, f := range fields {
		fmt.Fprintf(&b, "\t%s %s %s\n", f.Name, f.Type, f.Tag)
	}
	b.WriteString("}\n")
	return b.String()
}
