package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func cmdSecurity(args []string) {
	fs := flag.NewFlagSet("sec", flag.ExitOnError)
	url := fs.String("url", "http://localhost:3080", "Target URL")
	fs.Parse(args)

	if fs.NArg() > 0 {
		sub := fs.Arg(0)
		switch sub {
		case "headers":
			testSecurityHeaders(*url)
		case "csrf":
			testCSRF(*url)
		case "xss":
			testXSS(*url)
		case "all":
			testSecurityHeaders(*url)
			testCSRF(*url)
			testXSS(*url)
		default:
			fmt.Printf("unknown security test: %s\n", sub)
			printSecurityUsage()
		}
		return
	}
	testSecurityHeaders(*url)
}

func printSecurityUsage() {
	fmt.Println("usage: nitrogen security <test> [flags]")
	fmt.Println("")
	fmt.Println("Tests:")
	fmt.Println("  headers     Check security headers")
	fmt.Println("  csrf        Test CSRF protection")
	fmt.Println("  xss         Test XSS reflection")
	fmt.Println("  all         Run all security tests")
	fmt.Println("")
	fmt.Println("Flags:")
	fmt.Println("  -url string   Target URL (default: http://localhost:3080)")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  nitrogen security headers -url http://localhost:3080")
	fmt.Println("  nitrogen security all -url http://localhost:3080")
	os.Exit(1)
}

func testSecurityHeaders(url string) {
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║        SECURITY HEADERS CHECK           ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("  ✗ Failed to connect: %v\n", err)
		return
	}
	defer resp.Body.Close()

	expected := map[string]string{
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"X-Xss-Protection":                  "1; mode=block",
		"Referrer-Policy":                   "",
		"Content-Security-Policy":           "",
		"Strict-Transport-Security":         "",
		"X-Permitted-Cross-Domain-Policies": "",
		"Permissions-Policy":                "",
	}

	pass := true

	for header, want := range expected {
		got := resp.Header.Get(header)
		detail := ""
		if got == "" {
			detail = "MISSING"
			pass = false
		} else if want != "" && got != want {
			detail = fmt.Sprintf("expected %q, got %q", want, got)
			pass = false
		} else {
			detail = got
		}

		if strings.Contains(detail, "MISSING") {
			fmt.Printf("  ✗ %s: %s\n", header, detail)
		} else if strings.Contains(detail, "expected") {
			fmt.Printf("  ⚠ %s: %s\n", header, detail)
		} else {
			fmt.Printf("  ✓ %s: %s\n", header, detail)
		}
	}

	fmt.Println()

	if !pass {
		fmt.Println("nitrogen: some security headers are missing or incorrect")
	} else {
		fmt.Println("nitrogen: all security headers present")
	}
}

func testCSRF(url string) {
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║        CSRF PROTECTION CHECK            ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	client := &http.Client{Timeout: 5 * time.Second}

	// Test 1: POST without CSRF token
	req, _ := http.NewRequest("POST", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  ✗ Connection failed: %v\n", err)
		return
	}
	resp.Body.Close()

	sameSite := resp.Header.Get("Set-Cookie")

	fmt.Printf("  POST without CSRF token: %d", resp.StatusCode)
	switch {
	case resp.StatusCode == 403 || resp.StatusCode == 401:
		fmt.Println(" (protected)")
	case resp.StatusCode >= 400:
		fmt.Println(" (rejected)")
	default:
		fmt.Println(" ⚠ accepted (no CSRF protection)")
	}

	if sameSite != "" {
		fmt.Printf("  Set-Cookie: %s\n", sameSite)
	}

	// Test 2: Check for SameSite cookie attribute
	req2, _ := http.NewRequest("GET", url, nil)
	resp2, err := client.Do(req2)
	if err == nil {
		cookies := resp2.Header["Set-Cookie"]
		for _, c := range cookies {
			if strings.Contains(c, "SameSite") {
				fmt.Printf("  ✓ SameSite cookie attribute found\n")
			}
		}
		resp2.Body.Close()
	}

	fmt.Println()
	fmt.Println("nitrogen: CSRF check complete")
}

func testXSS(url string) {
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║        XSS REFLECTION CHECK             ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	client := &http.Client{Timeout: 5 * time.Second}

	payloads := []struct {
		name string
		url  string
	}{
		{"Simple script", url + "?q=<script>alert(1)</script>"},
		{"On error", url + "?q=<img src=x onerror=alert(1)>"},
		{"SVG onload", url + "?q=<svg onload=alert(1)>"},
		{"JS event", url + "?q=\" onmouseover=alert(1)"},
	}

	reflected := false
	for _, p := range payloads {
		resp, err := client.Get(p.url)
		if err != nil {
			fmt.Printf("  - %s: connection failed\n", p.name)
			continue
		}

		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		resp.Body.Close()

		bodyStr := string(body[:n])
		if strings.Contains(bodyStr, "<script>") ||
			strings.Contains(bodyStr, "onerror=") ||
			strings.Contains(bodyStr, "onload=") ||
			strings.Contains(bodyStr, "onmouseover") {
			fmt.Printf("  ✗ %s: potential XSS reflection detected\n", p.name)
			reflected = true
		} else {
			fmt.Printf("  ✓ %s: not reflected\n", p.name)
		}
	}

	fmt.Println()
	if reflected {
		fmt.Println("nitrogen: ⚠ potential XSS reflection detected - review input sanitization")
	} else {
		fmt.Println("nitrogen: no XSS reflection detected")
	}
}
