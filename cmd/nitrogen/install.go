package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func cmdInstall(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen install <postgresql|mariadb|mysql|sqlserver|sqlite|valkey|redis|rabbitmq|nats|kafka|minio|auth-jwt|auth-keycloak>")
		os.Exit(1)
	}
	driver := strings.ToLower(args[0])
	switch driver {
	case "postgres", "postgresql", "pg":
		driver = "postgresql"
	case "sql-server", "sqlserver", "mssql":
		driver = "sqlserver"
	case "maria", "mariadb":
		driver = "mariadb"
	case "mysql":
		driver = "mysql"
	case "sqlite", "sqlite3":
		driver = "sqlite"
	case "redis":
		driver = "redis"
	case "valkey":
		driver = "valkey"
	case "rabbit", "rabbitmq", "amqp":
		driver = "rabbitmq"
	case "nats", "jetstream":
		driver = "nats"
	case "kafka", "redpanda":
		driver = "kafka"
	case "minio", "s3":
		driver = "minio"
	case "auth-jwt", "jwt", "auth", "auth-keycloak", "keycloak":
		cmdInstallAuth(driver)
		return
	default:
		fmt.Fprintf(os.Stderr, "nitrogen: unknown driver %q\n", args[0])
		os.Exit(1)
	}

	if isSQLDriver(driver) {
		warnEngineSwitch(driver)
	}

	pass := randomDevPassword()
	vars := driverEnvVars(driver, pass)
	writeInstallEnv(driver, vars)

	if err := patchConfigForDriver(driver); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: config: %v\n", err)
	} else {
		fmt.Printf("nitrogen: installed %s config stubs\n", driver)
	}

	if err := patchDockerCompose(driver, pass); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: docker-compose: %v\n", err)
	} else if driver != "sqlite" {
		fmt.Printf("nitrogen: updated docker-compose.yml (%s)\n", driver)
	}
	if isSQLDriver(driver) {
		commentRivalSQLServices(driver)
	}

	if isSQLDriver(driver) {
		if err := writeSQLPlatform(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: platform: %v\n", err)
		}
		if err := ensureLocalNitroReplaces("data/adapter/sql", "data/adapter/pgx", "data/gateway", "blinq"); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: go.mod replace: %v\n", err)
		}
		goGetPackages("github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2")
		patchSQLMain()
		fmt.Println("nitrogen: wired cmd/server (sqladapter.OpenEnv + optional VALKEY_ADDR / REDIS_ADDR)")
	}
	switch driver {
	case "redis", "valkey":
		if err := writeRedisPlatform(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: platform cache: %v\n", err)
		}
		_ = ensureLocalNitroReplaces("adapters/cache")
		goGetPackages("github.com/redis/go-redis/v9")
	case "rabbitmq":
		if err := writeRabbitPlatform(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: platform queue: %v\n", err)
		}
		_ = ensureLocalNitroReplaces("adapters/amqp", "queue")
	case "nats", "kafka":
		// Same platform helper: nitro.QueueFromEnv picks the broker from
		// NATS_URL / KAFKA_BROKERS (in-memory fallback while it is down).
		if err := writeRabbitPlatform(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: platform queue: %v\n", err)
		}
		_ = ensureLocalNitroReplaces("adapters/"+driver, "queue")
	case "minio":
		if err := writeMinioPlatform(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: platform blob: %v\n", err)
		}
	}

	fmt.Println("nitrogen: ready — nitrogen make:scaffold Product name:string … then nitrogen up")
	if isSQLDriver(driver) {
		fmt.Println("nitrogen: generated DB password is for local/dev only; rotate before any shared deploy")
	}
}

func randomDevPassword() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read failing is effectively impossible on any
		// supported OS, but silently falling back to a fixed,
		// predictable string ("changeme-"+hex("fallback")) would have
		// scaffolded every affected project with the *same* "random"
		// dev password (see vuln.md V-37). Fail loudly instead of
		// generating a credential that isn't actually random.
		log.Fatalf("nitrogen: crypto/rand.Read failed, cannot generate a secure dev password: %v", err)
	}
	return hex.EncodeToString(b)
}

