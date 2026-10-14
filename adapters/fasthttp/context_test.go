package fasthttp

import (
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestNewFastCtx(t *testing.T) {
	fc := NewFastCtx()
	if fc == nil {
		t.Fatal("NewFastCtx returned nil")
	}
}

func TestFastCtx_Reset(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	if fc.FasthttpCtx() != fctx {
		t.Error("FasthttpCtx mismatch after Reset")
	}
	if fc.Ctx.AdapterData != fc {
		t.Error("AdapterData should be self-reference")
	}
}

func TestFastCtx_Method(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("POST")
	fc.Reset(fctx)

	if got := fc.Method(); got != "POST" {
		t.Errorf("Method() = %q, want %q", got, "POST")
	}
}

func TestFastCtx_Path(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/users/123")
	fc.Reset(fctx)

	if got := fc.Path(); got != "/users/123" {
		t.Errorf("Path() = %q, want %q", got, "/users/123")
	}
}

func TestFastCtx_Body(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetBody([]byte("hello"))
	fc.Reset(fctx)

	if got := string(fc.Body()); got != "hello" {
		t.Errorf("Body() = %q, want %q", got, "hello")
	}
}

func TestFastCtx_Status(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	fc.SetStatus(201)
	if fctx.Response.StatusCode() != 201 {
		t.Errorf("StatusCode = %d, want 201", fctx.Response.StatusCode())
	}
}

func TestFastCtx_SendString(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	err := fc.SendString("test body")
	if err != nil {
		t.Fatalf("SendString error: %v", err)
	}
	if got := string(fctx.Response.Body()); got != "test body" {
		t.Errorf("Body = %q, want %q", got, "test body")
	}
}

func TestFastCtx_JSON(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	data := map[string]string{"key": "value"}
	err := fc.JSON(200, data)
	if err != nil {
		t.Fatalf("JSON error: %v", err)
	}

	ct := string(fctx.Response.Header.ContentType())
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	body := string(fctx.Response.Body())
	if body != `{"key":"value"}` {
		t.Errorf("Body = %q, want %q", body, `{"key":"value"}`)
	}
}

func TestFastCtx_Bind(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetBody([]byte(`{"name":"test"}`))
	fc.Reset(fctx)

	var result map[string]string
	err := fc.Bind(&result)
	if err != nil {
		t.Fatalf("Bind error: %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("Bind result = %v, want map[name:test]", result)
	}
}

func TestFastCtx_Fasthttp(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	if fc.Fasthttp() != fctx {
		t.Error("Fasthttp() should return the underlying RequestCtx")
	}
}

func helperRequestCtx() *fasthttp.RequestCtx {
	return &fasthttp.RequestCtx{}
}

func TestFastCtx_ParamIntegration(t *testing.T) {
	fc := NewFastCtx()
	fctx := helperRequestCtx()
	fc.Reset(fctx)

	fc.SetParam("id", "42")
	if got := fc.Param("id"); got != "42" {
		t.Errorf("Param(id) = %q, want %q", got, "42")
	}
}

func TestFastCtx_JSON_Chainable(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	err := fc.JSON(200, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("JSON error: %v", err)
	}
	body := string(fctx.Response.Body())
	if body != "[1,2,3]" {
		t.Errorf("Body = %q, want %q", body, "[1,2,3]")
	}
}

func TestFastCtx_Bind_InvalidJSON(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetBody([]byte(`{invalid`))
	fc.Reset(fctx)

	var result map[string]string
	err := fc.Bind(&result)
	if err == nil {
		t.Error("Bind should return error for invalid JSON")
	}
}

func TestFastCtx_JSON_InvalidType(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	// channel cannot be marshaled
	err := fc.JSON(200, make(chan int))
	if err == nil {
		t.Error("JSON should return error for unsupported type")
	}
}

func TestFastCtx_Reset_ClearsOldValues(t *testing.T) {
	fc := NewFastCtx()

	fctx1 := &fasthttp.RequestCtx{}
	fctx1.Request.SetRequestURI("/first")
	fc.Reset(fctx1)

	if fc.Path() != "/first" {
		t.Errorf("Path = %q, want /first", fc.Path())
	}

	fctx2 := &fasthttp.RequestCtx{}
	fctx2.Request.SetRequestURI("/second")
	fc.Reset(fctx2)

	if fc.Path() != "/second" {
		t.Errorf("Path = %q, want /second", fc.Path())
	}
}

func TestFastCtx_FullRequestCycle(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("PUT")
	fctx.Request.SetRequestURI("/items/99")
	fctx.Request.SetBody([]byte(`{"active":true}`))
	fc.Reset(fctx)

	if fc.Method() != "PUT" {
		t.Errorf("Method = %q, want PUT", fc.Method())
	}
	if fc.Path() != "/items/99" {
		t.Errorf("Path = %q, want /items/99", fc.Path())
	}
	if string(fc.Body()) != `{"active":true}` {
		t.Errorf("Body mismatch")
	}

	fc.SetStatus(200)
	fc.SendString("ok")
	if fctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", fctx.Response.StatusCode())
	}
	if string(fctx.Response.Body()) != "ok" {
		t.Errorf("Body = %q, want ok", fctx.Response.Body())
	}
}

func BenchmarkFastCtx_Param(b *testing.B) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)
	fc.SetParam("id", "123")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = fc.Param("id")
	}
}

