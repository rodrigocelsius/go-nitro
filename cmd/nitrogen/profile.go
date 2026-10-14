package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func cmdProfile(args []string) {
	fs := flag.NewFlagSet("profile", flag.ExitOnError)
	output := fs.String("o", "", "Output file (default: <type>.pprof)")
	duration := fs.Duration("d", 30*time.Second, "CPU profile duration")
	server := fs.String("server", "http://localhost:3080", "Server URL (must have pprof enabled)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("usage: nitrogen profile <type> [flags]")
		fmt.Println("")
		fmt.Println("Types:")
		fmt.Println("  cpu         CPU profile (30s default)")
		fmt.Println("  heap        Heap memory profile")
		fmt.Println("  goroutine   Goroutine profile")
		fmt.Println("  block       Block profile")
		fmt.Println("  mutex       Mutex profile")
		fmt.Println("  allocs      Allocation profile")
		fmt.Println("  threadcreate Thread creation profile")
		fmt.Println("")
		fmt.Println("Flags:")
		fmt.Println("  -o string     Output file (default: <type>.pprof)")
		fmt.Println("  -d duration   CPU profile duration (default: 30s)")
		fmt.Println("  -server url   Server URL (default: http://localhost:3080)")
		fmt.Println("")
		fmt.Println("Note: Server must have pprof enabled.")
		fmt.Println("  fasthttp.SetupPprof(app, authMiddleware) // auth required in production")
		os.Exit(1)
	}

	profileType := fs.Arg(0)

	if *output == "" {
		*output = profileType + ".pprof"
	}

	url := *server + "/debug/pprof/" + profileType

	fmt.Printf("nitrogen: collecting %s profile from %s...\n", profileType, url)

	var resp *http.Response
	var err error

	switch profileType {
	case "cpu":
		url += "?seconds=" + fmt.Sprintf("%d", int(duration.Seconds()))
		resp, err = collectProfile(url, *duration+5*time.Second)
	default:
		resp, err = collectProfile(url, 10*time.Second)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: failed to collect profile: %v\n", err)
		fmt.Println("nitrogen: make sure pprof is enabled on the server:")
		fmt.Println("  fasthttp.SetupPprof(app, authMiddleware) // auth required in production")
		os.Exit(1)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: failed to read profile: %v\n", err)
		os.Exit(1)
	}

	if err := writeFileFmt(*output, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: failed to write profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("nitrogen: profile saved to %s (%d bytes)\n", *output, len(data))
	fmt.Printf("nitrogen: analyze with: go tool pprof %s\n", *output)
}

func collectProfile(url string, timeout time.Duration) (*http.Response, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	return resp, nil
}

func cmdPprof(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: nitrogen pprof <profile-file>")
		fmt.Println("")
		fmt.Println("Opens a pprof profile in the browser.")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  nitrogen pprof cpu.pprof")
		os.Exit(1)
	}

	profileFile := args[0]

	if _, err := os.Stat(profileFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "nitrogen: file not found: %s\n", profileFile)
		os.Exit(1)
	}

	fmt.Printf("nitrogen: opening %s in pprof...\n", profileFile)
	cmd := exec.Command("go", "tool", "pprof", "-http=:8080", profileFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: pprof failed: %v\n", err)
		os.Exit(1)
	}
}
