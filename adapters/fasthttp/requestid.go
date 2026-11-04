package fasthttp

import (
	"unsafe"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// maxRequestIDLen caps client-supplied request IDs.
const maxRequestIDLen = 64

// headerRequestID is domain.RequestIDHeader in fasthttp canonical form, so
// SetCanonical can skip key normalization.
var headerRequestID = []byte("X-Request-Id")

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
		var id string
		if raw := fctx.Request.Header.PeekBytes(headerRequestID); isSafeRequestID(b2s(raw)) {
			id = string(raw)
		} else {
			id = domain.GenerateRequestID()
		}
		c.SetRequestID(id)
		fctx.Response.Header.SetCanonical(headerRequestID, unsafe.Slice(unsafe.StringData(id), len(id)))
		return nil
	}
}

// isSafeRequestID reports whether id matches ^[A-Za-z0-9._-]{1,64}$.
// A byte loop instead of a regexp: this runs on every request.
func isSafeRequestID(id string) bool {
	if len(id) == 0 || len(id) > maxRequestIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		switch b := id[i]; {
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9',
			b == '.', b == '_', b == '-':
		default:
			return false
		}
	}
	return true
}