func driverEnvVars(driver, pass string) map[string]string {
	switch driver {
	case "postgresql":
		dsn := fmt.Sprintf("postgres://postgres:%s@127.0.0.1:5432/app?sslmode=disable", pass)
		return map[string]string{
			"NITRO_DB_DRIVER": "postgresql",
			"DB_DSN":          dsn,
			"DATABASE_URL":    dsn,
			"DB_SCHEMA":       "public",
		}
	case "mysql":
		dsn := fmt.Sprintf("user:%s@tcp(127.0.0.1:3306)/app?parseTime=true", pass)
		return map[string]string{
			"NITRO_DB_DRIVER": "mysql",
			"DB_DSN":          dsn,
			"DATABASE_URL":    dsn,
		}
	case "mariadb":
		dsn := fmt.Sprintf("user:%s@tcp(127.0.0.1:3306)/app?parseTime=true", pass)
		return map[string]string{
			"NITRO_DB_DRIVER": "mariadb",
			"DB_DSN":          dsn,
			"DATABASE_URL":    dsn,
		}
	case "sqlserver":
		dsn := fmt.Sprintf("sqlserver://sa:%s@127.0.0.1:1433?database=app", pass)
		return map[string]string{
			"NITRO_DB_DRIVER": "sqlserver",
			"DB_DSN":          dsn,
			"DATABASE_URL":    dsn,
		}
	case "sqlite":
		return map[string]string{
			"NITRO_DB_DRIVER": "sqlite",
			"DB_DSN":          "file:data/app.db",
			"DATABASE_URL":    "file:data/app.db",
		}
	case "redis":
		return map[string]string{
			"REDIS_ADDR":     "127.0.0.1:6379",
			"REDIS_PASSWORD": "",
			"REDIS_DB":       "0",
		}
	case "valkey":
		return map[string]string{
			"VALKEY_ADDR":     "127.0.0.1:6379",
			"VALKEY_PASSWORD": "",
			"VALKEY_DB":       "0",
		}
	case "rabbitmq":
		// "guest:guest" is RabbitMQ's own well-known default — leaving it
		// in the generated .env means every app scaffolded from this
		// template shares the same trivially-guessable credential
		// (targeted by mass scanners). Use the same per-project random
		// password as the SQL drivers instead (see vuln.md V-36).
		return map[string]string{
			"RABBITMQ_URL":      fmt.Sprintf("amqp://nitro:%s@127.0.0.1:5672/", pass),
			"RABBITMQ_USER":     "nitro",
			"RABBITMQ_PASSWORD": pass,
			"RABBITMQ_EXCHANGE": "nitro.events",
			"RABBITMQ_PREFETCH": "5",
		}
	case "nats":
		return map[string]string{
			"NATS_URL":       "nats://127.0.0.1:4222",
			"NATS_RETRY_MAX": "3",
		}
	case "kafka":
		return map[string]string{
			"KAFKA_BROKERS":   "127.0.0.1:9092",
			"KAFKA_RETRY_MAX": "3",
		}
	case "minio":
		// "minioadmin"/"minioadmin" is MinIO's own documented default —
		// same reasoning as rabbitmq above (see vuln.md V-23).
		return map[string]string{
			"MINIO_ENDPOINT":   "127.0.0.1:9000",
			"MINIO_ACCESS_KEY": "nitro",
			"MINIO_SECRET_KEY": pass,
			"MINIO_BUCKET":     "app",
			"MINIO_USE_SSL":    "false",
		}
	default:
		return nil
	}
}

func appendEnvVars(path string, vars map[string]string) error {
	existing := map[string]bool{}
	var lines []string
	if b, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			lines = append(lines, line)
			if i := strings.IndexByte(line, '='); i > 0 {
				existing[line[:i]] = true
			}
		}
	}
	changed := false
	for k, v := range vars {
		if existing[k] {
			continue
		}
		lines = append(lines, k+"="+v)
		changed = true
	}
	if !changed && len(lines) > 0 {
		return nil
	}
	body := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return writeFileFmt(path, []byte(body), 0644)
}

