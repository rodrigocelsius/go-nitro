package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// nitrogen test — run tests and optionally validate coverage.
//
//	nitrogen test
//	nitrogen test -v --race ./internal/...
//	nitrogen test --cover --min 60
//	nitrogen cover --min 50 --html
func cmdTest(args []string) {
	runTestCommand(args, false)
}

func cmdCover(args []string) {
	runTestCommand(args, true)
}

func runTestCommand(args []string, forceCover bool) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	race := fs.Bool("race", false, "Enable race detector")
	verbose := fs.Bool("v", false, "Verbose test output")
	short := fs.Bool("short", false, "Pass -short to go test")
	cover := fs.Bool("cover", false, "Collect coverage profile")
	minCover := fs.Float64("min", -1, "Minimum total coverage % (implies --cover; fail if below)")
	profile := fs.String("profile", "coverage.out", "Coverage profile path")
	html := fs.Bool("html", false, "Write HTML report next to profile")
	htmlOut := fs.String("html-out", "", "HTML report path (default: <profile>.html)")
	count := fs.String("count", "1", "go test -count value")
	mode := fs.String("covermode", "atomic", "Coverage mode: set|count|atomic")
	exclude := fs.String("exclude", "cmd/", "Comma-separated path fragments shown as filtered metric (does not change -min)")
	packages := fs.String("packages", "./...", "Packages to test (comma-separated)")

	// Aliases style: --verbose, --cover-min (parsed manually before FlagSet)
	args = normalizeTestArgs(args)
	if err := fs.Parse(args); err != nil {
		printTestUsage()
		os.Exit(2)
	}

	pkgs := csvArgs(*packages)
	if fs.NArg() > 0 {
		pkgs = fs.Args()
	}

	wantCover := forceCover || *cover || *minCover >= 0 || *html
	if *minCover >= 0 {
		wantCover = true
	}

	fmt.Println("nitrogen: running tests...")
	if *race {
		fmt.Println("nitrogen: race detection enabled")
	}
	if wantCover {
		fmt.Printf("nitrogen: coverage enabled (mode=%s, profile=%s)\n", *mode, *profile)
	}

	testArgs := []string{"test", "-count=" + *count}
	if *verbose {
		testArgs = append(testArgs, "-v")
	}
	if *short {
		testArgs = append(testArgs, "-short")
	}
	if *race {
		testArgs = append(testArgs, "-race")
	}
	if wantCover {
		testArgs = append(testArgs,
			"-coverprofile="+*profile,
			"-covermode="+*mode,
		)
	}
	testArgs = append(testArgs, pkgs...)

	cmd := exec.Command("go", testArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: tests failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("nitrogen: all tests passed")

	if !wantCover {
		return
	}

	report, err := analyzeCoverage(*profile, csvArgs(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: coverage analysis failed: %v\n", err)
		os.Exit(1)
	}
	printCoverageReport(report)

	if *html {
		out := *htmlOut
		if out == "" {
			out = strings.TrimSuffix(*profile, filepath.Ext(*profile)) + ".html"
			if out == ".html" {
				out = "coverage.html"
			}
		}
		if err := writeCoverHTML(*profile, out); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: html report failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("nitrogen: html report written to %s\n", out)
	}

	if *minCover >= 0 {
		fmt.Printf("nitrogen: coverage check total=%.1f%% min=%.1f%%\n", report.Total, *minCover)
		if report.Total+1e-9 < *minCover {
			fmt.Fprintf(os.Stderr, "nitrogen: coverage below minimum (%.1f%% < %.1f%%)\n", report.Total, *minCover)
			os.Exit(1)
		}
		fmt.Println("nitrogen: coverage OK")
	}
}

func printTestUsage() {
	fmt.Print(`usage: nitrogen test [packages...] [flags]
       nitrogen cover [packages...] [flags]

Flags:
  -v, --verbose       Verbose go test output
  -race, --race       Enable race detector
  -short              Pass -short to go test
  -cover, --cover     Collect coverage profile
  -min float          Minimum coverage %% (implies -cover; exit 1 if below)
  -profile string     Coverage profile path (default coverage.out)
  -covermode string   set|count|atomic (default atomic)
  -html               Write HTML coverage report
  -html-out string    HTML output path
  -packages string    Packages (default ./...); overridden by positional args
  -exclude string     Path fragments for filtered metric (default cmd/)
  -count string       go test -count (default 1)

Examples:
  nitrogen test
  nitrogen test -v --race ./internal/...
  nitrogen test --cover --min 60
  nitrogen cover --min 50 --html ./internal/...
  nitrogen cover --min 70 ./internal/...
`)
}

func normalizeTestArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--verbose":
			out = append(out, "-v")
		case a == "--race":
			out = append(out, "-race")
		case a == "--cover":
			out = append(out, "-cover")
		case a == "--short":
			out = append(out, "-short")
		case a == "--html":
			out = append(out, "-html")
		case a == "--min" || a == "--cover-min" || a == "--threshold":
			out = append(out, "-min")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				out = append(out, args[i])
			}
		case strings.HasPrefix(a, "--min="):
			out = append(out, "-min", strings.TrimPrefix(a, "--min="))
		case strings.HasPrefix(a, "--cover-min="):
			out = append(out, "-min", strings.TrimPrefix(a, "--cover-min="))
		case strings.HasPrefix(a, "--threshold="):
			out = append(out, "-min", strings.TrimPrefix(a, "--threshold="))
		case a == "--profile":
			out = append(out, "-profile")
		case strings.HasPrefix(a, "--profile="):
			out = append(out, "-profile", strings.TrimPrefix(a, "--profile="))
		case a == "--html-out":
			out = append(out, "-html-out")
		case strings.HasPrefix(a, "--html-out="):
			out = append(out, "-html-out", strings.TrimPrefix(a, "--html-out="))
		case a == "--packages":
			out = append(out, "-packages")
		case strings.HasPrefix(a, "--packages="):
			out = append(out, "-packages", strings.TrimPrefix(a, "--packages="))
		case a == "--exclude":
			out = append(out, "-exclude")
		case strings.HasPrefix(a, "--exclude="):
			out = append(out, "-exclude", strings.TrimPrefix(a, "--exclude="))
		case a == "--covermode":
			out = append(out, "-covermode")
		case strings.HasPrefix(a, "--covermode="):
			out = append(out, "-covermode", strings.TrimPrefix(a, "--covermode="))
		case a == "-h" || a == "--help":
			printTestUsage()
			os.Exit(0)
		default:
			out = append(out, a)
		}
	}
	return out
}