func BenchmarkFastCtx_JSON(b *testing.B) {
	data := map[string]string{"name": "test", "value": "123"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		fc := NewFastCtx()
		fctx := &fasthttp.RequestCtx{}
		fc.Reset(fctx)
		_ = fc.JSON(200, data)
	}
}

// --- QueryIntDefault tests ---

func TestFastCtx_QueryIntDefault_Present(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test?page=5")
	fc.Reset(fctx)

	got := fc.QueryIntDefault("page", 1)
	if got != 5 {
		t.Errorf("QueryIntDefault(page, 1) = %d, want 5", got)
	}
}

func TestFastCtx_QueryIntDefault_Missing(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test")
	fc.Reset(fctx)

	got := fc.QueryIntDefault("page", 20)
	if got != 20 {
		t.Errorf("QueryIntDefault(page, 20) = %d, want 20 (default)", got)
	}
}

func TestFastCtx_QueryIntDefault_Invalid(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test?page=abc")
	fc.Reset(fctx)

	got := fc.QueryIntDefault("page", 10)
	if got != 10 {
		t.Errorf("QueryIntDefault(page, 10) = %d, want 10 (default for invalid)", got)
	}
}

// --- QueryFloat64 tests ---

func TestFastCtx_QueryFloat64_Present(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test?price=19.99")
	fc.Reset(fctx)

	got, err := fc.QueryFloat64("price")
	if err != nil {
		t.Fatalf("QueryFloat64 error: %v", err)
	}
	if got != 19.99 {
		t.Errorf("QueryFloat64(price) = %f, want 19.99", got)
	}
}

func TestFastCtx_QueryFloat64_Missing(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test")
	fc.Reset(fctx)

	got, err := fc.QueryFloat64("price")
	if err != nil {
		t.Fatalf("QueryFloat64 error: %v", err)
	}
	if got != 0.0 {
		t.Errorf("QueryFloat64(price) = %f, want 0.0 for missing", got)
	}
}

func TestFastCtx_QueryFloat64_Invalid(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test?price=abc")
	fc.Reset(fctx)

	_, err := fc.QueryFloat64("price")
	if err == nil {
		t.Error("QueryFloat64 should return error for invalid float")
	}
}

// --- QueryFloat64Default tests ---

func TestFastCtx_QueryFloat64Default_Present(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test?price=9.50")
	fc.Reset(fctx)

	got := fc.QueryFloat64Default("price", 1.0)
	if got != 9.50 {
		t.Errorf("QueryFloat64Default(price, 1.0) = %f, want 9.50", got)
	}
}

func TestFastCtx_QueryFloat64Default_Missing(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test")
	fc.Reset(fctx)

	got := fc.QueryFloat64Default("price", 1.0)
	if got != 1.0 {
		t.Errorf("QueryFloat64Default(price, 1.0) = %f, want 1.0 (default)", got)
	}
}

func TestFastCtx_QueryFloat64Default_Invalid(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test?price=notanumber")
	fc.Reset(fctx)

	got := fc.QueryFloat64Default("price", 2.5)
	if got != 2.5 {
		t.Errorf("QueryFloat64Default(price, 2.5) = %f, want 2.5 (default for invalid)", got)
	}
}

// --- Context methods: Set/Get (locals) ---

func TestFastCtx_SetGet(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	fc.Set("user_id", "42")
	got := fc.Get("user_id")
	if got != "42" {
		t.Errorf("Get(user_id) = %v, want 42", got)
	}
}

func TestFastCtx_Get_MissingKey(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	got := fc.Get("nonexistent")
	if got != nil {
		t.Errorf("Get(nonexistent) = %v, want nil", got)
	}
}

// --- Context methods: QueryBool ---

func TestFastCtx_QueryBool_True(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test?verbose=true")
	fc.Reset(fctx)

	if !fc.QueryBool("verbose") {
		t.Error("QueryBool(verbose) = false, want true")
	}
}

