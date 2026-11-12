package fasthttp

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/valyala/fasthttp"
)

// NetHTTPHandler exposes the App as a standard net/http handler: every
// route, middleware and error handler runs exactly as under the native
// server. It is what ListenHTTP2 serves, and it also mounts a Nitro app
// inside an existing net/http server or behind any net/http-based runtime.
//
// Differences from the native server, all consequences of bridging:
//   - The request body is read into memory first (bounded by
//     Config.MaxRequestBodySize; larger bodies get 413).
//   - Connection hijacking (WebSocket upgrade) is not available: HTTP/2 has
//     no Upgrade, and the bridge answers 501 to a handler that hijacks.
//   - Each request is copied into fasthttp types, so it is slower than the
//     native HTTP/1.1 server. Streamed responses (SSE, StreamBody) are
//     flushed to the client as they are produced.
func (a *App) NetHTTPHandler() http.Handler {
	handler := a.Handler()
	maxBody := a.maxRequestBodySize
	if maxBody <= 0 {
		maxBody = DefaultMaxRequestBodySize
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var fctx fasthttp.RequestCtx
		fctx.Init2(newBridgeConn(r), bridgeLogger{}, true)

		req := &fctx.Request
		req.Header.SetMethod(r.Method)
		req.SetRequestURI(r.URL.RequestURI())
		req.Header.SetHost(r.Host)
		for name, values := range r.Header {
			for _, v := range values {
				req.Header.Add(name, v)
			}
		}
		if r.Body != nil && r.Body != http.NoBody {
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(maxBody)))
			if err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					writeBridgeError(w, http.StatusRequestEntityTooLarge, "payload too large")
					return
				}
				writeBridgeError(w, http.StatusBadRequest, "cannot read request body")
				return
			}
			req.SetBodyRaw(body)
			req.Header.SetContentLength(len(body))
		}

		handler(&fctx)

		if fctx.Hijacked() {
			writeBridgeError(w, http.StatusNotImplemented, "connection upgrade is not available over HTTP/2")
			return
		}
		resp := &fctx.Response
		out := w.Header()
		for name, value := range resp.Header.All() {
			switch string(name) {
			case "Connection", "Transfer-Encoding", "Keep-Alive":
				// Connection-level headers do not exist in HTTP/2 and are
				// managed by net/http in HTTP/1.1.
				continue
			case "Content-Length":
				if resp.IsBodyStream() {
					continue
				}
			}
			out.Add(string(name), string(value))
		}
		w.WriteHeader(resp.StatusCode())
		if r.Method == http.MethodHead {
			return
		}
		if !resp.IsBodyStream() {
			_, _ = w.Write(resp.Body())
			return
		}
		// Streamed body: forward every chunk as soon as it is produced.
		flusher, _ := w.(http.Flusher)
		stream := resp.BodyStream()
		buf := make([]byte, 32*1024)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					break // client went away
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			if err != nil {
				break
			}
		}
		_ = resp.CloseBodyStream()
	})
}

func writeBridgeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}

// ListenHTTP2 serves the App over HTTP/2 (and HTTP/1.1) with Go's net/http
// server, through NetHTTPHandler. fasthttp, which backs Listen, speaks only
// HTTP/1.1; use this when clients must reach the app over HTTP/2 directly —
// gRPC-Web gateways, browsers multiplexing many requests — without a proxy
// in front.
//
// With certFile/keyFile it serves TLS and negotiates h2 through ALPN
// (Config.TLSConfig is honored). With both empty it serves cleartext
// HTTP/2 (h2c, prior knowledge) next to HTTP/1.1, for use behind a
// TLS-terminating load balancer that speaks HTTP/2 to its backends.
//
// Config.Server timeouts apply; see NetHTTPHandler for what differs from
// the native server (buffered request bodies, no WebSocket).
func (a *App) ListenHTTP2(addr, certFile, keyFile string) error {
	if err := a.Err(); err != nil {
		return err
	}
	if err := a.fireOnListen(addr); err != nil {
		return err
	}
	srv := a.newHTTP2Server(addr, certFile != "" || keyFile != "")
	a.printStartupBanner(addr, certFile != "")
	var err error
	if certFile != "" || keyFile != "" {
		err = srv.ListenAndServeTLS(certFile, keyFile)
	} else {
		err = srv.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// ServeHTTP2 is ListenHTTP2 on an existing listener (cleartext: HTTP/1.1
// and h2c). Wrap the listener with tls.NewListener for TLS.
func (a *App) ServeHTTP2(ln net.Listener) error {
	if err := a.Err(); err != nil {
		return err
	}
	err := a.newHTTP2Server(ln.Addr().String(), false).Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// newHTTP2Server builds the net/http server and publishes it for Shutdown.
func (a *App) newHTTP2Server(addr string, useTLS bool) *http.Server {
	sc := a.serverCfg
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	if useTLS {
		protocols.SetHTTP2(true)
	} else {
		protocols.SetUnencryptedHTTP2(true)
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           a.NetHTTPHandler(),
		ReadTimeout:       orDuration(sc.ReadTimeout, DefaultReadTimeout),
		ReadHeaderTimeout: orDuration(sc.ReadTimeout, DefaultReadTimeout),
		WriteTimeout:      orDuration(sc.WriteTimeout, DefaultWriteTimeout),
		IdleTimeout:       orDuration(sc.IdleTimeout, DefaultIdleTimeout),
		MaxHeaderBytes:    orInt(sc.ReadBufferSize, DefaultReadBufferSize),
		Protocols:         protocols,
	}
	if useTLS && a.tlsConfig != nil {
		srv.TLSConfig = a.tlsConfig.Clone()
	}
	a.srvMu.Lock()
	a.stdServer = srv
	a.srvMu.Unlock()
	return srv
}

// shutdownStdServer stops the net/http server started by ListenHTTP2.
func (a *App) shutdownStdServer(ctx context.Context) error {
	a.srvMu.RLock()
	srv := a.stdServer
	a.srvMu.RUnlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// bridgeConn stands in for the connection of a bridged request: it gives
// fasthttp the peer and local addresses and, for a TLS request, the
// connection state (so IsTLS / Protocol() report https). It carries no data.
type bridgeConn struct {
	local, remote net.Addr
}

func newBridgeConn(r *http.Request) net.Conn {
	c := bridgeConn{local: zeroAddr, remote: zeroAddr}
	if ap, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		c.remote = net.TCPAddrFromAddrPort(ap)
	}
	if la, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && la != nil {
		c.local = la
	}
	if r.TLS != nil {
		return &bridgeTLSConn{bridgeConn: c, state: *r.TLS}
	}
	return &c
}

var zeroAddr net.Addr = &net.TCPAddr{IP: net.IPv4zero}

func (c *bridgeConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c *bridgeConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c *bridgeConn) Close() error                     { return nil }
func (c *bridgeConn) LocalAddr() net.Addr              { return c.local }
func (c *bridgeConn) RemoteAddr() net.Addr             { return c.remote }
func (c *bridgeConn) SetDeadline(time.Time) error      { return nil }
func (c *bridgeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *bridgeConn) SetWriteDeadline(time.Time) error { return nil }

type bridgeTLSConn struct {
	bridgeConn
	state tls.ConnectionState
}

func (c *bridgeTLSConn) Handshake() error                     { return nil }
func (c *bridgeTLSConn) ConnectionState() tls.ConnectionState { return c.state }

// bridgeLogger receives fasthttp's internal diagnostics for bridged requests.
type bridgeLogger struct{}

func (bridgeLogger) Printf(format string, args ...any) {
	slog.Debug("nitro: http bridge: "+format, args...)
}