func patchConfigForDriver(driver string) error {
	path := "internal/config/config.go"
	if _, err := os.Stat(path); err != nil {
		path = "config/config.go"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		dir := "internal/config"
		if !hasInternalLayout() {
			dir = "config"
		}
		path = filepath.Join(dir, "config.go")
		content := `package config

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Config struct {
	Port        int    ` + "`env:\"PORT\" default:\"8080\"`" + `
	Env         string ` + "`env:\"NITRO_ENV\" default:\"development\"`" + `
	ServiceName string ` + "`env:\"SERVICE_NAME\" default:\"app\"`" + `
` + configFieldLine(driver) + `}

func Load() (Config, error) {
	// Export .env / .env.local to the process (real env vars win) so code that
	// reads os.Getenv — sqladapter.OpenEnv, OTEL, REDIS_ADDR — sees the same
	// values as Config. No-op when the files are absent (containers).
	nitro.LoadDotEnv()
	return nitro.ConfigOf[Config]()
}
`
		return writeFile(path, content+"\n"+driverConfigTypes(driver)+"\n")
	}
	orig := string(b)
	src := ensureDotEnvLoad(orig)
	if driverConfigMarker(driver) != "" && strings.Contains(src, driverConfigMarker(driver)) {
		if src == orig {
			return nil
		}
		return writeFileFmt(path, []byte(src), 0644)
	}
	fieldLine := configFieldLine(driver)
	if fieldLine != "" && !strings.Contains(src, strings.TrimSpace(fieldLine)) {
		// insert before closing brace of Config struct
		idx := strings.Index(src, "type Config struct {")
		if idx >= 0 {
			rest := src[idx:]
			end := strings.Index(rest, "\n}")
			if end >= 0 {
				insertAt := idx + end
				src = src[:insertAt] + "\n" + fieldLine + src[insertAt:]
			}
		}
	}
	types := driverConfigTypes(driver)
	if types != "" && !strings.Contains(src, driverConfigMarker(driver)) {
		src = strings.TrimSpace(src) + "\n\n" + types + "\n"
	}
	return writeFileFmt(path, []byte(src), 0644)
}

// ensureDotEnvLoad upgrades a config.Load generated before .env was exported
// to the process: without it, `go run ./cmd/server` cannot see DATABASE_URL
// because sqladapter.OpenEnv reads os.Getenv while ConfigOf reads .env privately.
func ensureDotEnvLoad(src string) string {
	if strings.Contains(src, "LoadDotEnv(") {
		return src
	}
	return strings.Replace(src, "\treturn nitro.ConfigOf[Config]()",
		"\tnitro.LoadDotEnv()\n\treturn nitro.ConfigOf[Config]()", 1)
}

func configFieldLine(driver string) string {
	switch driver {
	case "postgresql", "mysql", "mariadb", "sqlserver", "sqlite":
		return "\tDB          DBConfig\n"
	case "redis":
		return "\tRedis       RedisConfig\n"
	case "valkey":
		return "\tValkey      nitro.ValkeyConfig\n"
	case "rabbitmq":
		return "\tRabbitMQ    RabbitConfig\n"
	case "nats":
		return "\tNATS        NATSConfig\n"
	case "kafka":
		return "\tKafka       KafkaConfig\n"
	case "minio":
		return "\tMinIO       MinIOConfig\n"
	default:
		return ""
	}
}

func driverConfigMarker(driver string) string {
	switch driver {
	case "postgresql", "mysql", "mariadb", "sqlserver", "sqlite":
		return "type DBConfig struct"
	case "redis":
		return "type RedisConfig struct"
	case "valkey":
		// nitro.ValkeyConfig lives in the framework; nothing to add here.
		return "nitro.ValkeyConfig"
	case "rabbitmq":
		return "type RabbitConfig struct"
	case "nats":
		return "type NATSConfig struct"
	case "kafka":
		return "type KafkaConfig struct"
	case "minio":
		return "type MinIOConfig struct"
	default:
		return ""
	}
}

