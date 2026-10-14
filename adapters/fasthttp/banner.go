package fasthttp

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// Version shown on the startup banner (update when publishing).
const Version = "2.0.0"

const (
	cBlack = "\u001b[30m"
	cCyan  = "\u001b[36m"
	cReset = "\u001b[0m"
)

// printStartupBanner prints the nitro ASCII (Fiber-style) plus metadata.
func (a *App) printStartupBanner(addr string, isTLS bool) {
	if a.disableStartupMessage {
		return
	}

	host, port := splitHostPort(addr)
	if host == "" {
		host = "0.0.0.0"
	}
	tlsStr := "FALSE"
	if isTLS {
		tlsStr = "TRUE"
	}

	color := shouldColor("auto")
	black, cyan, reset := "", "", ""
	if color {
		black, cyan, reset = cBlack, cCyan, cReset
	}
	cy := func(v any) string {
		if !color {
			return fmt.Sprint(v)
		}
		return cCyan + fmt.Sprint(v) + cBlack
	}

	osName := strings.ToUpper(runtime.GOOS)
	cpuCores := runtime.NumCPU()
	memTotal := formatMemTotal()
	routesLen := len(a.Routes())
	ppid := os.Getppid()

	lines := []string{
		black + "        _   _ __" + black,
		cyan + "  ____" + black + " / | / (_) /__________  " +
			fmt.Sprintf(" HOST   %s\tOS    %s", cy(host), cy(osName)),
		cyan + "_____" + black + " /  |/ / / __/ ___/ __ \\ " +
			fmt.Sprintf(" PORT   %s\tCORES %s", cy(port), cy(cpuCores)),
		cyan + "  __" + black + " / /|  / / /_/ /  / /_/ / " +
			fmt.Sprintf(" TLS    %s\tMEM   %s", cy(tlsStr), cy(memTotal)),
		black + "    /_/ |_/_/\\__/_/   \\____/ " +
			cy(Version) + fmt.Sprintf(" ROUTES %s\tPPID  %s%s", cy(routesLen), cy(ppid), reset),
	}

	fmt.Fprintln(os.Stdout, strings.Join(lines, "\n"))
	scheme := "http"
	if isTLS {
		scheme = "https"
	}
	displayHost := host
	if displayHost == "" || displayHost == "0.0.0.0" {
		displayHost = "127.0.0.1"
	}
	fmt.Fprintf(os.Stdout, "\n  Listening on %s://%s:%s\n\n", scheme, displayHost, port)
}

func splitHostPort(addr string) (host, port string) {
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[:i], addr[i+1:]
	}
	return addr, ""
}

func formatMemTotal() string {
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, _ := strconv.ParseUint(fields[1], 10, 64)
					return humanBytes(kb * 1024)
				}
			}
		}
	}
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err == nil {
		return humanBytes(uint64(info.Totalram) * uint64(info.Unit)) //nolint:unconvert
	}
	return "?"
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatUint(n, 10) + "B"
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(n)/float64(div), "KMGTPE"[exp])
}
