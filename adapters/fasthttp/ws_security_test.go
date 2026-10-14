package fasthttp

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"testing"

	"github.com/valyala/fasthttp"
)

func maskedFrame(opcode byte, payload []byte) []byte {
	var b bytes.Buffer
	b.WriteByte(0x80 | opcode)
	n := len(payload)
	switch {
	case n < 126:
		b.WriteByte(0x80 | byte(n))
	case n < 65536:
		b.Write([]byte{0x80 | 126, byte(n >> 8), byte(n)})
	}
	b.Write([]byte{1, 2, 3, 4})
	for i, c := range payload {
		b.WriteByte(c ^ []byte{1, 2, 3, 4}[i%4])
	}
	return b.Bytes()
}

func newTestWS(data []byte, max int64) *wsConn {
	return &wsConn{br: bufio.NewReader(bytes.NewReader(data)), conn: nopConn{}, maxMessage: max}
}

// Regression: a 14-byte header declaring 8 GiB used to allocate it and kill the process.
func TestWS_RejectsHugeFrame(t *testing.T) {
	frame := []byte{0x81, 0xFF, 0, 0, 0, 2, 0, 0, 0, 0, 1, 2, 3, 4}
	_, _, err := newTestWS(frame, 0).readFrame()
	if !errors.Is(err, ErrWSMessageTooBig) {
		t.Fatalf("want ErrWSMessageTooBig, got %v", err)
	}
	// high bit set (would be negative as int64)
	frame = []byte{0x81, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 1, 2, 3, 4}
	if _, _, err = newTestWS(frame, 0).readFrame(); !errors.Is(err, ErrWSMessageTooBig) {
		t.Fatalf("negative-length frame: got %v", err)
	}
}

func TestWS_MaxMessageSizeConfigurable(t *testing.T) {
	p := bytes.Repeat([]byte("a"), 200)
	if _, _, err := newTestWS(maskedFrame(1, p), 100).readFrame(); !errors.Is(err, ErrWSMessageTooBig) {
		t.Fatalf("200 > 100 should fail, got %v", err)
	}
	_, got, err := newTestWS(maskedFrame(1, p), 1000).readFrame()
	if err != nil || !bytes.Equal(got, p) {
		t.Fatalf("200 <= 1000 should pass: %v", err)
	}
}

func TestWS_RejectsUnmaskedClientFrame(t *testing.T) {
	if _, _, err := newTestWS([]byte{0x81, 0x01, 'a'}, 0).readFrame(); !errors.Is(err, ErrWSProtocol) {
		t.Fatalf("unmasked frame: got %v", err)
	}
}

func TestWS_OriginPolicy(t *testing.T) {
	mk := func(origin, host string) *fasthttp.RequestCtx {
		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetHost(host)
		if origin != "" {
			c.Request.Header.Set("Origin", origin)
		}
		return c
	}
	cases := []struct {
		name string
		cfg  WSConfig
		ctx  *fasthttp.RequestCtx
		want bool
	}{
		{"no origin (non-browser)", WSConfig{}, mk("", "api.com"), true},
		{"same origin", WSConfig{}, mk("https://api.com", "api.com"), true},
		{"cross origin denied by default", WSConfig{}, mk("https://evil.com", "api.com"), false},
		{"allow-list hit", WSConfig{AllowedOrigins: []string{"https://app.com"}}, mk("https://app.com", "api.com"), true},
		{"allow-list miss", WSConfig{AllowedOrigins: []string{"https://app.com"}}, mk("https://evil.com", "api.com"), false},
		{"explicit wildcard", WSConfig{AllowedOrigins: []string{"*"}}, mk("https://x.com", "api.com"), true},
		{"custom func", WSConfig{CheckOrigin: func(*fasthttp.RequestCtx) bool { return false }}, mk("", "api.com"), false},
	}
	for _, tc := range cases {
		if got := tc.cfg.originAllowed(tc.ctx); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

// nopConn is a net.Conn that discards writes (for failClose in unit tests).
type nopConn struct{ net.Conn }

func (nopConn) Write(b []byte) (int, error) { return len(b), nil }
func (nopConn) Close() error                { return nil }
