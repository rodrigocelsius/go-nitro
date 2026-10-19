package fasthttp

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Regression: header lookups must be case-insensitive on a real server.
func TestServer_HeaderNamesAreCaseInsensitive(t *testing.T) {
	app := New()
	app.Get("/h", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.Text(fc.Header("Authorization") + "|" + fc.Header("X-Custom-Thing"))
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Serve(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	for _, name := range []string{"authorization", "AUTHORIZATION", "Authorization"} {
		conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(conn, "GET /h HTTP/1.1\r\nHost: x\r\n%s: Bearer abc\r\nx-custom-thing: v\r\nConnection: close\r\n\r\n", name)
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var body strings.Builder
		sc := bufio.NewScanner(conn)
		for sc.Scan() {
			body.WriteString(sc.Text() + "\n")
		}
		conn.Close()
		if !strings.Contains(body.String(), "Bearer abc|v") {
			t.Fatalf("header %q not visible to c.Header: %s", name, body.String())
		}
	}
}