func TestFastCtx_QueryBool_Missing(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test")
	fc.Reset(fctx)

	if fc.QueryBool("verbose") {
		t.Error("QueryBool(verbose) = true, want false for missing")
	}
}

// --- Context methods: ClientIP ---

func TestFastCtx_ClientIP_Direct(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test")
	fc.Reset(fctx)

	ip := fc.ClientIP()
	if ip == "" {
		t.Error("ClientIP() should not be empty")
	}
}

func TestFastCtx_ClientIP_XForwardedFor(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test")
	fctx.Request.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	fc.Reset(fctx)

	// Trusting no proxy: XFF must be ignored entirely.
	if err := SetTrustedProxies(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ResetTrustedProxies)
	if ip := fc.ClientIP(); ip == "1.2.3.4" || ip == "5.6.7.8" {
		t.Fatalf("ClientIP trusted XFF with no trusted proxies: %q", ip)
	}

	// Convention default (private/loopback peers trusted): the rightmost
	// untrusted entry is the client; the leftmost is attacker-controlled.
	ResetTrustedProxies()
	if !fc.peerMaySetForwardedHeaders(fc.RemoteIP()) {
		t.Skipf("peer %q is not private/loopback in this environment", fc.RemoteIP())
	}
	if ip := fc.ClientIP(); ip != "5.6.7.8" {
		t.Errorf("ClientIP() = %q, want 5.6.7.8", ip)
	}
}

// --- Context methods: BaseURL ---

func TestFastCtx_BaseURL(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test")
	fctx.Request.Header.SetHost("example.com:8080")
	fc.Reset(fctx)

	url := fc.BaseURL()
	if url != "http://example.com:8080" {
		t.Errorf("BaseURL() = %q, want %q", url, "http://example.com:8080")
	}
}

// --- Context methods: IsWebSocket ---

func TestFastCtx_IsWebSocket_True(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.Set("Upgrade", "websocket")
	fc.Reset(fctx)

	if !fc.IsWebSocket() {
		t.Error("IsWebSocket() = false, want true")
	}
}

func TestFastCtx_IsWebSocket_False(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/test")
	fc.Reset(fctx)

	if fc.IsWebSocket() {
		t.Error("IsWebSocket() = true, want false")
	}
}

func TestFastCtx_CSV(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	err := fc.CSV("drivers", []string{"name", "phone"}, [][]string{{"Ada", "11"}, {"Bob", "22"}})
	if err != nil {
		t.Fatal(err)
	}
	if fctx.Response.StatusCode() != 200 {
		t.Fatalf("status=%d", fctx.Response.StatusCode())
	}
	ct := string(fctx.Response.Header.ContentType())
	if !strings.Contains(ct, "text/csv") {
		t.Fatalf("ct=%q", ct)
	}
	disp := string(fctx.Response.Header.Peek("Content-Disposition"))
	if !strings.Contains(disp, `filename="drivers.csv"`) {
		t.Fatalf("disposition=%q", disp)
	}
	body := string(fctx.Response.Body())
	if !strings.Contains(body, "name,phone") || !strings.Contains(body, "Ada,11") {
		t.Fatalf("body=%q", body)
	}
}

func TestFastCtx_CSVFrom(t *testing.T) {
	type row struct {
		Name     string   `json:"name"`
		Phone    string   `json:"phone"`
		Missions []string `json:"missions"`
	}
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	err := fc.CSVFrom("drivers.csv", []row{
		{Name: "Ada", Phone: "11", Missions: []string{"a"}},
	}, "name", "phone", "missions")
	if err != nil {
		t.Fatal(err)
	}
	body := string(fctx.Response.Body())
	if !strings.Contains(body, "name,phone,missions") || !strings.Contains(body, "Ada") {
		t.Fatalf("body=%q", body)
	}
	// encoding/csv escapes quotes inside JSON cells
	if !strings.Contains(body, "[\"\"a\"\"]") && !strings.Contains(body, `["a"]`) {
		t.Fatalf("expected missions JSON cell, body=%q", body)
	}
}

func TestFastCtx_Attachment(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	if err := fc.Attachment("note.txt", []byte("hi")); err != nil {
		t.Fatal(err)
	}
	disp := string(fctx.Response.Header.Peek("Content-Disposition"))
	if !strings.Contains(disp, `attachment; filename="note.txt"`) {
		t.Fatalf("disposition=%q", disp)
	}
	if string(fctx.Response.Body()) != "hi" {
		t.Fatalf("body=%q", fctx.Response.Body())
	}
}
