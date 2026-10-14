package usecases

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type captureAdapter struct {
	status int
	body   []byte
	hdr    map[string]string
}

func (a *captureAdapter) SetStatus(code int)    { a.status = code }
func (a *captureAdapter) SetHeader(k, v string) { a.hdr[k] = v }
func (a *captureAdapter) SetBody(data []byte)   { a.body = append([]byte(nil), data...) }

func newCaptureCtx(rid string) (*domain.Ctx, *captureAdapter) {
	a := &captureAdapter{hdr: map[string]string{}}
	c := &domain.Ctx{AdapterData: a}
	c.SetRequestID(rid)
	return c, a
}

func TestHandleErrorEnv_DevIncludesRequestID(t *testing.T) {
	c, a := newCaptureCtx("rid-1")
	HandleErrorEnv(c, errors.New("boom"), domain.EnvDevelopment)
	if a.status != 500 {
		t.Fatalf("status = %d", a.status)
	}
	var m map[string]string
	if err := json.Unmarshal(a.body, &m); err != nil {
		t.Fatalf("body %q: %v", a.body, err)
	}
	if m["request_id"] != "rid-1" {
		t.Fatalf("got %#v", m)
	}
	if m["error"] != "internal server error" {
		t.Fatalf("got %#v", m)
	}
	if m["hint"] == "" {
		t.Fatalf("missing hint: %#v", m)
	}
}

func TestHandleErrorEnv_ProdGeneric(t *testing.T) {
	c, a := newCaptureCtx("secret-rid")
	HandleErrorEnv(c, errors.New("x"), domain.EnvProduction)
	if string(a.body) != `{"error":"internal server error"}` {
		t.Fatalf("prod body = %q", a.body)
	}
}
