package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultFleetRouting maps service name → queue + routing key (NFe-compatible defaults).
var defaultFleetRouting = map[string][2]string{
	"gateway":    {"", ""},
	"nfe":        {"nfe_processing_queue", "nfe.send"},
	"nfce":       {"nfce_processing_queue", "nfce.send"},
	"nfse":       {"nfse_processing_queue", "nfse.send"},
	"danfe":      {"danfe_generation_queue", "danfe.send"},
	"cancel":     {"cancel_processing_queue", "nfe.cancel.send"},
	"cce":        {"cce_processing_queue", "nfe.cce.send"},
	"inutilizar": {"inutilizar_processing_queue", "nfe.inutilizar.send"},
	"mde":        {"mde_processing_queue", "nfe.mde.send"},
	"cte":        {"cte_processing_queue", "nfe.cte.send"},
	"mdfe":       {"mdfe_processing_queue", "nfe.mdfe.send"},
}

type newOpts struct {
	tier      string // basic | microservice | worker | fleet | default
	module    string
	name      string
	services  []string
	nitroPath string
	broker    string // fleet: nats (default) | rabbitmq
}

func parseNewArgs(args []string) (newOpts, bool) {
	var o newOpts
	if len(args) < 1 {
		return o, false
	}
	o.name = args[0]
	o.tier = "default"
	o.module = args[0]
	for i := 1; i < len(args); i++ {
		switch {
		case args[i] == "--basic":
			o.tier = "basic"
		case args[i] == "--microservice":
			o.tier = "microservice"
		case args[i] == "--worker":
			o.tier = "worker"
		case args[i] == "--fleet":
			o.tier = "fleet"
		case args[i] == "--module":
			if i+1 >= len(args) {
				return o, false
			}
			o.module = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--module="):
			o.module = strings.TrimPrefix(args[i], "--module=")
		case args[i] == "--services":
			if i+1 >= len(args) {
				return o, false
			}
			o.services = splitCSV(args[i+1])
			i++
		case strings.HasPrefix(args[i], "--services="):
			o.services = splitCSV(strings.TrimPrefix(args[i], "--services="))
		case args[i] == "--broker":
			if i+1 >= len(args) {
				return o, false
			}
			o.broker = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--broker="):
			o.broker = strings.TrimPrefix(args[i], "--broker=")
		case args[i] == "--nitro-path":
			if i+1 >= len(args) {
				return o, false
			}
			o.nitroPath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--nitro-path="):
			o.nitroPath = strings.TrimPrefix(args[i], "--nitro-path=")
		case args[i] == "--standard", args[i] == "--complex", args[i] == "--stack", strings.HasPrefix(args[i], "--stack="):
			fmt.Fprintf(os.Stderr, "nitrogen: flag %q removed; use default, --basic, --microservice, --worker, or --fleet\n", args[i])
			return o, false
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "nitrogen: unknown flag %s\n", args[i])
				return o, false
			}
		}
	}
	return o, true
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envSkeleton() string {
	return "PORT=8080\nNITRO_ENV=development\nSERVICE_NAME=app\n"
}

// brokerEnv is the worker .env block for a broker: nats (JetStream, the fleet
// default) or rabbitmq.
func brokerEnv(broker string) string {
	if broker == "rabbitmq" {
		return "RABBITMQ_URL=amqp://guest:guest@127.0.0.1:5672/\nRABBITMQ_EXCHANGE=nitro.events\nRABBITMQ_PREFETCH=5\n"
	}
	return "NATS_URL=nats://127.0.0.1:4222\nNATS_RETRY_MAX=3\n"
}

func workerEnvSkeleton(name, broker string) string {
	queue, rk := name+"_queue", name
	if pair, ok := defaultFleetRouting[name]; ok && pair[0] != "" {
		queue, rk = pair[0], pair[1]
	}
	return fmt.Sprintf(`HEALTH_PORT=8081
NITRO_ENV=development
SERVICE_NAME=%s
QUEUE_NAME=%s
ROUTING_KEY=%s
`, name, queue, rk) + brokerEnv(broker)
}

