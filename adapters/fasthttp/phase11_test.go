package fasthttp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// generateTestCert creates a temporary self-signed TLS cert/key pair for testing.
func generateTestCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	dir := t.TempDir()

	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	// Generate RSA key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	// Create cert template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	// Self-sign
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	// Write cert PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		t.Fatal(err)
	}

	// Write key PEM. 0600 (owner-only): even in a test fixture, a
	// private key readable by any other local user (0644) is bad
	// practice to model/copy from (see vuln.md V-38).
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	return
}

func TestListenTLS_Serve(t *testing.T) {
	app := New()
	ok := false
	app.Get("/hello", func(c *domain.Ctx) error {
		ok = true
		return nil
	})

	certFile, keyFile := generateTestCert(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	go func() {
		app.server = &fasthttp.Server{Handler: app.Handler()}
		if err := app.server.ServeTLS(ln, certFile, keyFile); err != nil {
			t.Logf("serve error: %v", err)
		}
	}()

	client := &fasthttp.Client{
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}
	statusCode, body, err := client.Get(nil, "https://"+addr+"/hello")
	if err != nil {
		t.Fatal(err)
	}
	if statusCode != fasthttp.StatusOK {
		t.Errorf("status = %d, want 200", statusCode)
	}
	if !ok {
		t.Error("handler was not called")
	}
	_ = body
}

func TestListenTLS_NotFound(t *testing.T) {
	app := New()
	app.Get("/hello", func(c *domain.Ctx) error { return nil })

	certFile, keyFile := generateTestCert(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	go func() {
		app.server = &fasthttp.Server{Handler: app.Handler()}
		_ = app.server.ServeTLS(ln, certFile, keyFile)
	}()

	client := &fasthttp.Client{
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}
	statusCode, _, err := client.Get(nil, "https://"+addr+"/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if statusCode != fasthttp.StatusNotFound {
		t.Errorf("status = %d, want 404", statusCode)
	}
}

func TestListenTLS_Handler(t *testing.T) {
	app := New()
	app.Get("/secure", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.SendString("secure content")
	})

	certFile, keyFile := generateTestCert(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	go func() {
		app.server = &fasthttp.Server{Handler: app.Handler()}
		_ = app.server.ServeTLS(ln, certFile, keyFile)
	}()

	client := &fasthttp.Client{
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}
	statusCode, body, err := client.Get(nil, "https://"+addr+"/secure")
	if err != nil {
		t.Fatal(err)
	}
	if statusCode != fasthttp.StatusOK {
		t.Errorf("status = %d, want 200", statusCode)
	}
	if string(body) != "secure content" {
		t.Errorf("body = %q, want 'secure content'", body)
	}
}

func TestListenTLS_EnableViaConfig(t *testing.T) {
	tlsConfig := &tls.Config{}

	app := New(domain.Config{
		TLSConfig: tlsConfig,
	})
	if app.tlsConfig == nil {
		t.Fatal("tlsConfig should be set via Config")
	}
}

func TestEnableTLS_Chaining(t *testing.T) {
	app := New()
	cfg := &tls.Config{}
	got := app.EnableTLS(cfg)
	if got != app {
		t.Error("EnableTLS should return *App for chaining")
	}
	if app.tlsConfig != cfg {
		t.Error("tlsConfig not set")
	}
}

func TestListenTLS_ViaListenMethod(t *testing.T) {
	app := New(domain.Config{
		TLSConfig: &tls.Config{},
	})
	if app.tlsConfig == nil {
		t.Fatal("expected tlsConfig from Config")
	}
}

func TestListenTLS_NoCert(t *testing.T) {
	app := New()
	err := app.ListenTLS("127.0.0.1:0", "/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Error("expected error with nonexistent cert files")
	}
}
