package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migrationgen "github.com/rodrigocelsius/go-nitro/codegen/migration/v2"
)

func cmdMigrateMake(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen migrate:make <model.go> [--name <migration_name>] [--dir migrations]")
		os.Exit(1)
	}

	inputPath := args[0]
	opts := migrationgen.MakeOptions{Dir: "migrations"}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				opts.Name = args[i+1]
				i++
			}
		case "--dir":
			if i+1 < len(args) {
				opts.Dir = args[i+1]
				i++
			}
		}
	}

	if opts.Name == "" {
		opts.Name = generateMigrationName(inputPath)
	}

	applyDBDriverDialect()

	path, err := migrationgen.Make(inputPath, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("nitrogen: migration created: %s\n", path)
}

func generateMigrationName(inputPath string) string {
	base := filepath.Base(inputPath)
	name := strings.TrimSuffix(base, ".go")
	// models.go → create_from_models; hello.go → create_hellos
	if name == "models" {
		return "create_tables"
	}
	plural := pluralize(name)
	return fmt.Sprintf("create_%s", strings.ToLower(plural))
}

func pluralize(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, "y") && !strings.HasSuffix(lower, "ay") &&
		!strings.HasSuffix(lower, "ey") && !strings.HasSuffix(lower, "oy") &&
		!strings.HasSuffix(lower, "uy"):
		return name[:len(name)-1] + "ies"
	case strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "sh") ||
		strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "x") ||
		strings.HasSuffix(lower, "z"):
		return name + "es"
	default:
		return name + "s"
	}
}