func projectTemplates(name, module, tier string) map[string]string {
	if tier == "basic" {
		return map[string]string{
			"main.go":       readTemplate("minimal_main.go.tmpl"),
			"go.mod":        sprintfTemplate("go.mod.tmpl", module, frameworkVersion),
			".gitignore":    readTemplate("gitignore.tmpl"),
			".dockerignore": readTemplate("dockerignore.tmpl"),
			"README.md":     sprintfTemplate("readme.md.tmpl", name),
			".env":          envSkeleton(),
			".env.example":  envSkeleton(),
		}
	}

	if tier == "worker" {
		return workerTemplates(name, module, "rabbitmq")
	}

	env := envSkeleton()
	files := map[string]string{
		"cmd/server/main.go":           sprintfTemplate("ca_main.go.tmpl", module),
		"internal/config/config.go":    readTemplate("ca_config.go.tmpl"),
		"internal/handler/routes.go":   readTemplate("ca_routes.go.tmpl"),
		"internal/handler/health.go":   readTemplate("ca_health.go.tmpl"),
		"internal/model/.gitkeep":      "",
		"internal/usecase/.gitkeep":    "",
		"internal/repository/.gitkeep": "",
		"migrations/.gitkeep":          "",
		"go.mod":                       sprintfTemplate("go.mod.tmpl", module, frameworkVersion),
		".gitignore":                   readTemplate("gitignore.tmpl"),
		".dockerignore":                readTemplate("dockerignore.tmpl"),
		"README.md":                    sprintfTemplate("readme.md.tmpl", name),
		"Makefile":                     readTemplate("ca_makefile.tmpl"),
		"Dockerfile":                   readTemplate("ca_dockerfile.tmpl"),
		".env":                         env,
		".env.example":                 env,
	}

	if tier == "microservice" {
		files["docker-compose.yml"] = readTemplate("ms_docker_compose.yml.tmpl")
		files["cmd/server/main.go"] = sprintfTemplate("ca_main_ms.go.tmpl", module)
	}

	return files
}

func workerTemplates(name, module, broker string) map[string]string {
	env := workerEnvSkeleton(name, broker)
	return map[string]string{
		"cmd/worker/main.go":            sprintfTemplate("worker_main.go.tmpl", module),
		"internal/config/config.go":     readTemplate("worker_config.go.tmpl"),
		"internal/consumer/consumer.go": sprintfTemplate("worker_consumer.go.tmpl", module),
		"internal/usecase/.gitkeep":     "",
		"go.mod":                        sprintfTemplate("go.mod.tmpl", module, frameworkVersion),
		".gitignore":                    readTemplate("gitignore.tmpl"),
		".dockerignore":                 readTemplate("dockerignore.tmpl"),
		"README.md":                     sprintfTemplate("readme.md.tmpl", name),
		"Makefile":                      readTemplate("worker_makefile.tmpl"),
		"Dockerfile":                    readTemplate("worker_dockerfile.tmpl"),
		".env":                          env,
		".env.example":                  env,
	}
}

func writeProjectFiles(root string, files map[string]string) error {
	for file, content := range files {
		path := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create dir for %s: %w", path, err)
		}
		body := content
		if content != "" {
			body = strings.TrimSpace(content) + "\n"
		}
		if err := writeFileFmt(path, []byte(body), 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// replaceTarget returns the Nitro checkout path relative to the go.mod
// directory ("../../go-nitro"), so generated projects stay portable across
// machines, containers and CI. Falls back to the absolute path when no
// relative form exists (e.g. a different Windows volume).
func replaceTarget(goModDir, nitroRoot string) string {
	modDir, err := filepath.Abs(goModDir)
	if err != nil {
		return filepath.ToSlash(nitroRoot)
	}
	rel, err := filepath.Rel(modDir, nitroRoot)
	if err != nil {
		return filepath.ToSlash(nitroRoot)
	}
	rel = filepath.ToSlash(rel)
	if rel != "." && !strings.HasPrefix(rel, "../") && rel != ".." {
		rel = "./" + rel
	}
	return rel
}

func appendNitroReplace(goModPath, nitroPath string) error {
	if nitroPath == "" {
		return nil
	}
	abs, err := filepath.Abs(nitroPath)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(goModPath)
	if err != nil {
		return err
	}
	src := string(b)
	if strings.Contains(src, "replace github.com/rodrigocelsius/go-nitro/pkg/v2") {
		return nil
	}
	mods := []string{
		"pkg",
		"pkg/uuid",
		"domain",
		"ports",
		"usecases",
		"scheduler",
		"queue",
		"resilience",
		"adapters/amqp",
		"adapters/nats",
		"adapters/kafka",
		"adapters/fasthttp",
		"adapters/cache",
		"adapters/otel",
		"adapters/ratelimit",
		"adapters/auth/jwt",
		"adapters/auth/keycloak",
		"adapters/auth/bcrypt",
		"adapters/auth/session",
		"adapters/auth/oauth2",
		"adapters/auth/oidc",
		"codegen/openapi",
		"data/gateway",
		"data/adapter/pgx",
		"data/adapter/sql",
		"blinq",
	}
	root := replaceTarget(filepath.Dir(goModPath), abs)
	var bld strings.Builder
	bld.WriteString("\n\nreplace (\n")
	for _, m := range mods {
		fmt.Fprintf(&bld, "\tgithub.com/rodrigocelsius/go-nitro/%s/v2 => %s/%s\n", m, root, m)
	}
	bld.WriteString(")\n")
	src = strings.TrimSpace(src) + bld.String()
	return writeFileFmt(goModPath, []byte(src), 0644)
}