func driverConfigTypes(driver string) string {
	switch driver {
	case "postgresql":
		return `type DBConfig struct {
	DSN    string ` + "`env:\"DB_DSN,DATABASE_URL\"`" + `
	Schema string ` + "`env:\"DB_SCHEMA\" default:\"public\"`" + `
}`
	case "mysql", "mariadb", "sqlserver", "sqlite":
		return `type DBConfig struct {
	DSN string ` + "`env:\"DB_DSN,DATABASE_URL\"`" + `
}`
	case "redis":
		return `type RedisConfig struct {
	Addr     string ` + "`env:\"REDIS_ADDR\" default:\"localhost:6379\"`" + `
	Password string ` + "`env:\"REDIS_PASSWORD\"`" + `
	DB       int    ` + "`env:\"REDIS_DB\" default:\"0\"`" + `
}`
	case "rabbitmq":
		// No hardcoded default URL/credential here on purpose — the actual
		// per-project random password lives in the generated .env
		// (RABBITMQ_URL). A fixed "guest:guest" fallback baked into every
		// scaffolded app's source would be as bad as the well-known
		// RabbitMQ default itself (see vuln.md V-36).
		return `type RabbitConfig struct {
	URL      string ` + "`env:\"RABBITMQ_URL\"`" + `
	Exchange string ` + "`env:\"RABBITMQ_EXCHANGE\" default:\"nitro.events\"`" + `
	Prefetch int    ` + "`env:\"RABBITMQ_PREFETCH\" default:\"5\"`" + `
}`
	case "nats":
		return `type NATSConfig struct {
	URL      string ` + "`env:\"NATS_URL\" default:\"nats://127.0.0.1:4222\"`" + `
	RetryMax int    ` + "`env:\"NATS_RETRY_MAX\" default:\"3\"`" + `
}`
	case "kafka":
		return `type KafkaConfig struct {
	Brokers  string ` + "`env:\"KAFKA_BROKERS\" default:\"127.0.0.1:9092\"`" + `
	RetryMax int    ` + "`env:\"KAFKA_RETRY_MAX\" default:\"3\"`" + `
}`
	case "minio":
		// AccessKey/SecretKey have no default for the same reason as
		// RabbitMQ above — "minioadmin"/"minioadmin" is MinIO's own
		// documented default and a common scanner target (see vuln.md
		// V-23); the real per-project credential lives in .env.
		return `type MinIOConfig struct {
	Endpoint  string ` + "`env:\"MINIO_ENDPOINT\" default:\"127.0.0.1:9000\"`" + `
	AccessKey string ` + "`env:\"MINIO_ACCESS_KEY\"`" + `
	SecretKey string ` + "`env:\"MINIO_SECRET_KEY\"`" + `
	Bucket    string ` + "`env:\"MINIO_BUCKET\" default:\"app\"`" + `
	UseSSL    bool   ` + "`env:\"MINIO_USE_SSL\" default:\"false\"`" + `
}`
	default:
		return ""
	}
}

func patchDockerCompose(driver, pass string) error {
	const path = "docker-compose.yml"
	svcName, svcBlock, volName := dockerComposeService(driver, pass)
	if svcName == "" {
		return nil
	}

	existing := ""
	if b, err := os.ReadFile(path); err == nil {
		existing = string(b)
		if strings.Contains(existing, "\n  "+svcName+":") || strings.Contains(existing, "\n  "+svcName+":\n") {
			return nil
		}
	}

	if existing == "" {
		body := "# Local development only — do not use these credentials in production.\nservices:\n" + svcBlock
		if volName != "" {
			body += "\nvolumes:\n  " + volName + ":\n"
		}
		return writeFileFmt(path, []byte(body), 0644)
	}

	if !strings.Contains(existing, "services:") {
		existing = "services:\n" + existing
	}
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}

	if idx := strings.Index(existing, "\nvolumes:"); idx >= 0 {
		existing = existing[:idx+1] + svcBlock + existing[idx+1:]
	} else {
		existing += svcBlock
	}

	if volName != "" && !strings.Contains(existing, "\n  "+volName+":") {
		if strings.Contains(existing, "\nvolumes:") {
			existing = strings.Replace(existing, "\nvolumes:\n", "\nvolumes:\n  "+volName+":\n", 1)
		} else {
			existing += "\nvolumes:\n  " + volName + ":\n"
		}
	}
	return writeFileFmt(path, []byte(existing), 0644)
}

