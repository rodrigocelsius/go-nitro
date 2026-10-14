package routetestdata

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/v2"
	"github.com/valyala/fasthttp"
)

func newFastCtx(method, path string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(path)
	return ctx
}

func TestGeneratedRoute_SingleIntParam(t *testing.T) {
	app := nitro.New()
	app.Get("/users/:id", GetUser_NitroHandler)

	handler := app.Handler()
	ctx := newFastCtx("GET", "/users/42")
	handler(ctx)

	got := string(ctx.Response.Body())
	want := "user-42"
	if got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}
}

func TestGeneratedRoute_TwoIntParams(t *testing.T) {
	app := nitro.New()
	app.Get("/orders/:orderId/items/:itemId", GetOrderItem_NitroHandler)

	handler := app.Handler()
	ctx := newFastCtx("GET", "/orders/7/items/99")
	handler(ctx)

	got := string(ctx.Response.Body())
	want := "order-7-item-99"
	if got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}
}

func TestGeneratedRoute_StringParam(t *testing.T) {
	app := nitro.New()
	app.Get("/products/:slug", GetProduct_NitroHandler)

	handler := app.Handler()
	ctx := newFastCtx("GET", "/products/mechanical-keyboard")
	handler(ctx)

	got := string(ctx.Response.Body())
	want := "product-mechanical-keyboard"
	if got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}
}

func TestGeneratedRoute_InvalidIntParamReturnsError(t *testing.T) {
	app := nitro.New()
	app.Get("/users/:id", GetUser_NitroHandler)

	handler := app.Handler()
	ctx := newFastCtx("GET", "/users/not-a-number")
	handler(ctx)

	if ctx.Response.StatusCode() != 500 {
		t.Fatalf("expected status 500 for invalid int param, got %d", ctx.Response.StatusCode())
	}
}

func TestGeneratedRoute_NegativeIntHandledByHandlerLogic(t *testing.T) {
	app := nitro.New()
	app.Get("/users/:id", GetUser_NitroHandler)

	handler := app.Handler()
	ctx := newFastCtx("GET", "/users/-5")
	handler(ctx)

	if ctx.Response.StatusCode() != 400 {
		t.Fatalf("expected status 400 (handler's own validation), got %d", ctx.Response.StatusCode())
	}
}
