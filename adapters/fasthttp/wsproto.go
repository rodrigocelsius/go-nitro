package fasthttp

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// WebSocket opcodes.
const (
	wsOpText   = 0x1
	wsOpBinary = 0x2
	wsOpClose  = 0x8
	wsOpPing   = 0x9
	wsOpPong   = 0xA
)

// WebSocket magic GUID for handshake.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

var (
	// ErrCloseSent is returned when writing to a closed WebSocket.
	ErrCloseSent = errors.New("websocket: close sent")
	// ErrCloseReceived is returned when reading after close frame.
	ErrCloseReceived = errors.New("websocket: close received")
)

// wsConn is a minimal WebSocket connection over a net.Conn.
type wsConn struct {
	conn net.Conn
	br   *bufio.Reader
	mu   sync.Mutex

	pongHandler func(string) error
	closed      bool

	// maxMessage caps a single frame payload (0 = DefaultWSMaxMessageSize).
	maxMessage int64
}

// DefaultWSMaxMessageSize is the default cap for one WebSocket frame payload.
const DefaultWSMaxMessageSize int64 = 1 << 20 // 1 MiB

// Close status codes used when the server rejects a frame.
const (
	wsCloseProtocolError = 1002
	wsCloseTooBig        = 1009
)

var (
	// ErrWSMessageTooBig is returned when a frame exceeds the configured limit.
	ErrWSMessageTooBig = errors.New("websocket: message too big")
	// ErrWSProtocol is returned on RFC 6455 violations (e.g. unmasked client frame).
	ErrWSProtocol = errors.New("websocket: protocol error")
)

func (ws *wsConn) limit() int64 {
	if ws.maxMessage > 0 {
		return ws.maxMessage
	}
	return DefaultWSMaxMessageSize
}

// failClose sends a close frame with the given status code (best effort).
func (ws *wsConn) failClose(code int) {
	ws.writeFrame(wsOpClose, []byte{byte(code >> 8), byte(code)}) //nolint:errcheck
}

// ReadMessage reads a complete WebSocket message (text or binary).
// Returns message type, payload, and any error.
func (ws *wsConn) ReadMessage() (int, []byte, error) {
	for {
		opcode, payload, err := ws.readFrame()
		if err != nil {
			return 0, nil, err
		}

		switch opcode {
		case wsOpText, wsOpBinary:
			return opcode, payload, nil
		case wsOpClose:
			ws.writeCloseFrame()
			return 0, nil, io.EOF
		case wsOpPing:
			ws.writeFrame(wsOpPong, payload)
		case wsOpPong:
			if ws.pongHandler != nil {
				ws.pongHandler(string(payload))
			}
		}
	}
}

// WriteMessage writes a WebSocket message with the given opcode.
func (ws *wsConn) WriteMessage(opcode int, data []byte) error {
	return ws.writeFrame(opcode, data)
}

