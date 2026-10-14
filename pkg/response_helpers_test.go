package nitro

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func helperAppCtx(app *App, method, path string, body []byte) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(path)
	if body != nil {
		ctx.Request.SetBody(body)
	}
	app.Handler()(ctx)
	return ctx
}

// --- Success Response Helpers ---

func TestResponse_OK(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.OK(map[string]string{"status": "ok"})
	})

	ctx := helperAppCtx(app, "GET", "/test", nil)

	if ctx.Response.StatusCode() != http.StatusOK {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body status = %q, want ok", body["status"])
	}
}

func TestResponse_OK_NilBody(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.OK(nil)
	})

	ctx := helperAppCtx(app, "GET", "/test", nil)
	if ctx.Response.StatusCode() != http.StatusOK {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusOK)
	}
}

func TestResponse_Created(t *testing.T) {
	app := New()
	app.Post("/users", func(c *Ctx) error {
		return c.Created(map[string]any{"id": "123", "name": "Joao"})
	})

	ctx := helperAppCtx(app, "POST", "/users", []byte(`{}`))

	if ctx.Response.StatusCode() != http.StatusCreated {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusCreated)
	}

	var body map[string]any
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["id"] != "123" {
		t.Errorf("body id = %v, want 123", body["id"])
	}
}

func TestResponse_NoContent(t *testing.T) {
	app := New()
	app.Delete("/users/:id", func(c *Ctx) error {
		return c.NoContent()
	})

	ctx := helperAppCtx(app, "DELETE", "/users/1", nil)

	if ctx.Response.StatusCode() != http.StatusNoContent {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusNoContent)
	}
}

// --- Error Response Helpers ---

func TestResponse_BadRequest_CustomMessage(t *testing.T) {
	app := New()
	app.Post("/users", func(c *Ctx) error {
		return BadRequest("name is required")
	})

	ctx := helperAppCtx(app, "POST", "/users", nil)

	if ctx.Response.StatusCode() != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["error"] != "name is required" {
		t.Errorf("body error = %q, want %q", body["error"], "name is required")
	}
}

func TestResponse_BadRequest_DefaultMessage(t *testing.T) {
	app := New()
	app.Post("/users", func(c *Ctx) error {
		return BadRequest()
	})

	ctx := helperAppCtx(app, "POST", "/users", nil)

	if ctx.Response.StatusCode() != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusBadRequest)
	}

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "bad request" {
		t.Errorf("body error = %q, want %q", body["error"], "bad request")
	}
}

func TestResponse_Unauthorized_CustomMessage(t *testing.T) {
	app := New()
	app.Get("/admin", func(c *Ctx) error {
		return Unauthorized("missing token")
	})

	ctx := helperAppCtx(app, "GET", "/admin", nil)

	if ctx.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusUnauthorized)
	}

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "missing token" {
		t.Errorf("body error = %q, want %q", body["error"], "missing token")
	}
}

func TestResponse_Unauthorized_DefaultMessage(t *testing.T) {
	app := New()
	app.Get("/admin", func(c *Ctx) error {
		return Unauthorized()
	})

	ctx := helperAppCtx(app, "GET", "/admin", nil)

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "unauthorized" {
		t.Errorf("body error = %q, want %q", body["error"], "unauthorized")
	}
}

func TestResponse_Forbidden_CustomMessage(t *testing.T) {
	app := New()
	app.Get("/admin", func(c *Ctx) error {
		return Forbidden("insufficient permissions")
	})

	ctx := helperAppCtx(app, "GET", "/admin", nil)

	if ctx.Response.StatusCode() != http.StatusForbidden {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusForbidden)
	}

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "insufficient permissions" {
		t.Errorf("body error = %q, want %q", body["error"], "insufficient permissions")
	}
}

func TestResponse_Forbidden_DefaultMessage(t *testing.T) {
	app := New()
	app.Get("/admin", func(c *Ctx) error {
		return Forbidden()
	})

	ctx := helperAppCtx(app, "GET", "/admin", nil)

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "forbidden" {
		t.Errorf("body error = %q, want %q", body["error"], "forbidden")
	}
}

func TestResponse_NotFound_CustomMessage(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error {
		return NotFound("user not found")
	})

	ctx := helperAppCtx(app, "GET", "/users/999", nil)

	if ctx.Response.StatusCode() != http.StatusNotFound {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusNotFound)
	}

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "user not found" {
		t.Errorf("body error = %q, want %q", body["error"], "user not found")
	}
}

