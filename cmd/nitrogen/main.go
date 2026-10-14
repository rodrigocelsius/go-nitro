// Command nitrogen is the Nitro CLI.
//
// Usage:
//
//	nitrogen <command> [flags]
package main

import (
	"fmt"
	"os"
	"strings"

	controllergen "github.com/rodrigocelsius/go-nitro/codegen/controller/v2"
	datagen "github.com/rodrigocelsius/go-nitro/codegen/data/v2"
	dtogen "github.com/rodrigocelsius/go-nitro/codegen/dto/v2"
	gen "github.com/rodrigocelsius/go-nitro/codegen/json/v2"
)

const nitrogenVersion = "2.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "new":
		cmdNew(args)
	case "install":
		cmdInstall(args)
	case "make":
		cmdMake(args)
	case "make:scaffold":
		cmdMakeScaffold(args)
	case "make:model":
		cmdMakeModel(args)
	case "make:handler":
		cmdMakeHandler(args)
	case "make:usecase":
		cmdMakeUsecase(args)
	case "make:repository":
		cmdMakeRepository(args)
	case "make:seeder":
		cmdMakeSeeder(args)
	case "make:factory":
		cmdMakeFactory(args)
	case "make:middleware":
		cmdMakeMiddleware(args)
	case "make:job":
		cmdMakeJob(args)
	case "make:relation":
		cmdMakeRelation(args)
	case "make:test":
		cmdMakeTest(args)
	case "make:controller", "make:ctrl":
		cmdMakeController(args)
	case "make:dto", "make:public":
		cmdMakeDTO(args)
	case "model:show":
		cmdModelShow(args)
	case "dev":
		cmdDev(args)
	case "up":
		cmdUp(args)
	case "down":
		cmdDown(args)
	case "routes":
		cmdRoutes(args)
	case "build":
		cmdBuild(args)
	case "migrate":
		cmdMigrate(args)
	case "migrate:make", "migrate:gen":
		cmdMigrateMake(args)
	case "seed":
		cmdSeed(args)
	case "openapi":
		cmdOpenAPI(args)
	case "doctor":
		cmdDoctor(args)
	case "ai:init", "ai":
		cmdAiInit(args)
	case "benchmark", "bench":
		cmdBenchmark(args)
	case "test":
		cmdTest(args)
	case "cover", "coverage":
		cmdCover(args)
	case "audit":
		cmdAudit(args)
	case "security", "sec":
		cmdSecurity(args)
	case "profile":
		cmdProfile(args)
	case "pprof":
		cmdPprof(args)
	case "stress":
		cmdStress(args)
	case "help", "--help", "-h":
		printUsage()
	case "version", "--version", "-v":
		fmt.Println("nitrogen version " + nitrogenVersion)
	case "g", "generate", "gen":
		fmt.Fprintln(os.Stderr, "nitrogen: 'g/generate' removed; use make:scaffold, make:model, migrate:make, …")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`nitrogen - Nitro HTTP Framework CLI

Usage: nitrogen <command> [flags]

Project:
  new              Create app (--basic | --microservice | --worker | --fleet | default CA)
                   [--module path] [--services a,b] [--nitro-path dir]
  install          Add driver (postgresql|mariadb|mysql|sqlserver|sqlite|redis|rabbitmq|minio|auth-jwt|auth-keycloak)

Generators:
  make             Codegen <file.go> [--data] [--same-package]
  make:scaffold    Full CRUD: CA usecases (default) or --resource (app.Resource)
                   [--tenant] [--with-seed] [--with-factory]
  make:model       Empty model; -r|--resource for CoC; --show to inspect
  make:handler     HTTP handler
  make:usecase     create/get/list/update/delete usecases; optional OpName
  make:repository  In-memory repository
  make:seeder      Seeder (creates internal/seed on demand)
  make:factory     Test factory (creates internal/factory on demand)
  make:middleware  HTTP middleware (creates internal/middleware on demand)
  make:job         Job / schedule (creates internal/job on demand)
  make:relation    belongs_to | has_many (FK + modelgen helper + migrate stub)
  make:test        Test stub
  make:dto         Public DTO
  make:controller  Controller from model
  model:show       Inspect model fields/tags

Runtime:
  up               Compose up + wait DB + MigrateUp + run server (flags: --no-dev --seed --migrate-only)
  down             docker compose down ([--volumes])
  dev              Hot reload (air)
  routes           List routes
  build            Compile
  test / cover     Tests and coverage
  migrate          Run migrations (goose)
  migrate:make     Generate SQL migration from model
  seed             List/run seeders
  openapi          Write openapi.json from static route scan ([--bearer] [-o path])
  doctor           Environment and layout diagnostics ([--db] ping SQL)
  ai:init          Write AGENTS.md, CLAUDE.md, AI.md, .cursor/rules/nitro.mdc, .cursor/prompts/nitro-blinq.md
  audit / security / profile / stress / benchmark

Examples:
  nitrogen new myapp
  nitrogen new myapp --microservice --module github.com/acme/myapp
  nitrogen new myapp --basic
  nitrogen new worker --worker --module github.com/acme/worker
  nitrogen new backend --fleet --services gateway,nfe,danfe --module github.com/acme/backend
  nitrogen install postgresql
  nitrogen install sqlite
  nitrogen install auth-jwt
  nitrogen install rabbitmq
  nitrogen make:scaffold Product name:string price:float64
  nitrogen make:scaffold Product name:string --resource --with-seed
  nitrogen make:scaffold Invoice total:float64 --resource --tenant   # auto tenant-scoped
  nitrogen make:scaffold products name:string
  nitrogen make:usecase Product
  nitrogen make:usecase Product ApplyDiscount
  nitrogen make:model Order -r
  nitrogen migrate:make internal/model/product.go
  nitrogen model:show Product
  nitrogen make:middleware Auth
  nitrogen make:job SyncOrders
  nitrogen make:relation Order belongs_to User
  nitrogen make:relation User has_many Order
  nitrogen seed
  nitrogen openapi --bearer -o openapi.json
  nitrogen up
  nitrogen up --migrate-only
  nitrogen down
  nitrogen doctor
  nitrogen doctor --db
  nitrogen ai:init

Version: ` + nitrogenVersion)
}
func cmdMakeDTO(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:dto <model.go> [--package <pkg>]")
		os.Exit(1)
	}
	inputPath := args[0]
	packageName := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--package" && i+1 < len(args) {
			packageName = args[i+1]
			i++
		}
	}
	if packageName == "" {
		var err error
		packageName, err = datagen.ParsePackageName(inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: failed to detect package name: %v\n", err)
			os.Exit(1)
		}
	}
	code, err := dtogen.GenerateFromFile(inputPath, packageName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	outPath := strings.TrimSuffix(inputPath, ".go") + "_public_gen.go"
	if err := writeFileFmt(outPath, []byte(code), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: failed to write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Printf("nitrogen: generated %s\n", outPath)
}

func cmdMake(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make <file.go> [--data] [--same-package]")
		os.Exit(1)
	}

	inputPath := args[0]
	generateData := false
	samePackage := false
	for _, arg := range args[1:] {
		switch arg {
		case "--data", "-d":
			generateData = true
		case "--same-package":
			samePackage = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "nitrogen: unknown flag %s\n", arg)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "nitrogen: unexpected argument %s\n", arg)
			os.Exit(1)
		}
	}

	if generateData {
		cmdMakeData(inputPath, samePackage)
	} else {
		cmdMakeJSON(inputPath)
	}
}