func csvArgs(s string) []string {
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

var (
	coverFuncLineRe = regexp.MustCompile(`^(.+):(\d+):\s+(\S+)\s+([\d.]+)%$`)
	coverTotalRe    = regexp.MustCompile(`^total:\s+\(statements\)\s+([\d.]+)%$`)
)

type coverFunc struct {
	File string
	Line int
	Func string
	Pct  float64
}

type coverageReport struct {
	Total       float64
	Funcs       []coverFunc
	Filtered    bool
	FilteredPct float64
	Excluded    int
}

func analyzeCoverage(profile string, exclude []string) (*coverageReport, error) {
	cmd := exec.Command("go", "tool", "cover", "-func="+profile)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go tool cover: %w", err)
	}
	return parseCoverFunc(buf.String(), exclude)
}

func parseCoverFunc(text string, exclude []string) (*coverageReport, error) {
	rep := &coverageReport{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if m := coverTotalRe.FindStringSubmatch(line); m != nil {
			pct, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				return nil, err
			}
			rep.Total = pct
			continue
		}
		m := coverFuncLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ln, _ := strconv.Atoi(m[2])
		pct, err := strconv.ParseFloat(m[4], 64)
		if err != nil {
			return nil, err
		}
		rep.Funcs = append(rep.Funcs, coverFunc{
			File: m[1],
			Line: ln,
			Func: m[3],
			Pct:  pct,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	if len(exclude) == 0 || len(rep.Funcs) == 0 {
		return rep, nil
	}

	var sum float64
	var n int
	for _, f := range rep.Funcs {
		if coverExcluded(f.File, exclude) {
			rep.Excluded++
			continue
		}
		sum += f.Pct
		n++
	}
	if n > 0 && rep.Excluded > 0 {
		rep.Filtered = true
		rep.FilteredPct = sum / float64(n)
	}
	return rep, nil
}

func coverExcluded(file string, exclude []string) bool {
	file = filepath.ToSlash(file)
	for _, ex := range exclude {
		ex = filepath.ToSlash(strings.TrimSpace(ex))
		if ex == "" {
			continue
		}
		if strings.Contains(file, ex) {
			return true
		}
	}
	return false
}

func printCoverageReport(r *coverageReport) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║         NITRO COVERAGE REPORT        ║")
	fmt.Println("╠══════════════════════════════════════╣")
	fmt.Printf("║ Total (statements):  %14.1f%% ║\n", r.Total)
	if r.Filtered {
		fmt.Printf("║ Filtered avg (fn):   %14.1f%% ║\n", r.FilteredPct)
		fmt.Printf("║ Excluded functions:  %15d ║\n", r.Excluded)
	}
	fmt.Printf("║ Functions measured:  %15d ║\n", len(r.Funcs))
	fmt.Println("╚══════════════════════════════════════╝")
	if r.Filtered {
		fmt.Println("note: -min uses Total (statements); filtered is informational")
	}
}

func writeCoverHTML(profile, out string) error {
	cmd := exec.Command("go", "tool", "cover", "-html="+profile, "-o", out)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
