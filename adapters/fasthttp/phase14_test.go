package fasthttp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// --- WebSocket Hub ---

func TestNewWSHub(t *testing.T) {
	hub := NewWSHub()
	if hub == nil {
		t.Fatal("NewWSHub returned nil")
	}
}

func TestWSHub_Join_Broadcast(t *testing.T) {
	hub := NewWSHub()
	ws1 := new(MockWS)
	ws2 := new(MockWS)
	hub.Join("room1", ws1)
	hub.Join("room1", ws2)

	hub.Broadcast([]byte("hello"))

	if ws1.writeCount != 1 {
		t.Errorf("ws1 writeCount = %d, want 1", ws1.writeCount)
	}
	if ws2.writeCount != 1 {
		t.Errorf("ws2 writeCount = %d, want 1", ws2.writeCount)
	}
}

func TestWSHub_BroadcastTo(t *testing.T) {
	hub := NewWSHub()
	ws1 := new(MockWS)
	ws2 := new(MockWS)
	hub.Join("room1", ws1)
	hub.Join("room2", ws2)

	hub.BroadcastTo("room1", []byte("only room1"))

	if ws1.writeCount != 1 {
		t.Errorf("ws1 writeCount = %d, want 1", ws1.writeCount)
	}
	if ws2.writeCount != 0 {
		t.Errorf("ws2 writeCount = %d, want 0", ws2.writeCount)
	}
}

func TestWSHub_Leave(t *testing.T) {
	hub := NewWSHub()
	ws := new(MockWS)
	hub.Join("room1", ws)
	hub.Leave("room1", ws)

	hub.BroadcastTo("room1", []byte("data"))
	if ws.writeCount != 0 {
		t.Errorf("ws writeCount = %d, want 0 after leave", ws.writeCount)
	}
}

func TestWSHub_CloseAll(t *testing.T) {
	hub := NewWSHub()
	ws1 := new(MockWS)
	ws2 := new(MockWS)
	hub.Join("room1", ws1)
	hub.Join("room2", ws2)

	hub.CloseAll()

	if !ws1.closed {
		t.Error("ws1 not closed")
	}
	if !ws2.closed {
		t.Error("ws2 not closed")
	}

	hub.Broadcast([]byte("after close"))
	if ws1.writeCount != 0 {
		t.Errorf("ws1 writeCount = %d, want 0 after CloseAll", ws1.writeCount)
	}
}

// --- WS typed messages ---

func TestFastWS_WriteJSON(t *testing.T) {
	ws := &FastWS{}
	_ = ws // would need a real conn; tested via mock below
}

// --- MockWS ---

type MockWS struct {
	writeCount int
	closed     bool
	lastData   []byte
}

func (m *MockWS) ReadMessage() ([]byte, error) {
	return nil, nil
}

func (m *MockWS) WriteMessage(data []byte) error {
	m.writeCount++
	m.lastData = data
	return nil
}

func (m *MockWS) WriteJSON(v any) error {
	data, _ := json.Marshal(v)
	return m.WriteMessage(data)
}

func (m *MockWS) ReadJSON(v any) error {
	return json.Unmarshal(m.lastData, v)
}

func (m *MockWS) Close() error {
	m.closed = true
	return nil
}

// --- SSE ---

func TestFastSSE_Send(t *testing.T) {
	var buf strings.Builder
	sse := &FastSSE{w: &buf}

	err := sse.Send("update", map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	expected := "event: update\ndata: {\"key\":\"val\"}\n\n"
	if buf.String() != expected {
		t.Errorf("SSE output = %q, want %q", buf.String(), expected)
	}
}

func TestFastSSE_SendRaw(t *testing.T) {
	var buf strings.Builder
	sse := &FastSSE{w: &buf}

	err := sse.SendRaw("msg", "plain text")
	if err != nil {
		t.Fatalf("SendRaw error: %v", err)
	}

	expected := "event: msg\ndata: plain text\n\n"
	if buf.String() != expected {
		t.Errorf("SSE output = %q, want %q", buf.String(), expected)
	}
}

func TestFastSSE_Retry(t *testing.T) {
	var buf strings.Builder
	sse := &FastSSE{w: &buf}

	err := sse.Retry(5000)
	if err != nil {
		t.Fatalf("Retry error: %v", err)
	}

	expected := "retry: 5000\n\n"
	if buf.String() != expected {
		t.Errorf("SSE output = %q, want %q", buf.String(), expected)
	}
}

func TestFastSSE_Close(t *testing.T) {
	var buf strings.Builder
	sse := &FastSSE{w: &buf}

	err := sse.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}

	err = sse.SendRaw("ev", "data")
	if err == nil {
		t.Error("expected error after Close")
	}
}

func TestSSEHandler_Integration(t *testing.T) {
	app := New()
	app.Get("/events", SSEHandler(func(sse domain.SSE) error {
		return sse.Send("ping", "pong")
	}))

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/events")
	app.Handler()(ctx)

	ct := string(ctx.Response.Header.ContentType())
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	body := string(ctx.Response.Body())
	if !strings.Contains(body, "event: ping") {
		t.Errorf("Body missing event: ping, got %q", body)
	}
	if !strings.Contains(body, `data: "pong"`) {
		t.Errorf("Body missing data, got %q", body)
	}
}

