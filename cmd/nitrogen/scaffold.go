package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migrationgen "github.com/rodrigocelsius/go-nitro/codegen/migration/v2"
)

// hasInternalLayout reports whether the project uses Clean Arch internal/ layout.
func hasInternalLayout() bool {
	st, err := os.Stat("internal")
	return err == nil && st.IsDir()
}

func modulePath() string {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func resourceDirs(name string) (modelDir, usecaseDir, handlerDir, repoDir string) {
	base := strings.ToLower(name)
	if hasInternalLayout() {
		return filepath.Join("internal", "model"),
			filepath.Join("internal", "usecase", base),
			filepath.Join("internal", "handler"),
			filepath.Join("internal", "repository")
	}
	return ".", filepath.Join("usecase", base), ".", "repository"
}

func writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := writeFileFmt(path, []byte(content), 0644); err != nil {
		return err
	}
	// Don't remove .gitkeep when the file we just wrote is itself .gitkeep.
	if filepath.Base(path) != ".gitkeep" {
		removeGitkeep(dir)
	}
	return nil
}

// writeFileMaybe skips when the file exists unless force is set.
// Returns true when a write happened.
func writeFileMaybe(path, content string, force bool) (wrote bool, err error) {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return false, nil
		}
	}
	if err := writeFile(path, content); err != nil {
		return false, err
	}
	return true, nil
}

// removeGitkeep deletes .gitkeep once real content exists in the directory.
func removeGitkeep(dir string) {
	keep := filepath.Join(dir, ".gitkeep")
	if _, err := os.Stat(keep); err != nil {
		return
	}
	_ = os.Remove(keep)
}

func parseMakeNameArgs(args []string) (name string, resource bool, fields []fieldSpec, show bool, rest []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-r" || a == "--resource":
			resource = true
		case a == "--show":
			show = true
		case a == "--force":
			rest = append(rest, a)
		case a == "--with-seed" || a == "--with-factory":
			rest = append(rest, a)
		case a == "--tenant":
			// Convention: a tenant_id column makes the Resource tenant-scoped
			// automatically (see docs/guide/conventions.md).
			fields = append(fields, fieldSpec{
				Name: "TenantID", Type: "string",
				Tag: "`json:\"tenant_id\" db:\"tenant_id\" blinq:\"index\"`",
			})
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "nitrogen: unknown flag %s\n", a)
			os.Exit(1)
		default:
			if name == "" && !strings.Contains(a, ":") {
				name = a
			} else if strings.Contains(a, ":") {
				fs, err := parseFieldSpec(a)
				if err == nil {
					fields = append(fields, fs)
				} else {
					rest = append(rest, a)
				}
			} else {
				rest = append(rest, a)
			}
		}
	}
	return
}

func cmdMakeScaffold(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:scaffold <Name> [field:type ...] [--resource] [--tenant] [--force] [--with-seed] [--with-factory]")
		os.Exit(1)
	}
	raw, fields, opt := parseScaffoldArgs(args)
	names, err := normalizeResourceName(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	name := names.Type
	table := names.Table
	modelDir, usecaseDir, handlerDir, repoDir := resourceDirs(names.Package)
	pkgModel := "model"
	if !hasInternalLayout() {
		pkgModel = "main"
	}

	if preferResourceMode(opt) {
		cmdMakeScaffoldResource(names, fields, pkgModel, opt.force)
		runScaffoldExtras(names.Type, opt)
		return
	}

	modelPath := filepath.Join(modelDir, names.File+".go")
	if wrote, err := writeFileMaybe(modelPath, renderModelFilePkg(pkgModel, name, table, fields), opt.force); err != nil {
		fatalWrite(err)
	} else if wrote {
		fmt.Printf("nitrogen: wrote %s (type %s, table %s)\n", modelPath, name, table)
	} else {
		fmt.Printf("nitrogen: skip %s (exists, use --force)\n", modelPath)
	}

	ops := []string{"create", "get", "list", "update", "delete"}
	for _, op := range ops {
		p := filepath.Join(usecaseDir, op+".go")
		if err := writeFile(p, renderUsecaseFile(name, op)); err != nil {
			fatalWrite(err)
		}
		fmt.Printf("nitrogen: wrote %s\n", p)
	}

	handlerPath := filepath.Join(handlerDir, names.File+".go")
	if err := writeFile(handlerPath, renderHandlerFile(name, names.Table, fields)); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", handlerPath)

	repoPath := filepath.Join(repoDir, names.File+"_memory.go")
	if err := writeFile(repoPath, renderRepoMemory(name, fields)); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", repoPath)

	ensureUsecaseRepoInterface(name, fields)
	patchRoutes(names)
	writeHandlerTest(names)

	if hasInternalLayout() {
		_ = os.MkdirAll("migrations", 0755)
	}

	if hasSQLInstalled() {
		if err := writeSQLPlatform(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: platform: %v\n", err)
		}
		blinqPath := filepath.Join(repoDir, names.File+".go")
		if err := writeFile(blinqPath, renderRepoBlinq(name, table, fields)); err != nil {
			fatalWrite(err)
		}
		fmt.Printf("nitrogen: wrote %s\n", blinqPath)
		_ = ensureLocalNitroReplaces("data/adapter/sql", "data/adapter/pgx", "data/gateway", "blinq")
		patchMainWithRepo(names)
		applyDBDriverDialect()
		if err := generateBlinqData(modelPath, false); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: --data: %v\n", err)
		}
		migPath, err := migrationgen.Make(modelPath, migrationgen.MakeOptions{
			Dir:  "migrations",
			Name: "create_" + table,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: migrate:make: %v\n", err)
		} else {
			fmt.Printf("nitrogen: migration created: %s\n", migPath)
		}
	} else {
		patchMainMemoryRepo(names)
		if hasInternalLayout() {
			fmt.Println("nitrogen: tip — nitrogen install postgresql|mysql|sqlite then re-run make:scaffold, or: nitrogen migrate:make " + modelPath)
		}
	}
	runScaffoldExtras(name, opt)
}