func TestResponse_NotFound_DefaultMessage(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error {
		return NotFound()
	})

	ctx := helperAppCtx(app, "GET", "/users/999", nil)

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "not found" {
		t.Errorf("body error = %q, want %q", body["error"], "not found")
	}
}

func TestResponse_Conflict_CustomMessage(t *testing.T) {
	app := New()
	app.Post("/users", func(c *Ctx) error {
		return Conflict("email already exists")
	})

	ctx := helperAppCtx(app, "POST", "/users", nil)

	if ctx.Response.StatusCode() != http.StatusConflict {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusConflict)
	}

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "email already exists" {
		t.Errorf("body error = %q, want %q", body["error"], "email already exists")
	}
}

func TestResponse_Conflict_DefaultMessage(t *testing.T) {
	app := New()
	app.Post("/users", func(c *Ctx) error {
		return Conflict()
	})

	ctx := helperAppCtx(app, "POST", "/users", nil)

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "conflict" {
		t.Errorf("body error = %q, want %q", body["error"], "conflict")
	}
}

func TestResponse_UnprocessableEntity_CustomMessage(t *testing.T) {
	app := New()
	app.Post("/users", func(c *Ctx) error {
		return UnprocessableEntity("invalid data")
	})

	ctx := helperAppCtx(app, "POST", "/users", nil)

	if ctx.Response.StatusCode() != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusUnprocessableEntity)
	}

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "invalid data" {
		t.Errorf("body error = %q, want %q", body["error"], "invalid data")
	}
}

func TestResponse_UnprocessableEntity_DefaultMessage(t *testing.T) {
	app := New()
	app.Post("/users", func(c *Ctx) error {
		return UnprocessableEntity()
	})

	ctx := helperAppCtx(app, "POST", "/users", nil)

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "unprocessable entity" {
		t.Errorf("body error = %q, want %q", body["error"], "unprocessable entity")
	}
}

func TestResponse_InternalError(t *testing.T) {
	app := New()
	app.Get("/fail", func(c *Ctx) error {
		return InternalError(errors.New("database connection failed"))
	})

	ctx := helperAppCtx(app, "GET", "/fail", nil)

	if ctx.Response.StatusCode() != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", ctx.Response.StatusCode(), http.StatusInternalServerError)
	}

	var body map[string]string
	json.Unmarshal(ctx.Response.Body(), &body)
	if body["error"] != "internal server error" {
		t.Errorf("body error = %q, want %q", body["error"], "internal server error")
	}
}

// --- Error type checks (StatusCode + Body) ---

func TestHTTPErrorStatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"BadRequest", BadRequest("x"), 400},
		{"Unauthorized", Unauthorized(), 401},
		{"Forbidden", Forbidden(), 403},
		{"NotFound", NotFound(), 404},
		{"Conflict", Conflict(), 409},
		{"UnprocessableEntity", UnprocessableEntity(), 422},
		{"InternalError", InternalError(errors.New("x")), 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpErr, ok := tt.err.(domain.HTTPError)
			if !ok {
				t.Fatalf("error %T does not implement HTTPError", tt.err)
			}
			if httpErr.StatusCode() != tt.wantCode {
				t.Errorf("StatusCode() = %d, want %d", httpErr.StatusCode(), tt.wantCode)
			}
			if httpErr.Body() == nil {
				t.Error("Body() should not be nil")
			}
		})
	}
}

// --- ErrInvalidJSON ---

func TestErrInvalidJSON(t *testing.T) {
	var e domain.ErrInvalidJSON
	if e.StatusCode() != 400 {
		t.Errorf("StatusCode() = %d, want 400", e.StatusCode())
	}
	if e.Error() != "nitro: invalid JSON body" {
		t.Errorf("Error() = %q", e.Error())
	}
}

// --- Benchmarks ---

func BenchmarkResponse_OK(b *testing.B) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.OK(map[string]string{"key": "value"})
	})

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		helperAppCtx(app, "GET", "/test", nil)
	}
}

func BenchmarkResponse_BadRequest(b *testing.B) {
	app := New()
	app.Post("/test", func(c *Ctx) error {
		return BadRequest("something went wrong")
	})

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		helperAppCtx(app, "POST", "/test", nil)
	}
}

func BenchmarkResponse_NotFound(b *testing.B) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return NotFound("not found")
	})

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		helperAppCtx(app, "GET", "/test", nil)
	}
}

func BenchmarkResponse_NoContent(b *testing.B) {
	app := New()
	app.Delete("/test", func(c *Ctx) error {
		return c.NoContent()
	})

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		helperAppCtx(app, "DELETE", "/test", nil)
	}
}
