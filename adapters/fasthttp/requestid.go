package fasthttp

import (
	"regexp"
	"unicode/utf8"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

var requestIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// RequestIDMiddleware returns middleware that:
// 1. Extracts X-Request-ID from the request (or generates a new one)
// 2. Stores it on Ctx
// 3. Adds it on the response header
//
// Client-supplied IDs are accepted only when they match a safe charset/length;
// otherwise a new ID is generated (mitigates log forging).
func RequestIDMiddleware() domain.Handler {
	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx).FasthttpCtx()
		id := b2s(fctx.Request.Header.Peek(domain.RequestIDHeader))
		if !isSafeRequestID(id) {
			id = domain.GenerateRequestID()
		}
		c.SetRequestID(id)
		fctx.Response.Header.Set(domain.RequestIDHeader, id)
		return nil
	}
}

func isSafeRequestID(id string) bool {
	if id == "" || !utf8.ValidString(id) {
		return false
	}
	return requestIDRe.MatchString(id)
}
