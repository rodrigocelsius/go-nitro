package fasthttp

import (
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// MaxBodySizeMiddleware returns middleware that limits request body size.
// Requests with a body larger than maxBytes receive 413 Payload Too Large.
// Checks both Content-Length and the actual body size to prevent bypass.
func MaxBodySizeMiddleware(maxBytes int64) domain.Handler {
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		fctx := fc.FasthttpCtx()

		contentLength := fctx.Request.Header.ContentLength()
		if contentLength > int(maxBytes) {
			return fc.JSON(413, map[string]any{"error": "payload too large", "max_bytes": maxBytes})
		}

		bodySize := int64(len(fctx.PostBody()))
		if bodySize > maxBytes {
			return fc.JSON(413, map[string]any{"error": "payload too large", "max_bytes": maxBytes})
		}

		return nil
	}
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
			return fc.JSON(431, map[string]string{"error": "headers too large"})
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
