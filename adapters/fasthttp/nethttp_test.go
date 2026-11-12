package fasthttp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func bridgeApp() *App {
	app := New(domain.Config{MaxRequestBodySize: 1 << 10})
	app.Use(func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetHeader("X-Mw", "ran")
		return nil
	})
	app.Get("/users/:id", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.JSON(200, map[string]string{
			"id": c.Param("id"), "q": fc.Query("q"), "proto": fc.Protocol(),
			"ip": fc.FasthttpCtx().RemoteIP().String(), "ua": fc.Header("User-Agent"),
		})
	})
	app.Post("/echo", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.Write(201, fc.FasthttpCtx().PostBody())
	})
	app.Get("/missing-entity", func(c *domain.Ctx) error { return domain.ErrNotFound })
	return app
}

// h2cClient speaks HTTP/2 over cleartext with prior knowledge.
func h2cClient() *http.Client {
	p := new(http.Protocols)
	p.SetUnencryptedHTTP2(true)
	return &http.Client{Transport: &http.Transport{Protocols: p}, Timeout: 5 * time.Second}
}

func serveH2C(t *testing.T, app *App) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.ServeHTTP2(ln) }()
	t.Cleanup(func() {
		if err := app.Shutdown(); err != nil {
			t.Errorf("shutdown: %v", err)
		}
		if err := <-done; err != nil {
			t.Errorf("ServeHTTP2: %v", err)
		}
	})
	return "http://" + ln.Addr().String()
}

func TestHTTP2_Cleartext_RoutesMiddlewareAndErrors(t *testing.T) {
	base := serveH2C(t, bridgeApp())
	cl := h2cClient()

	resp, err := cl.Get(base + "/users/42?q=x%20y")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.ProtoMajor != 2 || resp.StatusCode != 200 {
		t.Fatalf("proto=%s status=%d", resp.Proto, resp.StatusCode)
	}
	for _, want := range []string{`"id":"42"`, `"q":"x y"`, `"proto":"http"`, `"ip":"127.0.0.1"`, `"ua":"Go-http-client/2.0"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("body %s lacks %s", body, want)
		}
	}
	if resp.Header.Get("X-Mw") != "ran" || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Errorf("headers = %v", resp.Header)
	}

	resp, err = cl.Post(base+"/echo", "application/json", strings.NewReader(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 201 || string(body) != `{"a":1}` {
		t.Fatalf("echo = %d %s", resp.StatusCode, body)
	}

	for path, want := range map[string]int{"/missing-entity": 404, "/nope": 404} {
		resp, err := cl.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("%s = %d, want %d", path, resp.StatusCode, want)
		}
	}

	// Body limit (Config.MaxRequestBodySize) holds on the bridge too.
	resp, err = cl.Post(base+"/echo", "application/octet-stream", strings.NewReader(strings.Repeat("a", 4<<10)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 413 {
		t.Errorf("oversized body = %d, want 413", resp.StatusCode)
	}

	// HTTP/1.1 clients are served by the same listener.
	resp, err = http.Get(base + "/users/7")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.ProtoMajor != 1 || resp.StatusCode != 200 {
		t.Errorf("http/1.1: proto=%s status=%d", resp.Proto, resp.StatusCode)
	}
}

// Over TLS the protocol is negotiated by ALPN and the handler sees https.
func TestHTTP2_TLS(t *testing.T) {
	srv := httptest.NewUnstartedServer(bridgeApp().NetHTTPHandler())
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/users/1")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.ProtoMajor != 2 || !strings.Contains(string(body), `"proto":"https"`) {
		t.Fatalf("proto=%s body=%s", resp.Proto, body)
	}
}

// StreamBody reaches the client chunk by chunk — over the native server and
// over the HTTP/2 bridge — instead of when the whole body is ready.
func TestStreamBody_FlushesWhileProducing(t *testing.T) {
	newApp := func(release chan struct{}) *App {
		app := New()
		app.Get("/stream", func(c *domain.Ctx) error {
			fc := c.AdapterData.(*FastCtx)
			fc.SetHeader("Content-Type", "text/plain")
			return fc.StreamBody(func(w *bufio.Writer) error {
				fmt.Fprintln(w, "first")
				if err := w.Flush(); err != nil {
					return err
				}
				<-release // the rest is produced only after the client saw "first"
				fmt.Fprintln(w, "second")
				return nil
			})
		})
		return app
	}
	check := func(t *testing.T, cl *http.Client, url string, release chan struct{}) {
		t.Helper()
		resp, err := cl.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		r := bufio.NewReader(resp.Body)
		line, err := r.ReadString('\n')
		if err != nil || line != "first\n" {
			t.Fatalf("first chunk = %q, %v", line, err)
		}
		close(release)
		rest, _ := io.ReadAll(r)
		if string(rest) != "second\n" || resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/plain" {
			t.Fatalf("rest = %q status=%d headers=%v", rest, resp.StatusCode, resp.Header)
		}
	}

	t.Run("native", func(t *testing.T) {
		release := make(chan struct{})
		app := newApp(release)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go func() { _ = app.Serve(ln) }()
		defer func() { _ = app.Shutdown() }()
		check(t, &http.Client{Timeout: 5 * time.Second}, "http://"+ln.Addr().String()+"/stream", release)
	})
	t.Run("http2", func(t *testing.T) {
		release := make(chan struct{})
		base := serveH2C(t, newApp(release))
		check(t, h2cClient(), base+"/stream", release)
	})
}
