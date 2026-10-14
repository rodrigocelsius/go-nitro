package fasthttp

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestPrintStartupBanner(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	app := New()
	app.Get("/x", func(c *domain.Ctx) error { return nil })
	app.printStartupBanner(":8000", false)
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	for _, part := range []string{"HOST", "PORT", "8000", "TLS", "ROUTES", "OS", "CORES", "MEM", "PPID", "_   _ __", "/ | /"} {
		if !strings.Contains(out, part) {
			t.Fatalf("missing %q in banner:\n%s", part, out)
		}
	}
	t.Log("\n" + out)
}

func TestPrintStartupBannerDisabled(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	app := New(domain.Config{DisableStartupMessage: true})
	app.printStartupBanner(":8000", false)
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if buf.Len() != 0 {
		t.Fatalf("expected empty banner, got %q", buf.String())
	}
}
