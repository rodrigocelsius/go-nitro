package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// detectNitroPath finds a local Nitro monorepo checkout by walking parents of cwd
// (and optionally NITRO_PATH). Looks for go.mod with module github.com/rodrigocelsius/go-nitro/v2
// and a pkg/ subdirectory. Explicit --nitro-path always wins at the call site.
func detectNitroPath() string {
	if v := strings.TrimSpace(os.Getenv("NITRO_PATH")); v != "" {
		if abs, err := filepath.Abs(v); err == nil && isNitroRoot(abs) {
			return abs
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if isNitroRoot(dir) {
			return dir
		}
		if p := nitroRootFromGoWork(dir); p != "" {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func isNitroRoot(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	mod := modulePathFromGoMod(string(b))
	if mod != "github.com/rodrigocelsius/go-nitro/v2" {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "pkg", "go.mod")); err != nil {
		return false
	}
	return true
}

func modulePathFromGoMod(src string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// nitroRootFromGoWork reads go.work use directives and returns the first path that is a Nitro root.
func nitroRootFromGoWork(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "go.work"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "use ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "use "))
		rest = strings.Trim(rest, `"`)
		if rest == "" {
			continue
		}
		cand := rest
		if !filepath.IsAbs(cand) {
			cand = filepath.Join(dir, rest)
		}
		if abs, err := filepath.Abs(cand); err == nil && isNitroRoot(abs) {
			return abs
		}
		// use ./pkg inside a Nitro checkout
		parent := filepath.Dir(absOr(cand))
		if isNitroRoot(parent) {
			return parent
		}
	}
	return ""
}

func absOr(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// resolveNitroPath returns explicit path or auto-detected checkout.
func resolveNitroPath(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	if p := detectNitroPath(); p != "" {
		fmt.Printf("nitrogen: using local Nitro at %s (auto-detected)\n", p)
		return p
	}
	return ""
}