func runScaffoldExtras(typeName string, opt scaffoldOpts) {
	if opt.withSeed {
		cmdMakeSeeder([]string{typeName})
	}
	if opt.withFactory {
		cmdMakeFactory([]string{typeName})
	}
}

func cmdMakeModel(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:model <Name> [-r|--resource] [field:type ...] [--show]")
		os.Exit(1)
	}
	raw, resource, fields, show, _ := parseMakeNameArgs(args)
	if show {
		cmdModelShow([]string{raw})
		return
	}
	names, err := normalizeResourceName(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	modelDir, _, _, _ := resourceDirs(names.Package)
	pkg := "model"
	if !hasInternalLayout() {
		pkg = "main"
	}
	path := filepath.Join(modelDir, names.File+".go")
	var body string
	if resource {
		body = renderModelFilePkg(pkg, names.Type, names.Table, fields)
	} else {
		body = fmt.Sprintf("package %s\n\ntype %s struct {\n}\n", pkg, names.Type)
	}
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s (type %s)\n", path, names.Type)
}

func cmdMakeHandler(args []string) {
	raw, _, _, _, _ := parseMakeNameArgs(args)
	names, err := normalizeResourceName(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:handler <Name>")
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	_, _, handlerDir, _ := resourceDirs(names.Package)
	path := filepath.Join(handlerDir, names.File+".go")
	if err := writeFile(path, renderHandlerFile(names.Type, names.Table, nil)); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
}

func cmdMakeUsecase(args []string) {
	raw, _, _, _, rest := parseMakeNameArgs(args)
	names, err := normalizeResourceName(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:usecase <Name> [OpName]")
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	_, usecaseDir, _, _ := resourceDirs(names.Package)

	// Single custom usecase: nitrogen make:usecase Product ApplyDiscount
	if len(rest) > 0 {
		opType := exportIdent(rest[0])
		if opType == "" {
			fmt.Fprintln(os.Stderr, "usage: nitrogen make:usecase <Name> [OpName]")
			os.Exit(1)
		}
		p := filepath.Join(usecaseDir, toSnake(opType)+".go")
		if err := writeFile(p, renderCustomUsecaseFile(names.Type, opType)); err != nil {
			fatalWrite(err)
		}
		fmt.Printf("nitrogen: wrote %s\n", p)
		ensureUsecaseRepoInterface(names.Type, nil)
		return
	}

	for _, op := range []string{"create", "get", "list", "update", "delete"} {
		p := filepath.Join(usecaseDir, op+".go")
		if err := writeFile(p, renderUsecaseFile(names.Type, op)); err != nil {
			fatalWrite(err)
		}
		fmt.Printf("nitrogen: wrote %s\n", p)
	}
	ensureUsecaseRepoInterface(names.Type, nil)
}

func cmdMakeRepository(args []string) {
	raw, _, _, _, _ := parseMakeNameArgs(args)
	names, err := normalizeResourceName(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:repository <Name>")
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	_, _, _, repoDir := resourceDirs(names.Package)
	path := filepath.Join(repoDir, names.File+"_memory.go")
	if err := writeFile(path, renderRepoMemory(names.Type, nil)); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
}

func fatalWrite(err error) {
	fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
	os.Exit(1)
}
