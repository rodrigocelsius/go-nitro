package fasthttp

import (
	"io"

	"github.com/valyala/fasthttp"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// MaxBodySizeMiddleware returns middleware that limits request body size.
// Requests with a body larger than maxBytes receive 413 Payload Too Large.
// Checks both Content-Length and the actual body size to prevent bypass.
// The rejection is returned as an error, so the pipeline stops and the
// handler never sees the oversized body.
func MaxBodySizeMiddleware(maxBytes int64) domain.Handler {
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		fctx := fc.FasthttpCtx()

		contentLength := fctx.Request.Header.ContentLength()
		if int64(contentLength) > maxBytes {
			return &domain.PayloadTooLargeError{MaxBytes: maxBytes}
		}

		// Streamed body (Config.Server.StreamRequestBody). The server does
		// not enforce MaxRequestBodySize in this mode, and PostBody() would
		// buffer the whole stream before it could be measured. A body with a
		// Content-Length was already checked above (the stream ends there);
		// a chunked one is capped while it is read.
		if fctx.Request.IsBodyStream() {
			if contentLength < 0 {
				limited := &limitedBodyStream{src: fctx.RequestBodyStream(), left: maxBytes, max: maxBytes, fctx: fctx}
				fctx.Request.SetBodyStream(limited, -1)
			}
			return nil
		}

		bodySize := int64(len(fctx.PostBody()))
		if bodySize > maxBytes {
			return &domain.PayloadTooLargeError{MaxBytes: maxBytes}
		}

		return nil
	}
}

// limitedBodyStream caps a streamed request body of unknown length. Reading
// past the limit fails with PayloadTooLargeError (413 when the handler
// returns it) and the connection is closed after the response, since the
// rest of the body is never read.
type limitedBodyStream struct {
	src  io.Reader
	fctx *fasthttp.RequestCtx
	left int64
	max  int64
	over bool
}

func (l *limitedBodyStream) Read(p []byte) (int, error) {
	if l.over {
		return 0, &domain.PayloadTooLargeError{MaxBytes: l.max}
	}
	if len(p) == 0 {
		return 0, nil
	}
	// One byte past the limit tells "exactly at the limit" from "over it".
	if int64(len(p)) > l.left+1 {
		p = p[:l.left+1]
	}
	n, err := l.src.Read(p)
	if int64(n) > l.left {
		n = int(l.left)
		l.left = 0
		l.over = true
		l.fctx.SetConnectionClose()
		return n, &domain.PayloadTooLargeError{MaxBytes: l.max}
	}
	l.left -= int64(n)
	return n, err
}

// MaxHeaderSizeMiddleware returns middleware that limits the total
// size of request headers.
func MaxHeaderSizeMiddleware(maxBytes int) domain.Handler {
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}

		headerSize := len(fc.FasthttpCtx().Request.Header.Header())
		if headerSize > maxBytes {
			return &domain.CodedError{Status: 431, Code: "HEADERS_TOO_LARGE", Message: "headers too large"}
		}
		return nil
	}
}

// MaxParamsMiddleware returns middleware that limits the number
// of accepted query parameters.
func MaxParamsMiddleware(maxParams int) domain.Handler {
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}

		params := fc.FasthttpCtx().QueryArgs().Len()
		if params > maxParams {
			return domain.NewBadRequest("too many query parameters")
		}
		return nil
	}
}