func cmdMakeController(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:controller <model.go> [--package <pkg>]")
		os.Exit(1)
	}

	inputPath := args[0]
	packageName := ""

	// Parse flags
	for i := 1; i < len(args); i++ {
		if args[i] == "--package" && i+1 < len(args) {
			packageName = args[i+1]
			i++
		}
	}

	// Auto-detect package name if not provided
	if packageName == "" {
		var err error
		packageName, err = datagen.ParsePackageName(inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: failed to detect package name: %v\n", err)
			os.Exit(1)
		}
	}

	code, err := controllergen.GenerateControllerFromModel(inputPath, packageName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}

	// Generate output filename
	base := strings.TrimSuffix(inputPath, ".go")
	outPath := base + "_ctrl_gen.go"

	if err := writeFileFmt(outPath, []byte(code), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: failed to write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Printf("nitrogen: generated %s\n", outPath)
}

func cmdMakeJSON(inputPath string) {
	defs, err := gen.ParseFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}

	packageName, err := gen.ParsePackageName(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	code, err := gen.Generate(packageName, defs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}

	outPath := strings.TrimSuffix(inputPath, ".go") + "_gen.go"
	if err := writeFileFmt(outPath, []byte(code), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: failed to write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Printf("nitrogen: generated %s\n", outPath)
}