// SetReadDeadline sets the read deadline on the underlying connection.
func (ws *wsConn) SetReadDeadline(t time.Time) {
	ws.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline on the underlying connection.
func (ws *wsConn) SetWriteDeadline(t time.Time) {
	ws.conn.SetWriteDeadline(t)
}

// SetPongHandler sets the handler for pong messages.
func (ws *wsConn) SetPongHandler(h func(appData string) error) {
	ws.pongHandler = h
}

// Close sends a close frame and closes the connection.
func (ws *wsConn) Close() error {
	ws.mu.Lock()
	closed := ws.closed
	ws.closed = true
	ws.mu.Unlock()
	if closed {
		return nil
	}
	ws.writeCloseFrame()
	return ws.conn.Close()
}

func (ws *wsConn) writeCloseFrame() {
	ws.writeFrame(wsOpClose, nil) //nolint:errcheck
}

func (ws *wsConn) readFrame() (opcode int, payload []byte, err error) {
	// Read first 2 bytes
	header := make([]byte, 2)
	if _, err = io.ReadFull(ws.br, header); err != nil {
		return 0, nil, err
	}

	opcode = int(header[0] & 0x0F)
	masked := (header[1] & 0x80) != 0
	length := int64(header[1] & 0x7F)

	// Extended payload length
	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err = io.ReadFull(ws.br, ext); err != nil {
			return 0, nil, err
		}
		length = int64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err = io.ReadFull(ws.br, ext); err != nil {
			return 0, nil, err
		}
		u := binary.BigEndian.Uint64(ext)
		if u > uint64(ws.limit()) { // also rejects values with the high bit set
			ws.failClose(wsCloseTooBig)
			return 0, nil, ErrWSMessageTooBig
		}
		length = int64(u)
	}

	// RFC 6455 §5.1: clients MUST mask; control frames are <=125 bytes and unfragmented.
	if !masked {
		ws.failClose(wsCloseProtocolError)
		return 0, nil, ErrWSProtocol
	}
	if opcode >= 0x8 && (length > 125 || header[0]&0x80 == 0) {
		ws.failClose(wsCloseProtocolError)
		return 0, nil, ErrWSProtocol
	}
	if length > ws.limit() {
		ws.failClose(wsCloseTooBig)
		return 0, nil, ErrWSMessageTooBig
	}

	// Read mask key
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(ws.br, mask[:]); err != nil {
			return 0, nil, err
		}
	}

	// Read payload
	if length > 0 {
		payload = make([]byte, length)
		if _, err = io.ReadFull(ws.br, payload); err != nil {
			return 0, nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
	}

	return opcode, payload, nil
}

func (ws *wsConn) writeFrame(opcode int, payload []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.closed {
		return ErrCloseSent
	}

	// FIN + opcode
	frame := []byte{byte(0x80 | opcode)}

	// Payload length
	length := len(payload)
	switch {
	case length <= 125:
		frame = append(frame, byte(length))
	case length <= 65535:
		frame = append(frame, 126)
		frame = binary.BigEndian.AppendUint16(frame, uint16(length))
	default:
		frame = append(frame, 127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(length))
	}

	frame = append(frame, payload...)

	_, err := ws.conn.Write(frame)
	return err
}

// upgradeWebSocket performs the WebSocket handshake and hijacks the connection,
// calling the callback with the WebSocket connection after hijack succeeds.
func upgradeWebSocket(ctx *fasthttp.RequestCtx, maxMessage int64, cb func(*wsConn)) error {
	// Validate WebSocket handshake
	key := string(ctx.Request.Header.Peek("Sec-WebSocket-Key"))
	if key == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return errors.New("websocket: missing Sec-WebSocket-Key")
	}

	upgrade := string(ctx.Request.Header.Peek("Upgrade"))
	if !strings.EqualFold(strings.TrimSpace(upgrade), "websocket") {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return errors.New("websocket: missing Upgrade: websocket")
	}

	// Compute accept key
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte(wsGUID))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Set response headers for the handshake
	ctx.SetStatusCode(fasthttp.StatusSwitchingProtocols)
	ctx.Response.Header.Set("Upgrade", "websocket")
	ctx.Response.Header.Set("Connection", "Upgrade")
	ctx.Response.Header.Set("Sec-WebSocket-Accept", accept)

	// permessage-deflate is intentionally NOT negotiated: frames are never
	// inflated, so advertising it would corrupt messages from clients that
	// accept the extension (and invite decompression bombs).

	// Hijack the connection - runs after handler returns
	ctx.Hijack(func(c net.Conn) {
		// The hijack callback runs on its own goroutine: a panic in the
		// user handler would otherwise crash the whole process.
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "nitro: websocket handler panic: %v\n", r)
			}
			c.Close()
		}()
		ws := &wsConn{
			conn:       c,
			br:         bufio.NewReaderSize(c, 4096),
			maxMessage: maxMessage,
		}
		cb(ws)
	})

	return nil
}