func dockerComposeService(driver, pass string) (name, block, volume string) {
	switch driver {
	case "postgresql":
		return "postgres", fmt.Sprintf(`  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: %s
      POSTGRES_DB: app
    ports:
      - "127.0.0.1:5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
`, pass), "postgres_data"
	case "mysql":
		return "mysql", fmt.Sprintf(`  mysql:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: %s
      MYSQL_DATABASE: app
      MYSQL_USER: user
      MYSQL_PASSWORD: %s
    ports:
      - "127.0.0.1:3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
`, pass, pass), "mysql_data"
	case "mariadb":
		return "mariadb", fmt.Sprintf(`  mariadb:
    image: mariadb:11
    environment:
      MYSQL_ROOT_PASSWORD: %s
      MYSQL_DATABASE: app
      MYSQL_USER: user
      MYSQL_PASSWORD: %s
    ports:
      - "127.0.0.1:3306:3306"
    volumes:
      - mariadb_data:/var/lib/mysql
`, pass, pass), "mariadb_data"
	case "sqlserver":
		return "sqlserver", fmt.Sprintf(`  sqlserver:
    image: mcr.microsoft.com/mssql/server:2022-latest
    environment:
      ACCEPT_EULA: "Y"
      MSSQL_SA_PASSWORD: "%s"
    ports:
      - "127.0.0.1:1433:1433"
`, pass), ""
	case "redis":
		return "redis", `  redis:
    image: redis:7-alpine
    ports:
      - "127.0.0.1:6379:6379"
    volumes:
      - redis_data:/data
`, "redis_data"
	case "valkey":
		// Valkey (BSD-3) is the open fork of Redis 7.2; same protocol, same port.
		return "valkey", `  valkey:
    image: valkey/valkey:8-alpine
    ports:
      - "127.0.0.1:6379:6379"
    volumes:
      - valkey_data:/data
    healthcheck:
      test: ["CMD", "valkey-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10
`, "valkey_data"
	case "rabbitmq":
		// Matches RABBITMQ_USER/RABBITMQ_PASSWORD written to .env — avoids
		// shipping the well-known "guest"/"guest" default (vuln.md V-36).
		return "rabbitmq", fmt.Sprintf(`  rabbitmq:
    image: rabbitmq:3.13-management-alpine
    environment:
      RABBITMQ_DEFAULT_USER: nitro
      RABBITMQ_DEFAULT_PASS: %s
    ports:
      - "127.0.0.1:5672:5672"
      - "127.0.0.1:15672:15672"
`, pass), ""
	case "nats":
		return "nats", `  nats:
    image: nats:2-alpine
    command: ["-js", "-sd", "/data", "-m", "8222"]
    ports:
      - "127.0.0.1:4222:4222"
      - "127.0.0.1:8222:8222"
    volumes:
      - nats_data:/data
`, "nats_data"
	case "kafka":
		return "kafka", `  kafka:
    image: apache/kafka:3.9.1
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://127.0.0.1:9092
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@localhost:9093
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS: 0
    ports:
      - "127.0.0.1:9092:9092"
    volumes:
      - kafka_data:/var/lib/kafka/data
`, "kafka_data"
	case "minio":
		// Matches MINIO_ACCESS_KEY/MINIO_SECRET_KEY written to .env —
		// avoids shipping the well-known "minioadmin"/"minioadmin"
		// default (vuln.md V-23).
		return "minio", fmt.Sprintf(`  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: nitro
      MINIO_ROOT_PASSWORD: %s
    ports:
      - "127.0.0.1:9000:9000"
      - "127.0.0.1:9001:9001"
    volumes:
      - minio_data:/data
`, pass), "minio_data"
	default:
		return "", "", ""
	}
}
