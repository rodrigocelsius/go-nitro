package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacyMain = `package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
	ctx := context.Background()
	db, err := sqladapter.OpenEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}
	app := nitro.New(nitro.Config{})
	_ = db
` + legacyRedisBlock + `
	nitro.Ok(app.Listen(fmt.Sprintf(":%d", 8080)))
}
`

func parses(t *testing.T, src string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated main.go does not parse: %v\n%s", err, src)
	}
}

func TestUpgradeCacheBlockReadsValkeyEnv(t *testing.T) {
	out, ok := upgradeCacheBlock(legacyMain)
	if !ok {
		t.Fatal("legacy REDIS_ADDR block not recognised")
	}
	if !strings.Contains(out, "nitro.ValkeyClientFromEnv()") || strings.Contains(out, `os.Getenv("REDIS_ADDR")`) {
		t.Fatalf("block not swapped:\n%s", out)
	}
	for _, imp := range []string{`"github.com/redis/go-redis/v9"`, `"strconv"`, `"os"`} {
		if strings.Contains(out, imp) {
			t.Fatalf("unused import %s left behind:\n%s", imp, out)
		}
	}
	parses(t, out)

	if _, again := upgradeCacheBlock(out); again {
		t.Fatal("upgrade must be idempotent")
	}
}

func TestUpgradeCacheBlockKeepsImportsStillInUse(t *testing.T) {
	src := strings.Replace(legacyMain, "\t_ = db\n", "\t_ = db\n\t_ = os.Args\n", 1)
	out, _ := upgradeCacheBlock(src)
	if !strings.Contains(out, `"os"`) {
		t.Fatalf("os is still used and must stay imported:\n%s", out)
	}
	parses(t, out)
}

func TestValkeyComposeService(t *testing.T) {
	name, block, volume := dockerComposeService("valkey", "unused")
	if name != "valkey" || volume != "valkey_data" || !strings.Contains(block, "image: valkey/valkey:8-alpine") {
		t.Fatalf("name=%q volume=%q block=\n%s", name, volume, block)
	}
	if got := driverEnvVars("valkey", "x"); got["VALKEY_ADDR"] == "" {
		t.Fatalf("env = %v, want VALKEY_ADDR", got)
	}
}

func TestPatchSQLMainWiresValkey(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cmd/server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd/server/main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()

	patchSQLMain()
	b, err := os.ReadFile("cmd/server/main.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "nitro.ValkeyClientFromEnv()") || strings.Contains(src, "go-redis") {
		t.Fatalf("main.go not wired for Valkey:\n%s", src)
	}
	parses(t, src)
}
