package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func writeSQLPlatform() error {
	// SQL open + optional Redis L2 live in cmd/server (see patchSQLMain).
	// Platform package is only for rabbitmq/minio helpers.
	_ = os.Remove(filepath.Join("internal/platform", "db.go"))
	_ = os.Remove(filepath.Join("platform", "db.go"))
	_ = os.Remove(filepath.Join("internal/platform", "cache.go"))
	_ = os.Remove(filepath.Join("platform", "cache.go"))
	return nil
}

func writeRedisPlatform() error {
	// The L2 cache is inlined in patchSQLMain (VALKEY_ADDR / REDIS_ADDR).
	patchSQLMain()
	const path = "cmd/server/main.go"
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if src, ok := upgradeCacheBlock(string(b)); ok {
		if err := writeFileFmt(path, []byte(src), 0644); err != nil {
			return err
		}
		fmt.Println("nitrogen: cmd/server/main.go now reads VALKEY_ADDR (REDIS_ADDR still works)")
	}
	return nil
}

func writeRabbitPlatform() error {
	dir := "internal/platform"
	if !hasInternalLayout() {
		dir = "platform"
	}
	body := `package platform

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

// Queue is the broker selected by the environment — RabbitMQ (RABBITMQ_URL),
// NATS JetStream (NATS_URL) or Kafka (KAFKA_BROKERS) — with an in-memory
// fallback while it is unreachable; in-memory only when none is set.
func Queue() *nitro.FallbackQueue {
	return nitro.QueueFromEnv()
}
`
	return writeFile(filepath.Join(dir, "queue.go"), body)
}

func writeMinioPlatform() error {
	dir := "internal/platform"
	if !hasInternalLayout() {
		dir = "platform"
	}
	body := `package platform

import "os"

// BlobEnv is MinIO / S3-compatible settings from the environment.
type BlobEnv struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// BlobFromEnv reads MINIO_* (see nitrogen install minio).
func BlobFromEnv() BlobEnv {
	return BlobEnv{
		Endpoint:  os.Getenv("MINIO_ENDPOINT"),
		AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
		SecretKey: os.Getenv("MINIO_SECRET_KEY"),
		Bucket:    os.Getenv("MINIO_BUCKET"),
		UseSSL:    os.Getenv("MINIO_USE_SSL") == "true",
	}
}
`
	return writeFile(filepath.Join(dir, "blob.go"), body)
}

func nitroRootFromGoMod() string {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "github.com/rodrigocelsius/go-nitro/pkg/v2 =>") {
			continue
		}
		parts := strings.Split(line, "=>")
		if len(parts) != 2 {
			continue
		}
		p := strings.TrimSpace(parts[1])
		return filepath.Dir(p)
	}
	return ""
}

func ensureLocalNitroReplaces(mods ...string) error {
	root := nitroRootFromGoMod()
	if root == "" {
		return nil
	}
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return err
	}
	src := string(b)
	root = replaceTarget(".", root)
	var extra strings.Builder
	for _, m := range mods {
		needle := "github.com/rodrigocelsius/go-nitro/" + m + "/v2 =>"
		if strings.Contains(src, needle) {
			continue
		}
		fmt.Fprintf(&extra, "\nreplace github.com/rodrigocelsius/go-nitro/%s/v2 => %s/%s\n", m, root, m)
	}
	if extra.Len() == 0 {
		return nil
	}
	return writeFileFmt("go.mod", []byte(strings.TrimRight(src, "\n")+extra.String()), 0644)
}

func goGetPackages(pkgs ...string) {
	args := append([]string{"get"}, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("nitrogen: tip — run: go get %s\n", strings.Join(pkgs, " "))
	}
}

func isSQLDriver(driver string) bool {
	switch driver {
	case "postgresql", "mysql", "mariadb", "sqlserver", "sqlite":
		return true
	default:
		return false
	}
}

func patchSQLMain() {
	path := "cmd/server/main.go"
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	src := string(b)
	if strings.Contains(src, "sqladapter.OpenEnv") {
		return
	}
	mod := modulePath()
	if mod == "" {
		return
	}
	body := fmt.Sprintf(`package main

import (
	"context"
	"fmt"
	"log"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"%[1]s/internal/config"
	"%[1]s/internal/handler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %%v", err)
	}

	ctx := context.Background()
	db, err := sqladapter.OpenEnv(ctx)
	if err != nil {
		log.Fatalf("db: %%v (set DATABASE_URL / DB_DSN and NITRO_DB_DRIVER)", err)
	}
	defer db.Close()
	if err := db.MigrateUp(ctx); err != nil {
		log.Fatalf("migrate: %%v", err)
	}

	app := nitro.New(nitro.Config{Env: nitro.ParseEnv(cfg.Env), ServiceName: cfg.ServiceName})
	app.HealthCheck(nitro.NamedHealth("db", func() error {
		return db.Ping(context.Background())
	}))
%[2]s
	handler.Register(app)

	nitro.Ok(app.Listen(fmt.Sprintf(":%%d", cfg.Port)))
}
`, mod, valkeyCacheBlock)
	if err := writeFileFmt(path, []byte(body), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: main: %v\n", err)
		return
	}
	fmt.Println("nitrogen: wired cmd/server/main.go (sqladapter.OpenEnv + optional Valkey/Redis)")
}

// valkeyCacheBlock wires the shared L2 cache when VALKEY_ADDR (or REDIS_ADDR)
// is set. Valkey and Redis speak the same protocol, so one block serves both.
const valkeyCacheBlock = `	rdb, err := nitro.ValkeyClientFromEnv()
	if err != nil {
		log.Fatalf("cache: %v", err)
	}
	if rdb != nil {
		app.SetCache(nitro.RedisCache(rdb))
	}`

// legacyRedisBlock is what earlier nitrogen versions wrote into main.go; it
// reads REDIS_* only.
const legacyRedisBlock = `	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		rdb, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
		app.SetCache(nitro.RedisCache(redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       rdb,
		})))
	}`

// upgradeCacheBlock swaps the legacy REDIS_ADDR block for valkeyCacheBlock in
// an already wired main.go, so `nitrogen install valkey` takes effect there too,
// and drops the imports only the old block used.
func upgradeCacheBlock(src string) (string, bool) {
	if !strings.Contains(src, legacyRedisBlock) {
		return src, false
	}
	src = strings.Replace(src, legacyRedisBlock, valkeyCacheBlock, 1)
	for _, imp := range []struct{ line, use string }{
		{"\t\"github.com/redis/go-redis/v9\"\n", "redis."},
		{"\t\"strconv\"\n", "strconv."},
		{"\t\"os\"\n", "os."},
	} {
		if !strings.Contains(src, imp.use) {
			src = strings.Replace(src, imp.line, "", 1)
		}
	}
	return src, true
}