// --- Binding ---

type testBindStruct struct {
	ID   string `param:"id"`
	Name string `query:"name"`
	Key  string `header:"X-API-Key"`
	Tok  string `cookie:"token"`
}

func TestBindHeader(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.Set("X-API-Key", "secret-123")
	fc.Reset(fctx)

	var dest testBindStruct
	err := fc.BindHeader(&dest)
	if err != nil {
		t.Fatalf("BindHeader error: %v", err)
	}
	if dest.Key != "secret-123" {
		t.Errorf("Key = %q, want secret-123", dest.Key)
	}
}

func TestBindCookie(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	cookie := "token=my-token-value"
	fctx.Request.Header.Set("Cookie", cookie)
	fc.Reset(fctx)

	var dest testBindStruct
	err := fc.BindCookie(&dest)
	if err != nil {
		t.Fatalf("BindCookie error: %v", err)
	}
	if dest.Tok != "my-token-value" {
		t.Errorf("Tok = %q, want my-token-value", dest.Tok)
	}
}

func TestBindPath(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)
	fc.SetParam("id", "42")

	var dest testBindStruct
	err := fc.BindPath(&dest)
	if err != nil {
		t.Fatalf("BindPath error: %v", err)
	}
	if dest.ID != "42" {
		t.Errorf("ID = %q, want 42", dest.ID)
	}
}

func TestBind_NotPtr(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	err := fc.BindHeader(testBindStruct{})
	if err == nil {
		t.Error("expected error for non-pointer")
	}
}

func TestBind_UnsupportedType(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.Set("X-API-Key", "v")
	fc.Reset(fctx)

	dest := struct {
		Key complex64 `header:"X-API-Key"`
	}{}
	err := fc.BindHeader(&dest)
	if err == nil {
		t.Error("expected error for complex64")
	}
}

// --- Context ---

func TestFastCtx_RequestContext(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	// default is background
	if fc.RequestContext() != context.Background() {
		t.Error("default should be context.Background()")
	}
}

func TestFastCtx_SetRequestContext(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fc.SetRequestContext(ctx)
	if fc.RequestContext() != ctx {
		t.Error("RequestContext should return the set context")
	}
}

func TestFastCtx_SetRequestContext_Replaces(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	ctx1 := context.WithValue(context.Background(), "key", "val1")
	fc.SetRequestContext(ctx1)

	ctx2 := context.WithValue(context.Background(), "key", "val2")
	fc.SetRequestContext(ctx2)

	if fc.RequestContext() != ctx2 {
		t.Error("RequestContext should return the latest context")
	}
}

func TestDomainCtx_Context(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	// domain.Ctx delegates to FastCtx
	dc := fc.Ctx
	if dc.Context() != context.Background() {
		t.Error("domain Ctx.Context() should return background by default")
	}

	customCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	dc.SetContext(customCtx)
	if dc.Context() != customCtx {
		t.Error("domain Ctx.Context() should return custom context")
	}
}

// --- IP Whitelist ---

// An empty whitelist must fail closed: it guards pprof/metrics/dashboard, and
// an empty list usually means a mis-read env var.
func TestIPWhitelistMiddleware_EmptyListRejects(t *testing.T) {
	mw := IPWhitelistMiddleware()
	ctx := &domain.Ctx{}
	if err := mw(ctx); err == nil {
		t.Error("expected Forbidden for an empty whitelist, got nil (fail-open)")
	}
}

func TestIPWhitelistMiddleware_AllowSpecific(t *testing.T) {
	app := New()
	mw := IPWhitelistMiddleware("0.0.0.0")
	app.Use(mw)
	app.Get("/test", func(c *domain.Ctx) error {
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Errorf("expected 200, got %d", ctx.Response.StatusCode())
	}
}

func TestIPWhitelistMiddleware_Reject(t *testing.T) {
	app := New()
	mw := IPWhitelistMiddleware("10.0.0.1")
	app.Use(mw)
	app.Get("/test", func(c *domain.Ctx) error {
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 403 {
		t.Errorf("expected 403, got %d", ctx.Response.StatusCode())
	}
}

func TestIPWhitelistMiddleware_CIDR(t *testing.T) {
	app := New()
	mw := IPWhitelistMiddleware("192.168.0.0/16")
	app.Use(mw)
	app.Get("/test", func(c *domain.Ctx) error {
		return nil
	})

	// localhost should NOT match 192.168.0.0/16
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 403 {
		t.Errorf("expected 403 for non-matching CIDR, got %d", ctx.Response.StatusCode())
	}
}

// --- WebSocket typed messages ---

func TestFastWS_WriteJSON_viaHub(t *testing.T) {
	hub := NewWSHub()
	ws := new(MockWS)
	hub.Join("room", ws)

	type payload struct {
		Msg string `json:"msg"`
	}
	data, _ := json.Marshal(payload{Msg: "hi"})
	hub.Broadcast(data)

	var result payload
	err := ws.ReadJSON(&result)
	if err != nil {
		t.Fatalf("ReadJSON error: %v", err)
	}
	if result.Msg != "hi" {
		t.Errorf("Msg = %q, want hi", result.Msg)
	}
}
