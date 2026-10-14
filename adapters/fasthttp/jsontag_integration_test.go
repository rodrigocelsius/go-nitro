package fasthttp

import (
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
)

// fullyTaggedResp mirrors the models used everywhere in the framework
// today: every field has an explicit `json` tag. FastCtx.JSON must take
// the exact same pooled-encoder path as before (no jsontag overhead).
type fullyTaggedResp struct {
	Message string `json:"message"`
}

// noTagResp has no `json` tag at all: FastCtx.JSON must default to
// snake_case keys (see usecases/jsontag).
type noTagResp struct {
	CustomerName string
	UnitPriceUSD float64
}

func TestFastCtx_JSON_FullyTagged_Unchanged(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	if err := fc.JSON(200, fullyTaggedResp{Message: "hi"}); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	body := string(fctx.Response.Body())
	if body != `{"message":"hi"}` {
		t.Fatalf("got %q", body)
	}
}

func TestFastCtx_JSON_NoTags_DefaultsToSnakeCase(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fc.Reset(fctx)

	if err := fc.JSON(200, noTagResp{CustomerName: "Ada", UnitPriceUSD: 9.5}); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	body := string(fctx.Response.Body())
	if !strings.Contains(body, `"customer_name":"Ada"`) || !strings.Contains(body, `"unit_price_usd":9.5`) {
		t.Fatalf("got %q", body)
	}
	if strings.Contains(body, "CustomerName") {
		t.Fatalf("PascalCase key leaked: %q", body)
	}
}

func TestFastCtx_BodyJSON_NoTags_AcceptsSnakeCase(t *testing.T) {
	fc := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetBody([]byte(`{"customer_name":"Ada","unit_price_usd":9.5}`))
	fc.Reset(fctx)

	var v noTagResp
	if err := fc.BodyJSON(&v); err != nil {
		t.Fatalf("BodyJSON: %v", err)
	}
	if v.CustomerName != "Ada" || v.UnitPriceUSD != 9.5 {
		t.Fatalf("got %#v", v)
	}
}
