package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var aiInitRelFiles = []string{
	"AGENTS.md",
	"CLAUDE.md",
	"AI.md",
	".cursor/rules/nitro.mdc",
	".cursor/prompts/nitro-blinq.md",
}

func aiInitTemplateName(rel string) string {
	switch rel {
	case "AGENTS.md":
		return "ai/AGENTS.md.tmpl"
	case "CLAUDE.md":
		return "ai/CLAUDE.md.tmpl"
	case "AI.md":
		return "ai/AI.md.tmpl"
	case ".cursor/rules/nitro.mdc":
		return "ai/nitro.mdc.tmpl"
	case ".cursor/prompts/nitro-blinq.md":
		return "ai/nitro-blinq.md.tmpl"
	default:
		return ""
	}
}

type aiInitOpts struct {
	dir   string
	force bool
}

func parseAiInitArgs(args []string) (aiInitOpts, error) {
	o := aiInitOpts{dir: "."}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--force":
			o.force = true
		case a == "--dir" && i+1 < len(args):
			o.dir = args[i+1]
			i++
		case strings.HasPrefix(a, "--dir="):
			o.dir = strings.TrimPrefix(a, "--dir=")
		case a == "-h" || a == "--help":
			fmt.Println("usage: nitrogen ai:init [--dir <path>] [--force]")
			os.Exit(0)
		case strings.HasPrefix(a, "-"):
			return o, fmt.Errorf("nitrogen: unknown flag %s", a)
		default:
			return o, fmt.Errorf("nitrogen: unexpected argument %s", a)
		}
	}
	if o.dir == "" {
		o.dir = "."
	}
	return o, nil
}

func applyAiInit(dir string, force bool) (written, skipped []string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, nil, err
	}
	for _, rel := range aiInitRelFiles {
		path := filepath.Join(abs, rel)
		if !force {
			if _, err := os.Stat(path); err == nil {
				skipped = append(skipped, rel)
				continue
			}
		}
		body := strings.TrimSpace(readTemplate(aiInitTemplateName(rel))) + "\n"
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return written, skipped, err
		}
		if err := writeFileFmt(path, []byte(body), 0644); err != nil {
			return written, skipped, err
		}
		written = append(written, rel)
	}
	return written, skipped, nil
}

func cmdAiInit(args []string) {
	o, err := parseAiInitArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	written, skipped, err := applyAiInit(o.dir, o.force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: ai:init: %v\n", err)
		os.Exit(1)
	}
	if len(written) > 0 {
		fmt.Printf("nitrogen: ai:init wrote %s\n", strings.Join(written, ", "))
	}
	if len(skipped) > 0 {
		fmt.Printf("nitrogen: ai:init skipped (exists, use --force): %s\n", strings.Join(skipped, ", "))
	}
	if len(written) == 0 && len(skipped) == 0 {
		fmt.Println("nitrogen: ai:init: nothing to do")
	}
}

func initAiRules(dir string) {
	written, skipped, err := applyAiInit(dir, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: ai:init: %v\n", err)
		return
	}
	if len(written) > 0 {
		fmt.Printf("nitrogen: AI agent rules: %s\n", strings.Join(written, ", "))
	}
	if len(skipped) > 0 && len(written) == 0 {
		fmt.Println("nitrogen: AI agent rules already present")
	}
}
