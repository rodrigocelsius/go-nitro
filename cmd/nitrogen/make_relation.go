package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migrationgen "github.com/rodrigocelsius/go-nitro/codegen/migration/v2"
)

func cmdMakeRelation(args []string) {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:relation <A> belongs_to|has_many <B>")
		fmt.Fprintln(os.Stderr, "  nitrogen make:relation Order belongs_to User")
		fmt.Fprintln(os.Stderr, "  nitrogen make:relation User has_many Order")
		os.Exit(1)
	}
	left := args[0]
	kind := strings.ToLower(strings.ReplaceAll(args[1], "-", "_"))
	right := args[2]
	switch kind {
	case "belongs_to", "belongsto":
		makeBelongsTo(left, right)
	case "has_many", "hasmany":
		makeHasMany(left, right)
	default:
		fmt.Fprintf(os.Stderr, "nitrogen: unknown relation %q (use belongs_to or has_many)\n", args[1])
		os.Exit(1)
	}
}

func makeBelongsTo(childRaw, parentRaw string) {
	child, err := normalizeResourceName(childRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	parent, err := normalizeResourceName(parentRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	fkName := parent.Type + "ID"
	fkCol := strings.ToLower(parent.Package) + "_id"
	ensureModelFK(child, fkName, fkCol)
	writeRelationHelper(child, parent, "belongs_to", fkCol)
	writeRelationMigration(child.Table, fkCol)
	fmt.Printf("nitrogen: relation %s belongs_to %s (FK %s.%s)\n", child.Type, parent.Type, child.Table, fkCol)
	fmt.Printf("nitrogen: tip — q.Include(modelgen.%s)\n", child.Type+"BelongsTo"+parent.Type)
}

func makeHasMany(parentRaw, childRaw string) {
	parent, err := normalizeResourceName(parentRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	child, err := normalizeResourceName(childRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	fkName := parent.Type + "ID"
	fkCol := strings.ToLower(parent.Package) + "_id"
	ensureModelFK(child, fkName, fkCol)
	writeRelationHelper(child, parent, "has_many", fkCol)
	writeRelationMigration(child.Table, fkCol)
	fmt.Printf("nitrogen: relation %s has_many %s (FK %s.%s)\n", parent.Type, child.Type, child.Table, fkCol)
	fmt.Printf("nitrogen: tip — q.Include(modelgen.%s)\n", parent.Type+"HasMany"+child.Type)
}

func ensureModelFK(child resourceNames, fkName, fkCol string) {
	modelDir, _, _, _ := resourceDirs(child.Package)
	path := filepath.Join(modelDir, child.File+".go")
	b, err := os.ReadFile(path)
	if err != nil {
		pkg := "model"
		if !hasInternalLayout() {
			pkg = "main"
		}
		body := renderModelFilePkg(pkg, child.Type, child.Table, []fieldSpec{
			{Name: fkName, Type: "nitro.UUID", Tag: fmt.Sprintf("`json:%q db:%q blinq:\"index\"`", fkCol, fkCol)},
		})
		// nitro.UUID needs to be nitro.UUID without importing twice — Model already imports nitro
		body = strings.Replace(body, "nitro.UUID", "nitro.UUID", 1)
		if err := writeFile(path, body); err != nil {
			fatalWrite(err)
		}
		fmt.Printf("nitrogen: wrote %s (with %s)\n", path, fkName)
		return
	}
	src := string(b)
	if strings.Contains(src, fkName+" ") || strings.Contains(src, `db:"`+fkCol+`"`) {
		fmt.Printf("nitrogen: model already has %s\n", fkName)
		return
	}
	tag := fmt.Sprintf("\t%s nitro.UUID `json:%q db:%q blinq:\"index\"`\n", fkName, fkCol, fkCol)
	needle := "nitro.Model\n"
	if i := strings.Index(src, needle); i >= 0 {
		src = src[:i+len(needle)] + tag + src[i+len(needle):]
	} else if i := strings.Index(src, "struct {\n"); i >= 0 {
		j := i + len("struct {\n")
		src = src[:j] + tag + src[j:]
	} else {
		fmt.Fprintf(os.Stderr, "nitrogen: could not patch %s — add %s manually\n", path, fkName)
		return
	}
	if err := writeFileFmt(path, []byte(src), 0644); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: added %s to %s\n", fkName, path)
}

func writeRelationHelper(child, parent resourceNames, kind, fkCol string) {
	dir := "internal/modelgen"
	if !hasInternalLayout() {
		dir = "modelgen"
	}
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "relations_gen.go")

	varNameBT := child.Type + "BelongsTo" + parent.Type
	varNameHM := parent.Type + "HasMany" + child.Type
	check := varNameBT
	if kind == "has_many" {
		check = varNameHM
	}

	snippet := renderRelationSnippet(child, parent, kind, fkCol)

	if b, err := os.ReadFile(path); err == nil {
		src := string(b)
		if strings.Contains(src, "var "+check) {
			fmt.Printf("nitrogen: %s already defined\n", check)
			return
		}
		if !strings.Contains(src, "github.com/rodrigocelsius/go-nitro/blinq/v2/field") {
			src = "package modelgen\n\nimport \"github.com/rodrigocelsius/go-nitro/blinq/v2/field\"\n\n" + strings.TrimPrefix(src, "package modelgen\n")
		}
		_ = writeFileFmt(path, []byte(strings.TrimRight(src, "\n")+"\n"+snippet), 0644)
	} else {
		body := "package modelgen\n\nimport \"github.com/rodrigocelsius/go-nitro/blinq/v2/field\"\n\n" + snippet
		if err := writeFile(path, body); err != nil {
			fatalWrite(err)
		}
	}
	fmt.Printf("nitrogen: wrote/updated %s\n", path)
}

func renderRelationSnippet(child, parent resourceNames, kind, fkCol string) string {
	varNameBT := child.Type + "BelongsTo" + parent.Type
	varNameHM := parent.Type + "HasMany" + child.Type
	var b strings.Builder
	if kind == "belongs_to" {
		fmt.Fprintf(&b, "// %s — Include from %s → %s.\n", varNameBT, child.Type, parent.Type)
		fmt.Fprintf(&b, "var %s = field.BelongsTo(%q,\n", varNameBT, parent.Package)
		fmt.Fprintf(&b, "\tfield.Entity{Name: %q},\n", parent.Table)
		fmt.Fprintf(&b, "\tfield.Col[any](%q, %q).Ref(),\n", child.Table, fkCol)
		fmt.Fprintf(&b, "\tfield.Col[any](%q, \"id\").Ref(),\n", parent.Table)
		b.WriteString(")\n")
	} else {
		fmt.Fprintf(&b, "// %s — Include from %s → %s.\n", varNameHM, parent.Type, child.Type)
		fmt.Fprintf(&b, "var %s = field.HasMany(%q,\n", varNameHM, child.Table)
		fmt.Fprintf(&b, "\tfield.Entity{Name: %q},\n", child.Table)
		fmt.Fprintf(&b, "\tfield.Col[any](%q, \"id\").Ref(),\n", parent.Table)
		fmt.Fprintf(&b, "\tfield.Col[any](%q, %q).Ref(),\n", child.Table, fkCol)
		b.WriteString(")\n")
	}
	return b.String()
}

func writeRelationMigration(childTable, fkCol string) {
	if !hasInternalLayout() && !dirExists("migrations") {
		_ = os.MkdirAll("migrations", 0755)
	}
	if !dirExists("migrations") {
		return
	}
	ts := migrationgen.NextMigrationVersion("migrations")
	name := fmt.Sprintf("%s_add_%s_to_%s.sql", ts, fkCol, childTable)
	path := filepath.Join("migrations", name)
	if _, err := os.Stat(path); err == nil {
		return
	}
	body := fmt.Sprintf(`-- +goose Up
ALTER TABLE %s ADD COLUMN %s TEXT;

-- +goose Down
-- ALTER TABLE %s DROP COLUMN %s;
`, childTable, fkCol, childTable, fkCol)
	if err := writeFile(path, body); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: migration: %v\n", err)
		return
	}
	fmt.Printf("nitrogen: migration stub %s (adjust type/FK for your dialect)\n", path)
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
