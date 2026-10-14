package fasthttp

import (
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// WSConfig configures WebSocket behavior.
type WSConfig struct {
	// EnableCompression is accepted for compatibility but is a no-op:
	// permessage-deflate is not implemented, so it is never negotiated.
	EnableCompression bool
	// MaxMessageSize caps one incoming frame payload in bytes.
	// 0 = DefaultWSMaxMessageSize (1 MiB). Larger frames close the
	// connection with status 1009.
	MaxMessageSize int64
	// AllowedOrigins lists the browser Origins allowed to open the socket
	// (e.g. "https://app.example.com"; "*" allows any). Empty = same-origin
	// only: the Origin host must equal the request Host. Requests without an
	// Origin header (non-browser clients) are always accepted.
	AllowedOrigins []string
	// CheckOrigin overrides the origin policy entirely when set.
	CheckOrigin func(ctx *fasthttp.RequestCtx) bool
	// PingInterval is the ping send interval. 0 = disabled.
	PingInterval time.Duration
	// PongWait is how long to wait for pong before closing. 0 = 30s default.
	PongWait time.Duration
	// WriteWait is how long to wait for a write before closing. 0 = 10s default.
	WriteWait time.Duration
}

// FastWS implements domain.WebSocket with compression and ping/pong support.
type FastWS struct {
	conn   *wsConn
	config WSConfig
	mu     sync.Mutex
	closed bool
	done   chan struct{}
}

// ReadMessage reads a WebSocket message.
func (w *FastWS) ReadMessage() ([]byte, error) {
	_, data, err := w.conn.ReadMessage()
	return data, err
}

// WriteMessage sends a WebSocket message.
func (w *FastWS) WriteMessage(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return ErrCloseSent
	}
	return w.conn.WriteMessage(wsOpText, data)
}

// WriteJSON serializes v as JSON and sends it as a text message.
func (w *FastWS) WriteJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.WriteMessage(data)
}

// ReadJSON reads a text message and unmarshals JSON into v.
func (w *FastWS) ReadJSON(v any) error {
	_, data, err := w.conn.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// Close closes the WebSocket connection.
func (w *FastWS) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	close(w.done)
	return w.conn.Close()
}

// startPing starts the ping/pong loop.
func (w *FastWS) startPing() {
	if w.config.PingInterval <= 0 {
		return
	}

	pongWait := w.config.PongWait
	if pongWait <= 0 {
		pongWait = 30 * time.Second
	}
	writeWait := w.config.WriteWait
	if writeWait <= 0 {
		writeWait = 10 * time.Second
	}

	w.conn.SetReadDeadline(time.Now().Add(pongWait))
	w.conn.SetPongHandler(func(string) error {
		w.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		ticker := time.NewTicker(w.config.PingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.mu.Lock()
				if w.closed {
					w.mu.Unlock()
					return
				}
				w.conn.SetWriteDeadline(time.Now().Add(writeWait))
				err := w.conn.WriteMessage(wsOpPing, nil)
				w.mu.Unlock()
				if err != nil {
					return
				}
			case <-w.done:
				return
			}
		}
	}()
}

// FastWSHub manages multiple WebSocket connections with room support.
type FastWSHub struct {
	mu    sync.RWMutex
	conns map[domain.WebSocket]bool
	rooms map[string]map[domain.WebSocket]bool
}

// NewWSHub creates a new FastWSHub.
func NewWSHub() *FastWSHub {
	return &FastWSHub{
		conns: make(map[domain.WebSocket]bool),
		rooms: make(map[string]map[domain.WebSocket]bool),
	}
}

// Broadcast sends a message to all managed connections.
func (h *FastWSHub) Broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ws := range h.conns {
		ws.WriteMessage(data)
	}
}

// BroadcastTo sends a message to all connections in a room.
func (h *FastWSHub) BroadcastTo(room string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if conns, ok := h.rooms[room]; ok {
		for ws := range conns {
			ws.WriteMessage(data)
		}
	}
}

// Join adds a connection to a room.
func (h *FastWSHub) Join(room string, ws domain.WebSocket) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[ws] = true
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[domain.WebSocket]bool)
	}
	h.rooms[room][ws] = true
}

// Leave removes a connection from a room.
func (h *FastWSHub) Leave(room string, ws domain.WebSocket) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.rooms[room]; ok {
		delete(conns, ws)
		if len(conns) == 0 {
			delete(h.rooms, room)
		}
	}
	delete(h.conns, ws)
}

// CloseAll closes all managed connections.
func (h *FastWSHub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ws := range h.conns {
		ws.Close()
	}
	h.conns = make(map[domain.WebSocket]bool)
	h.rooms = make(map[string]map[domain.WebSocket]bool)
}

// originAllowed implements the WebSocket origin policy (anti Cross-Site
// WebSocket Hijacking). Browsers always send Origin on WebSocket handshakes
// and attach the user's cookies, so a wildcard policy lets any website talk
// to the socket as the logged-in user.
func (cfg WSConfig) originAllowed(ctx *fasthttp.RequestCtx) bool {
	if cfg.CheckOrigin != nil {
		return cfg.CheckOrigin(ctx)
	}
	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		return true
	}
	for _, o := range cfg.AllowedOrigins {
		if o == "*" || strings.EqualFold(o, origin) {
			return true
		}
	}
	if len(cfg.AllowedOrigins) > 0 {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, string(ctx.Host()))
}

// WebSocketHandler creates a WebSocket upgrade handler.
func WebSocketHandler(handler domain.WSHandler) domain.Handler {
	return WebSocketHandlerWithConfig(handler, WSConfig{})
}

// WebSocketHandlerWithConfig creates a WebSocket upgrade handler with config.
func WebSocketHandlerWithConfig(handler domain.WSHandler, cfg WSConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx).FasthttpCtx()

		if !cfg.originAllowed(fctx) {
			fctx.SetStatusCode(fasthttp.StatusForbidden)
			return nil
		}

		return upgradeWebSocket(fctx, cfg.MaxMessageSize, func(conn *wsConn) {
			ws := &FastWS{
				conn:   conn,
				config: cfg,
				done:   make(chan struct{}),
			}
			ws.startPing()
			handler(ws)
		})
	}
}
