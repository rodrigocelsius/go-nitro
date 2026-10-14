package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migrationgen "github.com/rodrigocelsius/go-nitro/codegen/migration/v2"
)

// upsertEnvVars writes vars into path, replacing existing keys (for engine switch).
func upsertEnvVars(path string, vars map[string]string) error {
	if len(vars) == 0 {
		return nil
	}
	order := make([]string, 0, len(vars))
	for k := range vars {
		order = append(order, k)
	}
	// stable-ish: prefer known SQL keys first
	prefer := []string{"NITRO_DB_DRIVER", "DATABASE_URL", "DB_DSN", "DB_SCHEMA", "JWT_SECRET",
		"KEYCLOAK_BASE_URL", "KEYCLOAK_REALM", "KEYCLOAK_CLIENT_ID", "KEYCLOAK_CLIENT_SECRET"}
	seen := map[string]bool{}
	sorted := make([]string, 0, len(vars))
	for _, k := range prefer {
		if _, ok := vars[k]; ok {
			sorted = append(sorted, k)
			seen[k] = true
		}
	}
	for _, k := range order {
		if !seen[k] {
			sorted = append(sorted, k)
		}
	}

	existingLines := []string{}
	keyLine := map[string]int{} // key -> index in existingLines
	if b, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			existingLines = append(existingLines, line)
			trim := strings.TrimSpace(line)
			if trim == "" || strings.HasPrefix(trim, "#") {
				continue
			}
			if i := strings.IndexByte(trim, '='); i > 0 {
				keyLine[trim[:i]] = len(existingLines) - 1
			}
		}
	}
	for _, k := range sorted {
		v := vars[k]
		entry := k + "=" + v
		if idx, ok := keyLine[k]; ok {
			existingLines[idx] = entry
		} else {
			existingLines = append(existingLines, entry)
			keyLine[k] = len(existingLines) - 1
		}
	}
	body := strings.TrimRight(strings.Join(existingLines, "\n"), "\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return writeFileFmt(path, []byte(body), 0644)
}

func writeInstallEnv(driver string, vars map[string]string) {
	writer := appendEnvVars
	if isSQLDriver(driver) || driver == "auth-jwt" || driver == "auth-keycloak" || driver == "jwt" || driver == "keycloak" {
		writer = upsertEnvVars
	}
	for _, envFile := range []string{".env", ".env.example"} {
		if err := writer(envFile, vars); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: %s: %v\n", envFile, err)
		} else {
			fmt.Printf("nitrogen: updated %s\n", envFile)
		}
	}
}

func warnEngineSwitch(newDriver string) {
	prev := strings.TrimSpace(dotenvValue("NITRO_DB_DRIVER"))
	if prev == "" {
		prev = strings.TrimSpace(os.Getenv("NITRO_DB_DRIVER"))
	}
	if prev == "" || prev == newDriver || !isSQLDriver(newDriver) {
		return
	}
	fmt.Printf("nitrogen: switching SQL engine %s → %s\n", prev, newDriver)
	if dirExists("migrations") {
		entries, _ := os.ReadDir("migrations")
		n := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".sql") {
				n++
			}
		}
		if n > 0 {
			oldD := migrationgen.DialectFromDriver(prev)
			newD := migrationgen.DialectFromDriver(newDriver)
			fmt.Printf("nitrogen: warning — %d migration(s) present; dialects %s → %s may differ\n", n, oldD, newD)
			fmt.Println("nitrogen: tip — review migrations/ or regenerate: nitrogen migrate:make internal/model/<file>.go")
			fmt.Println("nitrogen: tip — local tests without Docker: nitrogen install sqlite && nitrogen doctor --db")
		}
	}
}

// commentRivalSQLServices comments out other SQL engines in docker-compose when switching.
func commentRivalSQLServices(keep string) {
	const path = "docker-compose.yml"
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	rivals := map[string][]string{
		"postgresql": {"mysql", "mariadb", "sqlserver"},
		"mysql":      {"postgres", "mariadb", "sqlserver"},
		"mariadb":    {"postgres", "mysql", "sqlserver"},
		"sqlserver":  {"postgres", "mysql", "mariadb"},
		"sqlite":     {"postgres", "mysql", "mariadb", "sqlserver"},
	}
	list, ok := rivals[keep]
	if !ok {
		return
	}
	src := string(b)
	changed := false
	for _, name := range list {
		marker := "\n  " + name + ":"
		if !strings.Contains(src, marker) {
			continue
		}
		// Already commented?
		if strings.Contains(src, "\n  # "+name+":") || strings.Contains(src, "\n#  "+name+":") {
			continue
		}
		src = commentComposeService(src, name)
		changed = true
	}
	if changed {
		_ = writeFileFmt(path, []byte(src), 0644)
		fmt.Println("nitrogen: commented rival SQL services in docker-compose.yml (review before up)")
	}
}

func commentComposeService(src, name string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	in := false
	for _, line := range lines {
		if strings.HasPrefix(line, "  "+name+":") {
			in = true
			out = append(out, "# "+line+"  # disabled by nitrogen install (engine switch)")
			continue
		}
		if in {
			// end of service: next top-level service or volumes
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.Contains(line, ":") {
				in = false
				out = append(out, line)
				continue
			}
			if line == "volumes:" || strings.HasPrefix(line, "volumes:") {
				in = false
				out = append(out, line)
				continue
			}
			if line == "" {
				in = false
				out = append(out, line)
				continue
			}
			out = append(out, "# "+line)
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
