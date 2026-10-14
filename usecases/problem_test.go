package usecases

import (
	"errors"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestProblemDetails(t *testing.T) {
	h := ProblemDetails()
	c, a := newCaptureCtx("rid-9")
	_ = h(c, domain.NewNotFound("user 7 not found"))
	if a.status != 404 || a.hdr["Content-Type"] != "application/problem+json" ||
		!strings.Contains(string(a.body), `"detail":"user 7 not found"`) ||
		!strings.Contains(string(a.body), `"title":"Not Found"`) ||
		!strings.Contains(string(a.body), `"request_id":"rid-9"`) {
		t.Fatalf("%d %v %s", a.status, a.hdr, a.body)
	}
	c, a = newCaptureCtx("")
	_ = h(c, errors.New("pq: password authentication failed"))
	if a.status != 500 || strings.Contains(string(a.body), "pq") {
		t.Fatalf("leak: %d %s", a.status, a.body)
	}
}
